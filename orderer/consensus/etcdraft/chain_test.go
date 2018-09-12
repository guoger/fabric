/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package etcdraft_test

import (
	"encoding/pem"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"github.com/coreos/etcd/raft"
	"github.com/hyperledger/fabric/common/crypto/tlsgen"
	"github.com/hyperledger/fabric/common/flogging"
	mockconfig "github.com/hyperledger/fabric/common/mocks/config"
	"github.com/hyperledger/fabric/orderer/common/cluster"
	"github.com/hyperledger/fabric/orderer/consensus/etcdraft"
	"github.com/hyperledger/fabric/orderer/consensus/etcdraft/mocks"
	consensusmocks "github.com/hyperledger/fabric/orderer/consensus/mocks"
	mockblockcutter "github.com/hyperledger/fabric/orderer/mocks/common/blockcutter"
	"github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/orderer"
	raftprotos "github.com/hyperledger/fabric/protos/orderer/etcdraft"
	"github.com/hyperledger/fabric/protos/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

const interval = time.Second

var _ = Describe("Chain", func() {
	var (
		m           *common.Envelope
		normalBlock *common.Block
		channelID   string
		tlsCA       tlsgen.CA
		logger      *flogging.FabricLogger
	)

	BeforeEach(func() {
		tlsCA, _ = tlsgen.NewCA()
		channelID = "test-chain"
		logger = flogging.NewFabricLogger(zap.NewNop())
		m = &common.Envelope{
			Payload: utils.MarshalOrPanic(&common.Payload{
				Header: &common.Header{ChannelHeader: utils.MarshalOrPanic(&common.ChannelHeader{Type: int32(common.HeaderType_MESSAGE), ChannelId: channelID})},
				Data:   []byte("TEST_MESSAGE"),
			}),
		}
		normalBlock = &common.Block{Data: &common.BlockData{Data: [][]byte{[]byte("foo")}}}
	})

	Describe("Single raft node", func() {
		var (
			configurator      *mocks.Configurator
			consenterMetadata *raftprotos.Metadata
			clock             *fakeclock.FakeClock
			opt               etcdraft.Options
			support           *consensusmocks.FakeConsenterSupport
			cutter            *mockblockcutter.Receiver
			storage           *raft.MemoryStorage
			observe           chan uint64
			chain             *etcdraft.Chain
		)

		campaign := func() {
			clock.Increment(interval)
			Consistently(observe).ShouldNot(Receive())

			clock.Increment(interval)
			// The Raft election timeout is randomized in
			// [ElectionTick, 2 * ElectionTick - 1]
			// So we may need one extra tick to trigger
			// leader election.
			clock.Increment(interval)
			Eventually(observe).Should(Receive())
		}

		BeforeEach(func() {
			configurator = &mocks.Configurator{}
			configurator.On("Configure", mock.Anything, mock.Anything)
			clock = fakeclock.NewFakeClock(time.Now())
			storage = raft.NewMemoryStorage()
			observe = make(chan uint64, 1)
			opt = etcdraft.Options{
				RaftID:          uint64(1),
				Clock:           clock,
				TickInterval:    interval,
				ElectionTick:    2,
				HeartbeatTick:   1,
				MaxSizePerMsg:   1024 * 1024,
				MaxInflightMsgs: 256,
				Peers:           []raft.Peer{{ID: uint64(1)}},
				Logger:          logger,
				Storage:         storage,
			}
			support = &consensusmocks.FakeConsenterSupport{}
			support.ChainIDReturns(channelID)
			consenterMetadata = createMetadata(3, tlsCA)
			support.SharedConfigReturns(&mockconfig.Orderer{
				BatchTimeoutVal:      time.Hour,
				ConsensusMetadataVal: utils.MarshalOrPanic(consenterMetadata),
			})
			cutter = mockblockcutter.NewReceiver()
			support.BlockCutterReturns(cutter)

			var err error
			chain, err = etcdraft.NewChain(support, opt, configurator, nil, observe)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			chain.Start()

			// When the Raft node bootstraps, it produces a ConfChange
			// to add itself, which needs to be consumed with Ready().
			// If there are pending configuration changes in raft,
			// it refuses to campaign, no matter how many ticks elapse.
			// This is not a problem in the production code because raft.Ready
			// will be consumed eventually, as the wall clock advances.
			//
			// However, this is problematic when using the fake clock and
			// artificial ticks. Instead of ticking raft indefinitely until
			// raft.Ready is consumed, this check is added to indirectly guarantee
			// that the first ConfChange is actually consumed and we can safely
			// proceed to tick the Raft FSM.
			Eventually(func() error {
				_, err := storage.Entries(1, 1, 1)
				return err
			}).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			chain.Halt()
		})

		Context("when a node starts up", func() {
			It("properly configures the communication layer", func() {
				expectedNodeConfig := nodeConfigFromMetadata(consenterMetadata)
				configurator.AssertCalled(testingInstance, "Configure", channelID, expectedNodeConfig)
			})
		})

		Context("when no raft leader is elected", func() {
			It("fails to order envelope", func() {
				err := chain.Order(m, uint64(0))
				Expect(err).To(MatchError("no raft leader"))
			})
		})

		Context("when raft leader is elected", func() {
			JustBeforeEach(func() {
				campaign()
			})

			It("fails to order envelope if chain is halted", func() {
				chain.Halt()
				err := chain.Order(m, uint64(0))
				Expect(err).To(MatchError("chain is stopped"))
			})

			It("produces blocks following batch rules", func() {
				close(cutter.Block)
				support.CreateNextBlockReturns(normalBlock)

				By("cutting next batch directly")
				cutter.CutNext = true
				err := chain.Order(m, uint64(0))
				Expect(err).NotTo(HaveOccurred())
				Eventually(support.WriteBlockCallCount).Should(Equal(1))

				By("respecting batch timeout")
				cutter.CutNext = false
				timeout := time.Second
				support.SharedConfigReturns(&mockconfig.Orderer{BatchTimeoutVal: timeout})
				err = chain.Order(m, uint64(0))
				Expect(err).NotTo(HaveOccurred())

				clock.WaitForNWatchersAndIncrement(timeout, 2)
				Eventually(support.WriteBlockCallCount).Should(Equal(2))
			})

			It("does not reset timer for every envelope", func() {
				close(cutter.Block)
				support.CreateNextBlockReturns(normalBlock)

				timeout := time.Second
				support.SharedConfigReturns(&mockconfig.Orderer{BatchTimeoutVal: timeout})

				err := chain.Order(m, uint64(0))
				Expect(err).NotTo(HaveOccurred())
				Eventually(func() int {
					return len(cutter.CurBatch)
				}).Should(Equal(1))

				clock.WaitForNWatchersAndIncrement(timeout/2, 2)

				err = chain.Order(m, uint64(0))
				Expect(err).NotTo(HaveOccurred())
				Eventually(func() int {
					return len(cutter.CurBatch)
				}).Should(Equal(2))

				// the second envelope should not reset the timer; it should
				// therefore expire if we increment it by just timeout/2
				clock.Increment(timeout / 2)
				Eventually(support.WriteBlockCallCount).Should(Equal(1))
			})

			It("does not write a block if halted before timeout", func() {
				close(cutter.Block)
				timeout := time.Second
				support.SharedConfigReturns(&mockconfig.Orderer{BatchTimeoutVal: timeout})

				err := chain.Order(m, uint64(0))
				Expect(err).NotTo(HaveOccurred())
				Eventually(func() int {
					return len(cutter.CurBatch)
				}).Should(Equal(1))

				// wait for timer to start
				Eventually(clock.WatcherCount).Should(Equal(2))

				chain.Halt()
				Consistently(support.WriteBlockCallCount).Should(Equal(0))
			})

			It("stops the timer if a batch is cut", func() {
				close(cutter.Block)
				support.CreateNextBlockReturns(normalBlock)

				timeout := time.Second
				support.SharedConfigReturns(&mockconfig.Orderer{BatchTimeoutVal: timeout})

				err := chain.Order(m, uint64(0))
				Expect(err).NotTo(HaveOccurred())
				Eventually(func() int {
					return len(cutter.CurBatch)
				}).Should(Equal(1))

				clock.WaitForNWatchersAndIncrement(timeout/2, 2)

				By("force a batch to be cut before timer expires")
				cutter.CutNext = true
				err = chain.Order(m, uint64(0))
				Expect(err).NotTo(HaveOccurred())
				Eventually(support.WriteBlockCallCount).Should(Equal(1))
				Expect(support.CreateNextBlockArgsForCall(0)).To(HaveLen(2))
				Expect(cutter.CurBatch).To(HaveLen(0))

				// this should start a fresh timer
				cutter.CutNext = false
				err = chain.Order(m, uint64(0))
				Expect(err).NotTo(HaveOccurred())
				Eventually(func() int {
					return len(cutter.CurBatch)
				}).Should(Equal(1))

				clock.WaitForNWatchersAndIncrement(timeout/2, 2)
				Consistently(support.WriteBlockCallCount).Should(Equal(1))

				clock.Increment(timeout / 2)
				Eventually(support.WriteBlockCallCount).Should(Equal(2))
				Expect(support.CreateNextBlockArgsForCall(1)).To(HaveLen(1))
			})

			It("cut two batches if incoming envelope does not fit into first batch", func() {
				close(cutter.Block)
				support.CreateNextBlockReturns(normalBlock)

				timeout := time.Second
				support.SharedConfigReturns(&mockconfig.Orderer{BatchTimeoutVal: timeout})

				err := chain.Order(m, uint64(0))
				Expect(err).NotTo(HaveOccurred())
				Eventually(func() int {
					return len(cutter.CurBatch)
				}).Should(Equal(1))

				cutter.IsolatedTx = true
				err = chain.Order(m, uint64(0))
				Expect(err).NotTo(HaveOccurred())

				Eventually(support.CreateNextBlockCallCount).Should(Equal(2))
				Eventually(support.WriteBlockCallCount).Should(Equal(2))
			})

			Context("revalidation", func() {
				BeforeEach(func() {
					close(cutter.Block)
					support.CreateNextBlockReturns(normalBlock)

					timeout := time.Hour
					support.SharedConfigReturns(&mockconfig.Orderer{BatchTimeoutVal: timeout})
					support.SequenceReturns(1)
				})

				It("enqueue if an envelope is still valid", func() {
					support.ProcessNormalMsgReturns(1, nil)

					err := chain.Order(m, uint64(0))
					Expect(err).NotTo(HaveOccurred())
					Eventually(func() int {
						return len(cutter.CurBatch)
					}).Should(Equal(1))
				})

				It("does not enqueue if an envelope is not valid", func() {
					support.ProcessNormalMsgReturns(1, errors.Errorf("Envelope is invalid"))

					err := chain.Order(m, uint64(0))
					Expect(err).NotTo(HaveOccurred())
					Consistently(func() int {
						return len(cutter.CurBatch)
					}).Should(Equal(0))
				})
			})

			It("unblocks Errored if chain is halted", func() {
				Expect(chain.Errored()).NotTo(Receive())
				chain.Halt()
				Expect(chain.Errored()).Should(BeClosed())
			})

			Describe("Config updates", func() {
				var (
					configEnv   *common.Envelope
					configSeq   uint64
					configBlock *common.Block
				)

				// sets the configEnv var declared above
				createConfigEnvFromConfigUpdateEnv := func(chainID string, headerType common.HeaderType, configUpdateEnv *common.ConfigUpdateEnvelope) *common.Envelope {
					// TODO: use the config utility functions imported at end of file for doing the same
					return &common.Envelope{
						Payload: utils.MarshalOrPanic(&common.Payload{
							Header: &common.Header{
								ChannelHeader: utils.MarshalOrPanic(&common.ChannelHeader{
									Type:      int32(headerType),
									ChannelId: chainID,
								}),
							},
							Data: utils.MarshalOrPanic(&common.ConfigEnvelope{
								LastUpdate: &common.Envelope{
									Payload: utils.MarshalOrPanic(&common.Payload{
										Header: &common.Header{
											ChannelHeader: utils.MarshalOrPanic(&common.ChannelHeader{
												Type:      int32(common.HeaderType_CONFIG_UPDATE),
												ChannelId: chainID,
											}),
										},
										Data: utils.MarshalOrPanic(configUpdateEnv),
									}), // common.Payload
								}, // LastUpdate
							}),
						}),
					}
				}

				createConfigUpdateEnvFromOrdererValues := func(chainID string, values map[string]*common.ConfigValue) *common.ConfigUpdateEnvelope {
					return &common.ConfigUpdateEnvelope{
						ConfigUpdate: utils.MarshalOrPanic(&common.ConfigUpdate{
							ChannelId: chainID,
							ReadSet:   &common.ConfigGroup{},
							WriteSet: &common.ConfigGroup{
								Groups: map[string]*common.ConfigGroup{
									"Orderer": {
										Values: values,
									},
								},
							}, // WriteSet
						}),
					}
				}

				// ensures that configBlock has the correct configEnv
				JustBeforeEach(func() {
					configBlock = &common.Block{
						Data: &common.BlockData{
							Data: [][]byte{utils.MarshalOrPanic(configEnv)},
						},
					}
					support.CreateNextBlockReturns(configBlock)
				})

				Context("when a config update with invalid header comes", func() {

					BeforeEach(func() {
						configEnv = createConfigEnvFromConfigUpdateEnv(channelID,
							common.HeaderType_CONFIG_UPDATE, // invalid header; envelopes with CONFIG_UPDATE header never reach chain
							&common.ConfigUpdateEnvelope{ConfigUpdate: []byte("test invalid envelope")})
						configSeq = 0
					})

					It("should throw an error", func() {
						err := chain.Configure(configEnv, configSeq)
						Expect(err).To(MatchError("config transaction has unknown header type"))
					})
				})

				Context("when a type A config update comes", func() {

					Context("for existing channel", func() {

						// use to prepare the Orderer Values
						BeforeEach(func() {
							values := map[string]*common.ConfigValue{
								"BatchTimeout": {
									Version: 1,
									Value: utils.MarshalOrPanic(&orderer.BatchTimeout{
										Timeout: "3ms",
									}),
								},
							}
							configEnv = createConfigEnvFromConfigUpdateEnv(channelID,
								common.HeaderType_CONFIG,
								createConfigUpdateEnvFromOrdererValues(channelID, values),
							)
							configSeq = 0
						}) // BeforeEach block

						Context("without revalidation (i.e. correct config sequence)", func() {

							Context("without pending normal envelope", func() {
								It("should create a config block and no normal block", func() {
									err := chain.Configure(configEnv, configSeq)
									Expect(err).NotTo(HaveOccurred())
									Eventually(support.WriteConfigBlockCallCount).Should(Equal(1))
									Eventually(support.WriteBlockCallCount).Should(Equal(0))
								})
							})

							Context("with pending normal envelope", func() {
								It("should create a config block and a normal block", func() {
									close(cutter.Block)
									support.CreateNextBlockReturnsOnCall(0, normalBlock)
									support.CreateNextBlockReturnsOnCall(1, configBlock)

									By("adding a normal envelope")
									err := chain.Order(m, uint64(0))
									Expect(err).NotTo(HaveOccurred())
									Eventually(func() int {
										return len(cutter.CurBatch)
									}).Should(Equal(1))

									// // clock.WaitForNWatchersAndIncrement(timeout, 2)

									By("adding a config envelope")
									err = chain.Configure(configEnv, configSeq)
									By("came out of Configure")
									Expect(err).NotTo(HaveOccurred())

									Eventually(support.CreateNextBlockCallCount).Should(Equal(2))
									Eventually(support.WriteBlockCallCount).Should(Equal(1))
									Eventually(support.WriteConfigBlockCallCount).Should(Equal(1))
								})
							})
						})

						Context("with revalidation (i.e. incorrect config sequence)", func() {

							BeforeEach(func() {
								support.SequenceReturns(1) // this causes the revalidation
							})

							It("should create config block upon correct revalidation", func() {
								support.ProcessConfigMsgReturns(configEnv, 1, nil) // nil implies correct revalidation

								err := chain.Configure(configEnv, configSeq)
								Expect(err).NotTo(HaveOccurred())
								Eventually(support.WriteConfigBlockCallCount).Should(Equal(1))
							})

							It("should not create config block upon incorrect revalidation", func() {
								support.ProcessConfigMsgReturns(configEnv, 1, errors.Errorf("Invalid config envelope at changed config sequence"))

								err := chain.Configure(configEnv, configSeq)
								Expect(err).NotTo(HaveOccurred())
								Consistently(support.WriteConfigBlockCallCount).Should(Equal(0)) // no call to WriteConfigBlock
							})
						})
					})

					Context("for creating a new channel", func() {

						// use to prepare the Orderer Values
						BeforeEach(func() {
							chainID := "mychannel"
							configEnv = createConfigEnvFromConfigUpdateEnv(chainID,
								common.HeaderType_ORDERER_TRANSACTION,
								&common.ConfigUpdateEnvelope{ConfigUpdate: []byte("test channel creation envelope")})
							configSeq = 0
						}) // BeforeEach block

						It("should be able to create a channel", func() {
							err := chain.Configure(configEnv, configSeq)
							Expect(err).NotTo(HaveOccurred())
						})
					})
				}) // Context block for type A config

				Context("when a type B config update comes", func() {

					Context("for existing channel", func() {
						// use to prepare the Orderer Values
						BeforeEach(func() {
							values := map[string]*common.ConfigValue{
								"ConsensusType": {
									Version: 1,
									Value: utils.MarshalOrPanic(&orderer.ConsensusType{
										Metadata: []byte("new consenter"),
									}),
								},
							}
							configEnv = createConfigEnvFromConfigUpdateEnv(channelID,
								common.HeaderType_CONFIG,
								createConfigUpdateEnvFromOrdererValues(channelID, values))
							configSeq = 0
						}) // BeforeEach block

						It("should throw an error", func() {
							err := chain.Configure(configEnv, configSeq)
							Expect(err).To(MatchError("updates to ConsensusType not supported currently"))
						})
					})
				})
			})
		})
	})

	Describe("Multiple raft nodes", func() {
		var (
			network    *network
			channelID  string
			timeout    time.Duration
			c1, c2, c3 *chain
		)

		BeforeEach(func() {
			channelID = "multi-node-channel"
			timeout = 10 * time.Second

			network = createNetwork(timeout, channelID, []uint64{1, 2, 3})
			c1 = network.get(1)
			c2 = network.get(2)
			c3 = network.get(3)
		})

		When("2/3 nodes are running", func() {
			It("late node can catch up", func() {
				network.start(1, 2)
				network.elect(1)

				c1.cutter.CutNext = true
				err := c1.Order(m, uint64(0))
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() int { return c1.support.WriteBlockCallCount() }).Should(Equal(1))
				Eventually(func() int { return c2.support.WriteBlockCallCount() }).Should(Equal(1))
				Eventually(func() int { return c3.support.WriteBlockCallCount() }).Should(Equal(0))

				network.start(3)

				c1.clock.Increment(interval)
				Eventually(func() int { return c3.support.WriteBlockCallCount() }).Should(Equal(1))

				network.stop()
			})
		})

		When("3/3 nodes are running", func() {
			JustBeforeEach(func() {
				network.start()
				network.elect(1)
			})

			AfterEach(func() {
				network.stop()
			})

			It("orders envelope on leader", func() {
				By("instructed to cut next block")
				c1.cutter.CutNext = true
				err := c1.Order(m, uint64(0))
				Expect(err).ToNot(HaveOccurred())

				network.exec(
					func(c *chain) {
						Eventually(func() int { return c.support.WriteBlockCallCount() }).Should(Equal(1))
					})

				By("respect batch timeout")
				c1.cutter.CutNext = false

				timeout := time.Second
				c1.support.SharedConfigReturns(&mockconfig.Orderer{BatchTimeoutVal: timeout})
				c1.support.CreateNextBlockReturns(normalBlock)

				err = c1.Order(m, uint64(0))
				Expect(err).ToNot(HaveOccurred())

				c1.clock.WaitForNWatchersAndIncrement(timeout, 2)
				network.exec(
					func(c *chain) {
						Eventually(func() int { return c.support.WriteBlockCallCount() }).Should(Equal(2))
					})
			})

			It("orders envelope on follower", func() {
				By("instructed to cut next block")
				c1.cutter.CutNext = true
				err := network.get(2).Order(m, uint64(0))
				Expect(err).ToNot(HaveOccurred())

				network.exec(
					func(c *chain) {
						Eventually(func() int { return c.support.WriteBlockCallCount() }).Should(Equal(1))
					})

				By("respect batch timeout")
				c1.cutter.CutNext = false

				timeout := time.Second
				c1.support.SharedConfigReturns(&mockconfig.Orderer{BatchTimeoutVal: timeout})
				c1.support.CreateNextBlockReturns(normalBlock)

				err = network.get(2).Order(m, uint64(0))
				Expect(err).ToNot(HaveOccurred())

				c1.clock.WaitForNWatchersAndIncrement(timeout, 2)
				network.exec(
					func(c *chain) {
						Eventually(func() int { return c.support.WriteBlockCallCount() }).Should(Equal(2))
					})
			})

			Context("failover", func() {
				It("follower should step up as leader upon failover", func() {
					network.stop(1)
					network.elect(2)

					By("order env on new leader")
					c2.cutter.CutNext = true
					err := c2.Order(m, uint64(0))
					Expect(err).ToNot(HaveOccurred())

					// block should not be produced on chain 1
					Eventually(c1.support.WriteBlockCallCount).Should(Equal(0))

					// block should be produced on chain 2 & 3
					Eventually(c2.support.WriteBlockCallCount).Should(Equal(1))
					Eventually(c3.support.WriteBlockCallCount).Should(Equal(1))

					By("order env on follower")
					err = c3.Order(m, uint64(0))
					Expect(err).ToNot(HaveOccurred())

					// block should not be produced on chain 1
					Eventually(c1.support.WriteBlockCallCount).Should(Equal(0))

					// block should be produced on chain 2 & 3
					Eventually(c2.support.WriteBlockCallCount).Should(Equal(2))
					Eventually(c3.support.WriteBlockCallCount).Should(Equal(2))
				})

				It("purges blockcutter and stops timer if leadership is lost", func() {
					// enque one transaction into 1's blockcutter
					err := c1.Order(m, 0)
					Expect(err).ToNot(HaveOccurred())
					Eventually(func() int {
						return len(c1.cutter.CurBatch)
					}).Should(Equal(1))

					network.disconnect(1)
					network.elect(2)
					network.connect()

					// 1 steps down upon reconnect
					c2.clock.Increment(interval)                             // trigger a heartbeat 2->1
					Eventually(c1.observe).Should(Receive(Equal(uint64(2)))) // leader change
					Expect(c1.clock.WatcherCount()).To(Equal(1))             // timer should be stopped

					Eventually(func() int {
						return len(c1.cutter.CurBatch)
					}).Should(Equal(0))

					network.disconnect(2)
					network.elect(1)

					err = c1.Order(m, 0)
					Expect(err).ToNot(HaveOccurred())

					// we ticked 3*interval in campaign. if batch timer was
					// properly stopped, this should not trigger a block to be cut
					c1.clock.WaitForNWatchersAndIncrement(timeout-3*interval, 2)
					Eventually(func() int { return c1.support.WriteBlockCallCount() }).Should(Equal(0))
					Eventually(func() int { return c3.support.WriteBlockCallCount() }).Should(Equal(0))

					c1.clock.Increment(3 * interval)
					Eventually(func() int { return c1.support.WriteBlockCallCount() }).Should(Equal(1))
					Eventually(func() int { return c3.support.WriteBlockCallCount() }).Should(Equal(1))
				})

				It("stale leader should not be able to propose block because of lagged term", func() {
					network.disconnect(1)
					network.elect(2)
					network.connect()

					c1.cutter.CutNext = true
					err := c1.Order(m, 0)
					Expect(err).NotTo(HaveOccurred())

					network.exec(
						func(c *chain) {
							Consistently(c.support.WriteBlockCallCount).Should(Equal(0))
						})
				})

				It("aborts waiting for block to be committed upon leadership lost", func() {
					network.disconnect(1)

					c1.cutter.CutNext = true
					err := c1.Order(m, 0)
					Expect(err).NotTo(HaveOccurred())

					network.exec(
						func(c *chain) {
							Consistently(c.support.WriteBlockCallCount).Should(Equal(0))
						})

					network.elect(2)
					network.connect()

					c2.clock.Increment(interval)
					err = c1.Order(m, 0)
					Expect(err).ShouldNot(HaveOccurred())

					network.exec(
						func(c *chain) {
							Consistently(c.support.WriteBlockCallCount).Should(Equal(0))
						})
				})
			})
		})
	})
})

