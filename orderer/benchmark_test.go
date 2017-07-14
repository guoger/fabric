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

package main

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/hyperledger/fabric/common/configtx/tool/localconfig"
	"github.com/hyperledger/fabric/common/configtx/tool/provisional"
	perf "github.com/hyperledger/fabric/orderer/performance"
	cb "github.com/hyperledger/fabric/protos/common"
	"github.com/stretchr/testify/assert"
	"strconv"
)

const (
	LogLevel        = "error"
	MaxMessageCount = 10
	Solo            = "solo"
	Kafka           = "kafka"

	// This cannot be less than ~13 KB, otherwise chain creation msg would be rejected
	AbsoluteMaxBytes  = 15 // KB
	PreferredMaxBytes = 5  // KB
)

var (
	channelCounts             []int = []int{2, 10}
	messageCounts             []int = []int{5000, 10000}
	messagesSizes             []int = []int{1, 10}
	broadcastClientPerChannel []int = []int{1, 10}
	deliverClientPerChannel   []int = []int{1, 10}

	//channelCounts             []int = []int{2}
	//messageCounts             []int = []int{40}
	//messagesSizes             []int = []int{1}
	//broadcastClientPerChannel []int = []int{1}
	//deliverClientPerChannel   []int = []int{1}

	args [][]int = [][]int{channelCounts, messageCounts, messagesSizes, broadcastClientPerChannel, deliverClientPerChannel}

	envvars map[string]string = map[string]string{
		"ORDERER_GENERAL_GENESISPROFILE":                            localconfig.SampleDevModeSolo,
		"ORDERER_GENERAL_LEDGERTYPE":                                "ram",
		"ORDERER_GENERAL_LOGLEVEL":                                  LogLevel,
		"ORDERER_FILELEDGER_LOCATION":                               "/tmp/hyperledger-benchmark",
		localconfig.Prefix + "_ORDERER_ORDERERTYPE":                 Solo,
		localconfig.Prefix + "_ORDERER_BATCHSIZE_MAXMESSAGECOUNT":   strconv.Itoa(MaxMessageCount),
		localconfig.Prefix + "_ORDERER_BATCHSIZE_ABSOLUTEMAXBYTES":  strconv.Itoa(AbsoluteMaxBytes) + " KB",
		localconfig.Prefix + "_ORDERER_BATCHSIZE_PREFERREDMAXBYTES": strconv.Itoa(PreferredMaxBytes) + " KB",
	}
)

type factors struct {
	numOfChannels             int // number of channels
	messageCount              int // total number of messages
	messageSize               int // message size in KB
	broadcastClientPerChannel int // concurrent broadcast clients
	deliverClientPerChannel   int // concurrent deliver clients
}

func TestOrdererBenchmark(t *testing.T) {
	// clean block files after test
	defer os.RemoveAll(envvars["ORDERER_FILELEDGER_LOCATION"])

	if os.Getenv("BENCHMARK") == "" {
		t.Skip("Skipping benchmark test")
	}

	for key, value := range envvars {
		os.Setenv(key, value)
	}

	for factors := range testArgs() {
		t.Run("orderer_benchmark", func(t *testing.T) {
			benchmarkBroadcast(
				t,
				factors.numOfChannels,
				factors.messageCount,
				factors.messageSize,
				factors.broadcastClientPerChannel,
				factors.deliverClientPerChannel,
				true,
			)
		})
	}
}

