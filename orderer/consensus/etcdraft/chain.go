/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package etcdraft

import (
	"code.cloudfoundry.org/clock"
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/orderer/consensus"
	"github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/orderer"
	"github.com/hyperledger/fabric/protos/utils"

	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
)

// Storage essentially represents etcd/raft.MemoryStorage.
//
// This interface is defined to expose dependencies of fsm
// so that it may be swapped in the future.
//
// TODO(jay) add other necessary methods to this interface
// once we need them in implementation, e.g. ApplySnapshot
type Storage interface {
	raft.Storage
	Append(entries []raftpb.Entry) error
}

// Options contains necessary artifacts to start raft-based chain
type Options struct {
	RaftID uint64

	Clock clock.Clock

	Storage Storage
	Logger  *flogging.FabricLogger

	TickInterval    time.Duration
	ElectionTick    int
	HeartbeatTick   int
	MaxSizePerMsg   uint64
	MaxInflightMsgs int
	Peers           []raft.Peer
}

// Chain implements consensus.Chain interface with raft-based consensus
type Chain struct {
	raftID uint64

	submitC chan *orderer.SubmitRequest
	commitC chan *common.Block
	exitC   chan struct{}
	observe chan uint64

	clock  clock.Clock  // test could inject a fake clock
	timer  clock.Timer  // batch timer
	ticker clock.Ticker // raft ticker

	support consensus.ConsenterSupport

	leader       uint64 // Should this be accessed atomically?
	appliedIndex uint64

	node    raft.Node
	storage Storage
	opts    Options

	logger *flogging.FabricLogger
}

// NewChain constructs a chain object
func NewChain(support consensus.ConsenterSupport, opts Options, observe bool) (*Chain, error) {
	exitC := make(chan struct{})
	close(exitC)

	var o chan uint64
	if observe {
		o = make(chan uint64)
	}

	return &Chain{
		raftID:  opts.RaftID,
		submitC: make(chan *orderer.SubmitRequest),
		commitC: make(chan *common.Block),
		exitC:   exitC,
		observe: o,
		support: support,
		clock:   opts.Clock,
		logger:  opts.Logger,
		storage: opts.Storage,
		opts:    opts,
	}, nil
}

// Start starts the chain
func (c *Chain) Start() {
	config := &raft.Config{
		ID:              c.raftID,
		ElectionTick:    c.opts.ElectionTick,
		HeartbeatTick:   c.opts.HeartbeatTick,
		MaxSizePerMsg:   c.opts.MaxSizePerMsg,
		MaxInflightMsgs: c.opts.MaxInflightMsgs,
		Logger:          c.opts.Logger,
		Storage:         c.opts.Storage,
	}

	c.node = raft.StartNode(config, c.opts.Peers)

	c.exitC = make(chan struct{})

	go c.serveRaft()
	go c.serveRequest()
}

// Order submits normal type transactions
func (c *Chain) Order(env *common.Envelope, configSeq uint64) error {
	return c.Submit(&orderer.SubmitRequest{LastValidationSeq: configSeq, Content: env})
}

// Configure submits config type transactins
func (c *Chain) Configure(env *common.Envelope, configSeq uint64) error {
	c.logger.Panicf("Configure not implemented yet")
	return nil
}

// WaitReady is currently no-op
func (c *Chain) WaitReady() error {
	return nil
}

// Errored indicates if chain is still running
func (c *Chain) Errored() <-chan struct{} {
	return c.exitC
}

// Halt stops chain
func (c *Chain) Halt() {
	select {
	case <-c.exitC:
	default:
		close(c.exitC)
	}
}

// Submit submits requests to
// - local serveRequest go routine if this is leader
// - actual leader via transport
// - fails if there's no leader elected yet
func (c *Chain) Submit(req *orderer.SubmitRequest) error {
	select {
	case <-c.exitC:
		return fmt.Errorf("chain is not started")
	default:
	}

	if c.leader == raft.None {
		return fmt.Errorf("there's no raft leader")
	}

	if c.leader == c.raftID {
		c.submitC <- req
		return nil
	}

	// TODO forward request to actual leader when we implement multi-node raft
	return fmt.Errorf("only single raft node is currently supported")
}

// Observe exposes leader changes.
// It is used as a synchronization mechanism so that
// tests could deterministically observe node behavior
func (c *Chain) Observe() <-chan uint64 {
	return c.observe
}

