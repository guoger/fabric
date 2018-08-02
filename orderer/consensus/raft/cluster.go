package raft

import (
	"context"
	"fmt"
	"github.com/coreos/etcd/raft/raftpb"

	etcdraft "github.com/coreos/etcd/raft"
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
	Submit(req *orderer.SubmitRequest) error
	Propose(block *cb.Block) error
	CommitC() <-chan *cb.Block
	Dispatch(ctx context.Context, methodName string, msg proto.Message) (proto.Message, error)
	Stop()
}

type cluster struct {
	sendC   chan<- *orderer.SubmitRequest
	commitC chan *cb.Block

	storage *etcdraft.MemoryStorage
	node    etcdraft.Node
	support RaftSupport
}

func NewRaftCluster(support RaftSupport, sendC chan<- *orderer.SubmitRequest) RaftCluster {
	return &cluster{
		sendC:   sendC,
		commitC: make(chan *cb.Block),
		storage: etcdraft.NewMemoryStorage(),
		support: support,
	}
}

func (c *cluster) Start() {
	raftConfig := etcdraft.Config{
		ID:              c.support.ID(),
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

func (c *cluster) CommitC() <-chan *cb.Block {
	return c.commitC
}

func (c *cluster) Submit(msg *orderer.SubmitRequest) error {
	state := c.node.Status().SoftState

	if state.Lead == 0 {
		return fmt.Errorf("there's no leader")
	}

	if state.RaftState == etcdraft.StateLeader {
		c.sendC <- msg
		return nil
	}

	logger.Debugf("Forwarding msg to leader: %d", state.Lead)
	return c.support.SendPropose(context.TODO(), state.Lead, msg)
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
					logger.Warningf("failed to marshal raft message: %s", err)
				}
				c.support.Step(context.Background(), msg.To, &orderer.StepRequest{Channel: c.support.ChainID(), Payload: msgBytes})
			}

			if len(rd.CommittedEntries) != 0 {
				for i := range rd.CommittedEntries {
					switch rd.CommittedEntries[i].Type {
					case raftpb.EntryNormal:
						if rd.CommittedEntries[i].Data == nil {
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

func (c *cluster) Dispatch(ctx context.Context, methodName string, msg proto.Message) (proto.Message, error) {
	switch methodName {
	case "Submit":
		if msg, ok := msg.(*orderer.SubmitRequest); !ok {
			return nil, fmt.Errorf("expect message type of SubmitRequest, got %T", msg)
		}

		return nil, c.Submit(msg)
	case "Step":
		if msg, ok := msg.(*orderer.StepRequest); !ok {
			return nil, fmt.Errorf("expect message type of StepRequest, got %T", msg)
		}

		raftMsg := &raftpb.Message{}
		if err := proto.Unmarshal(msg.Payload, raftMsg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal payload into raft message: %s", err)
		}

		return nil, c.node.Step(ctx, *raftMsg)
	default:
		return nil, fmt.Errorf("method name '%s' is not recognized", methodName)
	}
}