func nodeConfigFromMetadata(consenterMetadata *raftprotos.Metadata) []cluster.RemoteNode {
	var nodes []cluster.RemoteNode
	for i, consenter := range consenterMetadata.Consenters {
		// For now, skip ourselves
		if i == 0 {
			continue
		}
		serverDER, _ := pem.Decode(consenter.ServerTlsCert)
		clientDER, _ := pem.Decode(consenter.ClientTlsCert)
		node := cluster.RemoteNode{
			ID:            uint64(i + 1),
			Endpoint:      "localhost:7050",
			ServerTLSCert: serverDER.Bytes,
			ClientTLSCert: clientDER.Bytes,
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func createMetadata(nodeCount int, tlsCA tlsgen.CA) *raftprotos.Metadata {
	md := &raftprotos.Metadata{}
	for i := 0; i < nodeCount; i++ {
		md.Consenters = append(md.Consenters, &raftprotos.Consenter{
			Host:          "localhost",
			Port:          7050,
			ServerTlsCert: serverTLSCert(tlsCA),
			ClientTlsCert: clientTLSCert(tlsCA),
		})
	}
	return md
}

func serverTLSCert(tlsCA tlsgen.CA) []byte {
	cert, err := tlsCA.NewServerCertKeyPair("localhost")
	if err != nil {
		panic(err)
	}
	return cert.Cert
}

func clientTLSCert(tlsCA tlsgen.CA) []byte {
	cert, err := tlsCA.NewClientCertKeyPair()
	if err != nil {
		panic(err)
	}
	return cert.Cert
}

// helpers to facilitate tests
type chain struct {
	support      *consensusmocks.FakeConsenterSupport
	cutter       *mockblockcutter.Receiver
	configurator *mocks.Configurator
	rpc          *mocks.FakeRPC
	storage      *raft.MemoryStorage
	clock        *fakeclock.FakeClock

	observe      chan uint64
	unstarted    chan struct{}
	disconnected chan struct{}

	*etcdraft.Chain
}

func newChain(timeout time.Duration, channel string, id uint64, all []uint64) *chain {
	rpc := &mocks.FakeRPC{}
	clock := fakeclock.NewFakeClock(time.Now())
	storage := raft.NewMemoryStorage()

	peers := []raft.Peer{}
	for _, i := range all {
		peers = append(peers, raft.Peer{ID: i})
	}

	opts := etcdraft.Options{
		RaftID:          uint64(id),
		Clock:           clock,
		TickInterval:    interval,
		ElectionTick:    2,
		HeartbeatTick:   1,
		MaxSizePerMsg:   1024 * 1024,
		MaxInflightMsgs: 256,
		Peers:           peers,
		Logger:          flogging.NewFabricLogger(zap.NewNop()),
		Storage:         storage,
	}

	support := &consensusmocks.FakeConsenterSupport{}
	support.ChainIDReturns(channel)
	support.CreateNextBlockReturns(&common.Block{Data: &common.BlockData{Data: [][]byte{[]byte("foo")}}})
	support.SharedConfigReturns(&mockconfig.Orderer{BatchTimeoutVal: timeout})

	cutter := mockblockcutter.NewReceiver()
	close(cutter.Block)
	support.BlockCutterReturns(cutter)

	observe := make(chan uint64, 2)

	configurator := &mocks.Configurator{}
	configurator.On("Configure", mock.Anything, mock.Anything)

	c, err := etcdraft.NewChain(support, opts, configurator, rpc, observe)
	Expect(err).NotTo(HaveOccurred())

	ch := make(chan struct{})
	close(ch)
	return &chain{
		support:   support,
		cutter:    cutter,
		rpc:       rpc,
		storage:   storage,
		observe:   observe,
		clock:     clock,
		unstarted: ch,
		Chain:     c,
	}
}

type network map[uint64]*chain

func createNetwork(timeout time.Duration, channel string, ids []uint64) *network {
	n := make(network)

	for _, i := range ids {
		c := newChain(timeout, channel, i, ids)
		n[i] = c
	}

	n.connect()

	return &n
}

func (n *network) start(ids ...uint64) {
	nodes := ids
	if len(nodes) == 0 {
		for i := range *n {
			nodes = append(nodes, i)
		}
	}

	for _, i := range nodes {
		(*n)[i].Start()
		(*n)[i].unstarted = nil

		// When raft node bootstraps, it produces a ConfChange
		// to add itself, which needs to be consumed with Ready().
		// If there are pending configuration changes in raft,
		// it refused to campaign, no matter how many ticks supplied.
		// This is not a problem in production code because eventually
		// raft.Ready will be consumed as real time goes by.
		//
		// However, this is problematic when using fake clock and artificial
		// ticks. Instead of ticking raft indefinitely until raft.Ready is
		// consumed, this check is added to indirectly guarantee
		// that first ConfChange is actually consumed and we can safely
		// proceed to tick raft.
		Eventually(func() error {
			_, err := (*n)[i].storage.Entries(1, 1, 1)
			return err
		}).ShouldNot(HaveOccurred())
	}
}

func (n *network) stop(ids ...uint64) {
	nodes := ids
	if len(nodes) == 0 {
		for i := range *n {
			nodes = append(nodes, i)
		}
	}

	for _, c := range nodes {
		(*n)[c].Halt()
		<-(*n)[c].Errored()
	}
}

func (n *network) get(id uint64) *chain {
	return (*n)[id]
}

func (n *network) exec(f func(c *chain), ids ...uint64) {
	if len(ids) == 0 {
		for _, c := range *n {
			f(c)
		}

		return
	}

	for _, i := range ids {
		f((*n)[i])
	}
}

func (n *network) elect(id uint64) {
	c := (*n)[id]
	c.clock.Increment(interval)
	Consistently(c.observe).ShouldNot(Receive())

	c.clock.Increment(interval)
	// raft election timeout is randomized in
	// [electiontimeout, 2 * electiontimeout - 1]
	// So we may need one extra tick to trigger
	// leader election.
	c.clock.Increment(interval)
	for _, c := range *n {
		select {
		case <-c.Errored():
		case <-c.disconnected:
		case <-c.unstarted:
		default:
			Eventually(c.observe).Should(Receive(Equal(id)))
		}
	}
}

func (n *network) disconnect(i uint64) {
	close((*n)[i].disconnected)

	// drop i -> ANY
	(*n)[i].rpc.StepStub = func(dest uint64, msg *orderer.StepRequest) (*orderer.StepResponse, error) {
		return nil, nil
	}

	// drop ANY -> x
	for id, c := range *n {
		if id == i {
			continue
		}

		c.rpc.StepStub = func(dest uint64, msg *orderer.StepRequest) (*orderer.StepResponse, error) {
			if dest == i {
				return nil, nil
			}
			go (*n)[dest].Step(msg, id)
			return nil, nil
		}
	}
}

func (n *network) connect() {
	for id, c := range *n {
		c.disconnected = make(chan struct{})

		c.rpc.SendSubmitStub = func(dest uint64, msg *orderer.SubmitRequest) error {
			go (*n)[dest].Submit(msg, id)
			return nil
		}

		c.rpc.StepStub = func(dest uint64, msg *orderer.StepRequest) (*orderer.StepResponse, error) {
			go (*n)[dest].Step(msg, id)
			return nil, nil
		}
	}
}
