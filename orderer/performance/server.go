/*
Copyright IBM Corp. 2017 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

                 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package performance

import (
	"io"
	"sync"

	cb "github.com/hyperledger/fabric/protos/common"
	ab "github.com/hyperledger/fabric/protos/orderer"
	"github.com/op/go-logging"
	"google.golang.org/grpc"
)

var logger = logging.MustGetLogger("orderer/benchmark")

type BenchmarkServer struct {
	server ab.AtomicBroadcastServer
	start  chan struct{}
	halt   chan struct{}
}

var server *BenchmarkServer
var once sync.Once

func GetBenchmarkServer() *BenchmarkServer {
	once.Do(func() {
		server = &BenchmarkServer{
			server: nil,
			start:  make(chan struct{}),
			halt:   make(chan struct{}),
		}
	})
	return server
}

func (bs *BenchmarkServer) Start() {
	bs.halt = make(chan struct{})
	close(bs.start) // signal waiters that service is registered

	// Block reading here to prevent process exit
	<-bs.halt
}

func (bs *BenchmarkServer) Halt() {
	logger.Info("Stopping benchmark server")
	bs.server = nil
	bs.start = make(chan struct{})
	close(bs.halt)
}

// Wait for service to be registered
func (bs *BenchmarkServer) WaitForService() {
	<-bs.start
}

func (bs *BenchmarkServer) RegisterService(s ab.AtomicBroadcastServer) {
	bs.server = s
}

func (bs *BenchmarkServer) CreateBroadcastClient() *BroadcastClient {
	client := &BroadcastClient{
		requestChan:  make(chan *cb.Envelope),
		responseChan: make(chan *ab.BroadcastResponse),
		errChan:      make(chan error),
	}
	go func() {
		client.errChan <- bs.server.Broadcast(client)
	}()
	return client
}

type BroadcastClient struct {
	grpc.ServerStream
	requestChan  chan *cb.Envelope
	responseChan chan *ab.BroadcastResponse
	errChan      chan error
}

func (bc *BroadcastClient) SendRequest(request *cb.Envelope) {
	// TODO make this async
	bc.requestChan <- request
}

func (bc *BroadcastClient) GetResponse() *ab.BroadcastResponse {
	return <-bc.responseChan
}

func (bc *BroadcastClient) Close() {
	close(bc.requestChan)
}

func (bc *BroadcastClient) Errors() <-chan error {
	return bc.errChan
}

// Implement AtomicBroadcast_BroadcastServer interface
func (bc *BroadcastClient) Send(br *ab.BroadcastResponse) error {
	bc.responseChan <- br
	return nil
}

func (bc *BroadcastClient) Recv() (*cb.Envelope, error) {
	msg, ok := <-bc.requestChan
	if !ok {
		return msg, io.EOF
	}
	return msg, nil
}

func (bs *BenchmarkServer) CreateDeliverClient() *DeliverClient {
	client := &DeliverClient{
		requestChan:  make(chan *cb.Envelope),
		ResponseChan: make(chan *ab.DeliverResponse),
		ResultChan:   make(chan error),
	}
	go func() {
		client.ResultChan <- bs.server.Deliver(client)
	}()
	return client
}

type DeliverClient struct {
	grpc.ServerStream
	requestChan  chan *cb.Envelope
	ResponseChan chan *ab.DeliverResponse
	ResultChan   chan error
}

func (bc *DeliverClient) SendRequest(request *cb.Envelope) {
	// TODO make this async
	bc.requestChan <- request
}

func (bc *DeliverClient) GetResponse() *ab.DeliverResponse {
	return <-bc.ResponseChan
}

func (bc *DeliverClient) Close() {
	close(bc.requestChan)
}

// Implement AtomicBroadcast_BroadcastServer interface
func (bc *DeliverClient) Send(br *ab.DeliverResponse) error {
	bc.ResponseChan <- br
	return nil
}

func (bc *DeliverClient) Recv() (*cb.Envelope, error) {
	msg, ok := <-bc.requestChan
	if !ok {
		return msg, io.EOF
	}
	return msg, nil
}