func (c *Chain) serveRequest() {
	var err error
	c.timer = c.clock.NewTimer(c.support.SharedConfig().BatchTimeout())
	if !c.timer.Stop() {
		<-c.timer.C()
	}

	for {
		seq := c.support.Sequence()
		err = nil

		select {
		case msg := <-c.submitC:
			if isConfig(msg.Content) {
				c.logger.Panicf("Processing config envelope is not implemented yet")
			}

			if msg.LastValidationSeq < seq {
				if _, err = c.support.ProcessNormalMsg(msg.Content); err != nil {
					c.logger.Warningf("Discarding bad normal message: %s", err)
					continue
				}
			}

			batches, _ := c.support.BlockCutter().Ordered(msg.Content)
			if len(batches) == 0 {
				if c.timer == nil {
					c.timer = c.clock.NewTimer(c.support.SharedConfig().BatchTimeout())
				}
				// batches still of zero length
				continue
			}

			c.timer = nil
			if err = c.commitBatches(batches...); err != nil {
				c.logger.Errorf("Failed to commit block: %s", err)
			}

		case <-c.timer.C():
			c.timer = nil

			batch := c.support.BlockCutter().Cut()
			if len(batch) == 0 {
				c.logger.Warningf("Batch timer expired with no pending requests, this might indicate a bug")
				continue
			}

			c.logger.Debugf("Batch timer expired, creating block")
			if err = c.commitBatches(batch); err != nil {
				c.logger.Errorf("Failed to commit block: %s", err)
			}

		case <-c.exitC:
			c.logger.Infof("Stop serving requests")
			return
		}
	}
}

func (c *Chain) commitBatches(batches ...[]*common.Envelope) error {
	for _, batch := range batches {
		b := c.support.CreateNextBlock(batch)
		data := utils.MarshalOrPanic(b)
		if err := c.node.Propose(context.TODO(), data); err != nil {
			return fmt.Errorf("failed to propose data to raft: %s", err)
		}

		select {
		case block := <-c.commitC:
			if utils.IsConfigBlock(block) {
				c.logger.Panicf("Config block is not supported yet")
			} else {
				c.support.WriteBlock(block, nil)
			}

		case <-c.exitC:
			return nil
		}
	}

	return nil
}

func (c *Chain) serveRaft() {
	c.ticker = c.clock.NewTicker(c.opts.TickInterval)

	defer func() {
		c.logger.Infof("Stop raft node")
		c.ticker.Stop()
		c.node.Stop()
	}()

	for {
		select {
		case <-c.ticker.C():
			c.node.Tick()

		case rd := <-c.node.Ready():
			c.storage.Append(rd.Entries)
			// TODO send messages to other peers when we implement multi-node raft
			c.apply(c.entriesToApply(rd.CommittedEntries))
			c.node.Advance()

			if rd.SoftState != nil {
				newLead := atomic.LoadUint64(&rd.SoftState.Lead)
				if newLead != c.leader {
					c.logger.Infof("[node: %d] Raft leader changed: %d -> %d", c.raftID, c.leader, newLead)
					c.leader = newLead

					// notify external observer
					if c.observe != nil {
						c.observe <- newLead
					}
				}
			}

		case <-c.exitC:
			return
		}
	}
}

func (c *Chain) apply(ents []raftpb.Entry) {
	for i := range ents {
		switch ents[i].Type {
		case raftpb.EntryNormal:
			if len(ents[i].Data) == 0 {
				break
			}

			c.commitC <- utils.UnmarshalBlockOrPanic(ents[i].Data)

		case raftpb.EntryConfChange:
			var cc raftpb.ConfChange
			if err := cc.Unmarshal(ents[i].Data); err != nil {
				c.logger.Warnf("Failed to unmarshal ConfChange data: %s", err)
				continue
			}

			c.node.ApplyConfChange(cc)
		}

		c.appliedIndex = ents[i].Index
	}
}

// this is taken from coreos/contrib/raftexample/raft.go
func (c *Chain) entriesToApply(ents []raftpb.Entry) (nents []raftpb.Entry) {
	if len(ents) == 0 {
		return
	}

	firstIdx := ents[0].Index
	if firstIdx > c.appliedIndex+1 {
		c.logger.Fatalf("first index of committed entry[%d] should <= progress.appliedIndex[%d]+1", firstIdx, c.appliedIndex)
	}

	// If we do have unapplied entries in nents.
	//    |     applied    |       unapplied      |
	//    |----------------|----------------------|
	// firstIdx       appliedIndex              last
	if c.appliedIndex-firstIdx+1 < uint64(len(ents)) {
		nents = ents[c.appliedIndex-firstIdx+1:]
	}
	return nents
}

func isConfig(env *common.Envelope) bool {
	h, err := utils.ChannelHeader(env)
	if err != nil {
		panic("programming error: failed to extract channel header from envelope")
	}

	return h.Type == int32(common.HeaderType_CONFIG_UPDATE)
}