func benchmarkBroadcast(
	t *testing.T,
	numOfChannels int,
	messages int,
	msgSize int,
	broadcastClientPerChannel int,
	deliverClientPerChannel int,
	multiplex bool,
) {
	// Compute intermediate parameters
	txPerChannelPerClient := messages / (broadcastClientPerChannel * numOfChannels)
	totalNumOfTx := txPerChannelPerClient * broadcastClientPerChannel * numOfChannels
	txPerChannel := totalNumOfTx / numOfChannels
	// Message size consists of payload and signature
	msg := perf.MakeNormalTx("abcdefghij", msgSize)
	actualMsgSize := (len(msg.Payload) + len(msg.Signature))/1000
	// max(1, x) in case a block can only contain exactly one tx
	txPerBlk := min(MaxMessageCount, max(1, PreferredMaxBytes/actualMsgSize))
	// Round-down here is ok because we don't really care about trailing tx
	blkPerChannel := txPerChannel / txPerBlk

	// Start orderer main process
	os.Args = []string{"orderer", "benchmark"}
	go main()

	// Get server instance
	benchmarkServer := perf.GetBenchmarkServer()
	defer benchmarkServer.Halt()

	// Wait for server to boot and systemchain to be ready
	benchmarkServer.WaitForService()
	perf.WaitForChains(provisional.TestChainID)

	// Create chains
	chainIDs := make([]string, numOfChannels)
	txs := make(map[string]*cb.Envelope)
	for i := 0; i < numOfChannels; i++ {
		id := perf.CreateChain()
		chainIDs[i] = id
		txs[id] = perf.MakeNormalTx(id, msgSize)
	}

	// Wait for all the created chains to be ready
	perf.WaitForChains(chainIDs...)

	// Broadcast
	broadcast := func(wg *sync.WaitGroup) {
		var broadcastWG sync.WaitGroup
		broadcastWG.Add(numOfChannels)
		for _, chainID := range chainIDs {
			go func(chainID string) {
				var clientWG sync.WaitGroup
				clientWG.Add(broadcastClientPerChannel + 1)

				// Spawn a deliver instance per channel to track the progress of broadcast
				go func(chainID string) {
					deliverClient := benchmarkServer.CreateDeliverClient()
					assert.NoError(
						t,
						perf.SeekAllBlocks(deliverClient, chainID, uint64(blkPerChannel)),
						"Expect deliver to succeed")
					clientWG.Done()
				}(chainID)

				for c := 0; c < broadcastClientPerChannel; c++ {
					go func() {
						broadcastClient := benchmarkServer.CreateBroadcastClient()
						defer func() {
							broadcastClient.Close()
							err := <-broadcastClient.Errors()
							assert.NoError(t, err, "Expect broadcast handler to shutdown gracefully")
						}()

						for i := 0; i < txPerChannelPerClient; i++ {
							broadcastClient.SendRequest(txs[chainID])
							assert.Equal(t, cb.Status_SUCCESS, broadcastClient.GetResponse().Status, "Expect enqueue to succeed")
						}
						clientWG.Done()
					}()
				}

				clientWG.Wait()
				broadcastWG.Done()
			}(chainID)
		}

		broadcastWG.Wait()

		if wg != nil {
			wg.Done()
		}
	}

	// Deliver
	deliver := func(wg *sync.WaitGroup) {
		var deliverWG sync.WaitGroup
		deliverWG.Add(deliverClientPerChannel * numOfChannels)
		for g := 0; g < deliverClientPerChannel; g++ {
			go func() {
				for _, chainID := range chainIDs {
					go func(chainID string) {
						deliverClient := benchmarkServer.CreateDeliverClient()
						assert.NoError(
							t,
							perf.SeekAllBlocks(deliverClient, chainID, uint64(blkPerChannel)),
							"Expect deliver to succeed")

						deliverWG.Done()
					}(chainID)
				}
			}()
		}
		deliverWG.Wait()

		if wg != nil {
			wg.Done()
		}
	}

	var wg sync.WaitGroup
	var btime, dtime time.Duration

	if multiplex {
		// Parallel
		start := time.Now()

		wg.Add(2)
		go broadcast(&wg)
		go deliver(&wg)
		wg.Wait()

		btime = time.Since(start)
		dtime = time.Since(start)
	} else {
		// Serial
		start := time.Now()
		broadcast(nil)
		btime = time.Since(start)

		start = time.Now()
		deliver(nil)
		dtime = time.Since(start)
	}

	fmt.Printf(
		"Message: %6d  Message Size: %3dKB  Channels: %3d | "+
			"Broadcast Clients: %3d  Write tps: %5.1f tx/s Elapsed Time: %0.2fs | "+
			"Deliver clients: %3d  Read tps: %8.1f blk/s Elapsed Time: %0.2fs\n",
		totalNumOfTx,
		msgSize,
		numOfChannels,
		broadcastClientPerChannel*numOfChannels,
		float64(totalNumOfTx)/btime.Seconds(),
		btime.Seconds(),
		deliverClientPerChannel*numOfChannels,
		float64(blkPerChannel*deliverClientPerChannel*numOfChannels)/dtime.Seconds(),
		dtime.Seconds())
}

func testArgs() <-chan factors {
	ch := make(chan factors)
	go func() {
		defer close(ch)
		for c := range combinations(args) {
			ch <- factors{
				numOfChannels:             c[4],
				messageCount:              c[3],
				messageSize:               c[2],
				broadcastClientPerChannel: c[1],
				deliverClientPerChannel:   c[0],
			}
		}
	}()
	return ch
}

func combinations(args [][]int) <-chan []int {
	ch := make(chan []int)
	go func() {
		defer close(ch)
		if len(args) == 1 {
			for _, i := range args[0] {
				ch <- []int{i}
			}
		} else {
			for _, i := range args[0] {
				for j := range combinations(args[1:]) {
					ch <- append(j, i)
				}
			}
		}
	}()
	return ch
}

func min(x, y int) int {
	if x > y {
		return y
	}
	return x
}

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}
