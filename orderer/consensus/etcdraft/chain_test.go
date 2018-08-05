/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/
package etcdraft_test

import (
	"fmt"
	"time"

	"github.com/hyperledger/fabric/common/flogging"
	mockconfig "github.com/hyperledger/fabric/common/mocks/config"
	"github.com/hyperledger/fabric/orderer/consensus/etcdraft"
	consensusmocks "github.com/hyperledger/fabric/orderer/consensus/mocks"
	mockblockcutter "github.com/hyperledger/fabric/orderer/mocks/common/blockcutter"
	"github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/utils"

	"code.cloudfoundry.org/clock/fakeclock"
	"github.com/coreos/etcd/raft"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
)

var logger *flogging.FabricLogger

func init() {
	logger = flogging.NewFabricLogger(zap.NewExample())
}

var _ = Describe("Chain", func() {
	var m = &common.Envelope{
		Payload: utils.MarshalOrPanic(&common.Payload{
			Header: &common.Header{ChannelHeader: utils.MarshalOrPanic(&common.ChannelHeader{Type: int32(common.HeaderType_MESSAGE), ChannelId: "foo"})},
			Data:   []byte("TEST_MESSAGE"),
		}),
	}

	var normalBlock = &common.Block{Data: &common.BlockData{Data: [][]byte{[]byte("foo")}}}

	Describe("Single Raft node", func() {
		Context("Order envelope", func() {
			var (
				clock   *fakeclock.FakeClock
				opt     etcdraft.Options
				support *consensusmocks.FakeConsenterSupport
				cutter  *mockblockcutter.Receiver
				storage *raft.MemoryStorage
				chain   *etcdraft.Chain
			)

			BeforeEach(func() {
				clock = fakeclock.NewFakeClock(time.Now())
				storage = raft.NewMemoryStorage()
				opt = etcdraft.Options{
					RaftID:          uint64(1),
					Clock:           clock,
					TickInterval:    time.Second,
					ElectionTick:    2,
					HeartbeatTick:   1,
					MaxSizePerMsg:   1024 * 1024,
					MaxInflightMsgs: 256,
					Peers:           []raft.Peer{{ID: uint64(1)}},
					Logger:          logger,
					Storage:         storage,
				}
				support = new(consensusmocks.FakeConsenterSupport)
				support.SharedConfigReturns(&mockconfig.Orderer{BatchTimeoutVal: time.Hour})

				cutter = mockblockcutter.NewReceiver()
				support.BlockCutterReturns(cutter)

				var err error
				chain, err = etcdraft.NewChain(support, opt, true)
				Expect(err).ToNot(HaveOccurred())
				chain.Start()

				// When raft node bootstraps, it produces a ConfChange
				// to add itself, which needs to be consumed with Ready().
				// If there are pending configuration changes in raft,
				// it refused to campaign, no matter how many ticks supplied.
				//
				// This check indirectly guarantees first ConfChange
				// is actually consumed and we can safely proceed to
				// tick raft.
				Eventually(func() error {
					_, err := storage.Entries(1, 1, 1)
					return err
				}).ShouldNot(HaveOccurred())
			})

			AfterEach(func() {
				chain.Halt()
			})

			It("should fail if there is no raft leader", func() {
				err := chain.Order(m, uint64(0))
				Expect(err).To(HaveOccurred())
			})

			It("should fail if chain is halted", func() {
				// According to ticker documentation, ticks may be dropped
				// in case of a slow receiver. We need to keep advancing
				// clock until leader is elected.
				elected := false
				for !elected {
					select {
					case <-chain.Observe():
						elected = true
					default:
						clock.IncrementBySeconds(1)
					}
				}

				chain.Halt()

				Consistently(func() error {
					return chain.Order(m, uint64(0))
				}).Should(HaveOccurred())
			})

			It("should be able to produce a block", func() {
				cutter.CutNext = true
				close(cutter.Block)
				support.CreateNextBlockReturns(normalBlock)

				elected := false
				for !elected {
					select {
					case <-chain.Observe():
						elected = true
					default:
						clock.IncrementBySeconds(1)
					}
				}

				err := chain.Order(m, uint64(0))
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() int { return support.WriteBlockCallCount() }).Should(Equal(1))
			})

			FIt("should respect batch timer", func() {
				close(cutter.Block)

				support.SharedConfigReturns(&mockconfig.Orderer{BatchTimeoutVal: time.Second})
				support.CreateNextBlockReturns(normalBlock)

				elected := false
				for !elected {
					select {
					case <-chain.Observe():
						elected = true
					default:
						clock.IncrementBySeconds(1)
					}
				}

				err := chain.Order(m, uint64(0))
				Expect(err).ToNot(HaveOccurred())

				clock.IncrementBySeconds(2)
				Eventually(func() int { return support.WriteBlockCallCount() }).Should(Equal(1))
			})

			It("Timer", func() {
				timer := clock.NewTimer(time.Second)
				done := make(chan struct{})
				go func() {
					<-timer.C()
					fmt.Printf("Fired\n")
					close(done)
				}()
				clock.IncrementBySeconds(1)
				<-done
			})
		})
	})
})
