package raft_test

import (
	"fmt"
	etcdraft "github.com/coreos/etcd/raft"
	"github.com/hyperledger/fabric/orderer/consensus/raft"
	"github.com/hyperledger/fabric/orderer/consensus/raft/raftfakes"
	"sync"

	"time"

	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/protos/orderer"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func init() {
	flogging.SetModuleLevel(raft.PkgLogID, "DEBUG")
}

var _ = Describe("Raft", func() {
	Describe("submit a message", func() {

		Context("when there is only one peer", func() {
			var (
				signalC         chan time.Time
				fakeRaftTicker  *raftfakes.FakeTicker
				fakeRaftSupport *raftfakes.FakeRaftSupport
			)

			BeforeEach(func() {
				signalC = make(chan time.Time)

				fakeRaftTicker = new(raftfakes.FakeTicker)
				fakeRaftTicker.SignalReturns(signalC)

				fakeRaftSupport = new(raftfakes.FakeRaftSupport)
				fakeRaftSupport.NewTickerReturns(fakeRaftTicker)
				fakeRaftSupport.IDReturns(uint64(1))
				fakeRaftSupport.HeartbeatTickReturns(1)
				fakeRaftSupport.ElectionTickReturns(5)
				fakeRaftSupport.MaxSizePerMsgReturns(1024 * 1024)
				fakeRaftSupport.MaxInflightMsgsReturns(256)
				fakeRaftSupport.PeersReturns([]etcdraft.Peer{etcdraft.Peer{ID: uint64(1)}})
			})

			It("fails if no leader exists", func() {
				sendC := make(chan *ab.FabricMessage)

				cluster := raft.NewRaftCluster(fakeRaftSupport, sendC)
				cluster.Start()
				defer cluster.Stop()

				done := make(chan struct{})
				go func() {
					Consistently(sendC).ShouldNot(Receive())
					close(done)
				}()
				Expect(cluster.Submit(&ab.FabricMessage{})).Should(HaveOccurred())
				<-done
			})

			It("succeeds if this is leader", func() {
				sendC := make(chan *ab.FabricMessage)

				cluster := raft.NewRaftCluster(fakeRaftSupport, sendC)
				cluster.Start()
				defer cluster.Stop()

				done := make(chan struct{})
				go func() {
					Eventually(sendC).Should(Receive())
					close(done)
				}()
				tickNTimes(11, signalC)
				Expect(cluster.Submit(&ab.FabricMessage{})).ShouldNot(HaveOccurred())
				<-done
			})
		})

		Context("when there are 3 peers", func() {
			type testCluter struct {
				signal  chan time.Time
				recv    chan *ab.Message
				support *raftfakes.FakeRaftSupport
			}

			var (
				clusters map[uint64]*testCluter
			)

			BeforeEach(func() {
				clusters = make(map[uint64]*testCluter)

				newTestCluster := func(id uint64) *testCluter {
					signal := make(chan time.Time)
					fakeRaftTicker := new(raftfakes.FakeTicker)
					fakeRaftTicker.SignalReturns(signal)

					fakeRaftSupport := new(raftfakes.FakeRaftSupport)
					fakeRaftSupport.NewTickerReturns(fakeRaftTicker)
					fakeRaftSupport.IDReturns(id)
					fakeRaftSupport.HeartbeatTickReturns(1)
					fakeRaftSupport.ElectionTickReturns(2)
					fakeRaftSupport.MaxSizePerMsgReturns(1024 * 1024)
					fakeRaftSupport.MaxInflightMsgsReturns(256)
					fakeRaftSupport.PeersReturns([]etcdraft.Peer{
						etcdraft.Peer{ID: uint64(1)},
						etcdraft.Peer{ID: uint64(2)},
						etcdraft.Peer{ID: uint64(3)},
					})

					recv := make(chan *ab.Message)
					fakeRaftSupport.RecvReturns(recv)

					return &testCluter{signal: signal, recv: recv, support: fakeRaftSupport}
				}

				for _, i := range []uint64{1, 2, 3} {
					c := newTestCluster(i)
					c.support.SendStub = func(msg *ab.Message) error {
						go func() {
							fmt.Printf("%d -> %d\n", msg.From, msg.To)
							clusters[msg.To].recv <- msg
						}()
						return nil
					}

					clusters[i] = c
				}
			})

			It("can elect leader", func() {
				sendC1 := make(chan *ab.FabricMessage)

				cluster1 := raft.NewRaftCluster(clusters[uint64(1)].support, sendC1)
				cluster1.Start()
				defer cluster1.Stop()

				cluster2 := raft.NewRaftCluster(clusters[uint64(2)].support, make(chan *ab.FabricMessage))
				cluster2.Start()
				defer cluster2.Stop()

				cluster3 := raft.NewRaftCluster(clusters[uint64(3)].support, make(chan *ab.FabricMessage))
				cluster3.Start()
				defer cluster3.Stop()

				// Only tick node 1 to have deterministic leader election
				tickNTimes(3, clusters[uint64(1)].signal)
				Eventually(clusters[uint64(1)].support.SendCallCount).Should(Equal(6))

				done := make(chan struct{})
				go func() {
					Eventually(sendC1).Should(Receive())
					close(done)
				}()
				Expect(cluster2.Submit(&ab.FabricMessage{})).ShouldNot(HaveOccurred())
				<-done
			})
		})
	})
})

func tickNTimes(n uint16, signals ...chan<- time.Time) {
	for i := uint16(0); i < n; i++ {
		var wg sync.WaitGroup
		wg.Add(len(signals))

		for _, signal := range signals {
			go func(s chan<- time.Time) {
				s <- time.Time{}
				wg.Done()
			}(signal)
		}

		wg.Wait()
	}
}
