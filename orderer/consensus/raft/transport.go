package raft

import (
	"context"

	"github.com/hyperledger/fabric/protos/orderer"
)

//go:generate counterfeiter . Transport
type Transport interface {
	Step(ctx context.Context, destination uint64, msg *orderer.StepRequest, opts ...grpc.CallOption) (*orderer.StepResponse, error)

	SendPropose(ctx context.Context, destination uint64, request *orderer.ProposeRequest, opts ...grpc.CallOption) error

	ReceiveProposeResponse(ctx context.Context, destination uint64, opts ...grpc.CallOption) (*orderer.ProposeResponse, error)
}
