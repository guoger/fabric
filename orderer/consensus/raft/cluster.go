package raft

import (
	"context"
	"fmt"

	etcdraft "github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/gogo/protobuf/proto"
	"github.com/hyperledger/fabric/common/flogging"
	cb "github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/orderer"
	"github.com/op/go-logging"
)

const PkgLogID = "orderer/consensus/raft"

var logger *logging.Logger

func init() {
	logger = flogging.MustGetLogger(PkgLogID)
}

type RaftCluster interface {
	Start()

	Process(msg proto.Message) error
	SendC() <-chan *orderer.SubmitRequest

	Propose(block *cb.Block) error
	CommitC() <-chan *cb.Block

	Stop()
}

type cluster struct {
	sendC   chan *orderer.SubmitRequest
	commitC chan *cb.Block

	storage *etcdraft.MemoryStorage
	node    etcdraft.Node
	support RaftSupport
}

func NewRaftCluster(support RaftSupport) RaftCluster {
	return &cluster{
		sendC:   make(chan *orderer.SubmitRequest),
		commitC: make(chan *cb.Block),
		storage: etcdraft.NewMemoryStorage(),
		support: support,
	}
}

func (c *cluster) Start() {
	raftConfig := etcdraft.Config{
		ID:              c.support.NodeID(),
		ElectionTick:    c.support.ElectionTick(),
		HeartbeatTick:   c.support.HeartbeatTick(),
		Storage:         c.storage,
		MaxSizePerMsg:   c.support.MaxSizePerMsg(),
		MaxInflightMsgs: c.support.MaxInflightMsgs(),
		Logger:          logger,
	}
	c.node = etcdraft.StartNode(&raftConfig, c.support.Peers())

	go c.serveRaft()
}

func (c *cluster) Process(msg proto.Message) error {
	switch m := msg.(type) {
	case *orderer.SubmitRequest:
		state := c.node.Status().SoftState

		if state.Lead == 0 {
			return fmt.Errorf("there's no leader")
		}

		if state.RaftState == etcdraft.StateLeader {
			c.sendC <- m
			return nil
		}

		logger.Infof("Forwarding msg to leader: %d", state.Lead)
		return c.support.SendSubmitRequest(state.Lead, m)

	case *orderer.StepRequest:
		stepMsg := &raftpb.Message{}
		if err := proto.Unmarshal(m.Payload, stepMsg); err != nil {
			return fmt.Errorf("failed to unmarshal SubmitRequest payload to raft Message: %s", err)
		}

		return c.node.Step(context.TODO(), *stepMsg)

	default:
		return fmt.Errorf("unknown message type")
	}
}

func (c *cluster) SendC() <-chan *orderer.SubmitRequest {
	return c.sendC
}

func (c *cluster) CommitC() <-chan *cb.Block {
	return c.commitC
}

func (c *cluster) Propose(block *cb.Block) error {
	if c.node.Status().SoftState.RaftState != etcdraft.StateLeader {
		return fmt.Errorf("non-leader is not allowed to propose block")
	}

	data, err := proto.Marshal(block)
	if err != nil {
		return err
	}

	return c.node.Propose(context.TODO(), data)
}

func (c *cluster) Stop() {
	c.node.Stop()
}

func (c *cluster) serveRaft() {
	ticker := c.support.NewTicker()
	defer ticker.Stop()

	for {
		select {
		case <-ticker.Signal():
			c.node.Tick()

		case rd := <-c.node.Ready():
			c.storage.Append(rd.Entries)

			for _, msg := range rd.Messages {
				msgBytes, err := proto.Marshal(&msg)
				if err != nil {
					logger.Warningf("Failed to marshal raft message: %s", err)
				}

				_, err = c.support.Step(msg.To, &orderer.StepRequest{Channel: c.support.ChainID(), Payload: msgBytes})
				if err != nil {
					logger.Warningf("Failed to send StepRequest to %d, because: %s", msg.To, err)
				}
			}

			if len(rd.CommittedEntries) != 0 {
				for i := range rd.CommittedEntries {
					switch rd.CommittedEntries[i].Type {
					case raftpb.EntryNormal:
						if len(rd.CommittedEntries[i].Data) == 0 {
							continue
						}

						block := &cb.Block{}
						if err := proto.Unmarshal(rd.CommittedEntries[i].Data, block); err != nil {
							logger.Warningf("Failed to unmarshal entry to block: %s", err)
						} else {
							c.commitC <- block
						}

					case raftpb.EntryConfChange:
						var cc raftpb.ConfChange
						cc.Unmarshal(rd.CommittedEntries[i].Data)
						c.node.ApplyConfChange(cc)
					}
				}
			}

			c.node.Advance()
		}
	}
}
