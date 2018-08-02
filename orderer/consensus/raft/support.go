package raft

import (
	"time"

	etcdraft "github.com/coreos/etcd/raft"
)

//go:generate counterfeiter . RaftSupport
type RaftSupport interface {
	NewTicker() Ticker
	RaftConfig
	Transport
}

//go:generate counterfeiter . Ticker
type Ticker interface {
	Signal() <-chan time.Time
	Stop()
}

//go:generate counterfeiter . RaftConfig
type RaftConfig interface {
	ChainID() string
	ID() uint64
	ElectionTick() int
	HeartbeatTick() int
	MaxSizePerMsg() uint64
	MaxInflightMsgs() int
	Peers() []etcdraft.Peer
	TickInterval() time.Duration
}

type raftSupport struct {
	RaftConfig
	Transport
}

func (r *raftSupport) NewTicker() Ticker {
	return &ticker{time.NewTicker(r.TickInterval())}
}

type ticker struct {
	*time.Ticker
}

func (tk *ticker) Signal() <-chan time.Time {
	return tk.C
}
