/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package server

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"io/ioutil"
	"runtime/pprof"

	"github.com/hyperledger/fabric/common/configtx/tool/localconfig"
	"github.com/hyperledger/fabric/common/configtx/tool/provisional"
	"github.com/hyperledger/fabric/orderer/common/localconfig"
	perf "github.com/hyperledger/fabric/orderer/common/performance"
	cb "github.com/hyperledger/fabric/protos/common"
	"github.com/stretchr/testify/assert"
)

// Usage: BENCHMARK=true go test -run=TestOrdererBenchmark[Solo|Kafka]
//
// Benchmark test makes [a] channels, creates [b] clients for each channel. There are
// [e] orderer instances in total. A client ONLY interacts with ONE channel and ONE
// orderer, so the number of client in total is [a * b * e].
//
// The test sends [c] transactions of size [d] in total. These tx are evenly distributed
// among every clients, which is [c / (a * b * e)] tx per client.
//
// Additionally, [f] deliver clients per channel seeks all the blocks in that channel.
// It would 'predict' the last block number, and seek from the oldest to that. In this
// manner, we could reliably assert that all transactions we send are ordered. This is
// important for evaluating elapsed time of async broadcast operations.
//
// Again, each deliver client only interacts with one channel and one orderer, which
// results in [a * f * e] deliver clients in total.
//
// a -> channelCounts
// b -> broadcastClientPerChannel
// c -> totalTx
// d -> messagesSizes
// e -> numOfOrderer
// f -> deliverClientPerChannel
//
// Note: a Kafka broker listening on localhost:9092 is required to run Kafka based benchmark
// TODO(jay_guo) use ephemeral kafka container for test

const (
	MaxMessageCount = 10

	// This cannot be less than ~13 KB, otherwise channel creation msg would be rejected
	AbsoluteMaxBytes  = 15 // KB
	PreferredMaxBytes = 10 // KB

	// If multiplex is true, broadcast and deliver are running simultaneously.
	// Otherwise, broadcast is run before deliver. This is useful when testing
	// deliver performance.
	multiplex = true
)

var (
	//channelCounts             = []int{1, 10}
	//totalTx                   = []int{10000}
	//messagesSizes             = []int{1}
	//broadcastClientPerChannel = []int{1, 5, 10}
	//deliverClientPerChannel   = []int{0, 5, 10}
	//numOfOrderer              = []int{1, 5}

	channelCounts             = []int{10}
	totalTx                   = []int{50000}
	messagesSizes             = []int{1}
	broadcastClientPerChannel = []int{10}
	deliverClientPerChannel   = []int{0}
	numOfOrderer              = []int{5}

	args = [][]int{
		channelCounts,
		totalTx,
		messagesSizes,
		broadcastClientPerChannel,
		deliverClientPerChannel,
		numOfOrderer,
	}

	envvars = map[string]string{
		"ORDERER_GENERAL_GENESISPROFILE":                            localconfig.SampleDevModeSolo,
		"ORDERER_GENERAL_LEDGERTYPE":                                "ram",
		"ORDERER_GENERAL_LOGLEVEL":                                  "error",
		localconfig.Prefix + "_ORDERER_BATCHSIZE_MAXMESSAGECOUNT":   strconv.Itoa(MaxMessageCount),
		localconfig.Prefix + "_ORDERER_BATCHSIZE_ABSOLUTEMAXBYTES":  strconv.Itoa(AbsoluteMaxBytes) + " KB",
		localconfig.Prefix + "_ORDERER_BATCHSIZE_PREFERREDMAXBYTES": strconv.Itoa(PreferredMaxBytes) + " KB",
		localconfig.Prefix + "_ORDERER_KAFKA_BROKERS":               "[localhost:9092]",
	}
)

type factors struct {
	numOfChannels             int // number of channels
	totalTx                   int // total number of messages
	messageSize               int // message size in KB
	broadcastClientPerChannel int // concurrent broadcast clients
	deliverClientPerChannel   int // concurrent deliver clients
	numOfOrderer              int // number of orderer instances (Kafka ONLY)
}

func TestOrdererBenchmarkSolo(t *testing.T) {
	if os.Getenv("BENCHMARK") == "" {
		t.Skip("Skipping benchmark test")
	}

	os.Setenv(localconfig.Prefix+"_ORDERER_ORDERERTYPE", provisional.ConsensusTypeSolo)

	for key, value := range envvars {
		os.Setenv(key, value)
	}

	cpupprof, err := os.Create("/tmp/orderercpu.prof")
	assert.NoError(t, err, "Should be able to create pprof file")

	pprof.StartCPUProfile(cpupprof)
	defer func() {
		pprof.StopCPUProfile()
		cpupprof.Close()
	}()

	for factors := range testArgs() {
		t.Run("orderer_benchmark", func(t *testing.T) {
			benchmarkBroadcast(
				t,
				factors.numOfChannels,
				factors.totalTx,
				factors.messageSize,
				factors.broadcastClientPerChannel,
				factors.deliverClientPerChannel,
				1, // For solo orderer, we should always have exactly one instance
				multiplex,
			)

			mempprof, err := os.Create("/tmp/orderermem.prof")
			defer mempprof.Close()
			assert.NoError(t, err, "Should be able to create pprof file")
			pprof.WriteHeapProfile(mempprof)
		})
	}
}

func TestOrdererBenchmarkKafka(t *testing.T) {
	if os.Getenv("BENCHMARK") == "" {
		t.Skip("Skipping benchmark test")
	}

	os.Setenv(localconfig.Prefix+"_ORDERER_ORDERERTYPE", provisional.ConsensusTypeKafka)

	for key, value := range envvars {
		os.Setenv(key, value)
	}

	for factors := range testArgs() {
		t.Run("orderer_benchmark", func(t *testing.T) {
			benchmarkBroadcast(
				t,
				factors.numOfChannels,
				factors.totalTx,
				factors.messageSize,
				factors.broadcastClientPerChannel,
				factors.deliverClientPerChannel,
				factors.numOfOrderer,
				multiplex,
			)
		})
	}
}

func benchmarkBroadcast(
	t *testing.T,
	numOfChannels int,
	totalTx int,
	msgSize int,
	broadcastClientPerChannel int,
	deliverClientPerChannel int,
	numOfOrderer int,
	multiplex bool,
) {
	// If we are using json or file ledger, we should point ledger location to "/tmp"
	// because default location "/var/hyperledger/production/orderer" in sample config
	// isn't always writable. Also the temp dir is cleaned up after each test run
	if os.Getenv("ORDERER_GENERAL_LEDGERTYPE") != "ram" {
		tempDir, err := ioutil.TempDir("", "fabric-benchmark-test-")
		assert.NoError(t, err, "Should be able to create temp dir")
		os.Setenv("ORDERER_FILELEDGER_LOCATION", tempDir)
		defer os.RemoveAll(tempDir)
	}

	txPerClient := totalTx / (broadcastClientPerChannel * numOfChannels * numOfOrderer)
	totalTx = txPerClient * broadcastClientPerChannel * numOfChannels * numOfOrderer
	txPerChannel := totalTx / numOfChannels
	// Message size consists of payload and signature
	msg := perf.MakeNormalTx("abcdefghij", msgSize)
	actualMsgSize := len(msg.Payload) + len(msg.Signature)
	// max(1, x) in case a block can only contain exactly one tx
	txPerBlk := min(MaxMessageCount, max(1, PreferredMaxBytes*perf.Kilo/actualMsgSize))
	// Round-down here is ok because we don't really care about trailing tx
	blkPerChannel := txPerChannel / txPerBlk
	var txCount uint64

	// Initialization shared by all orderers
	conf := config.Load()
	initializeLoggingLevel(conf)
	initializeLocalMsp(conf)
	perf.InitializeServerPool(numOfOrderer)

	// Generate a random system channel id for each test run,
	// so it does not recover ledgers from previous run.
	systemchannel := "system-channel-" + perf.RandomID(5)
	conf.General.SystemChannel = systemchannel

	// Spawn orderers
	for i := 0; i < numOfOrderer; i++ {
		go Start("benchmark", conf)
	}
	defer perf.OrdererExec(perf.Halt)

	// Wait for server to boot and systemchannel to be ready
	perf.OrdererExec(perf.WaitForService)
	perf.OrdererExecWithArgs(perf.WaitForChannels, systemchannel)

	// Create channels
	benchmarkServers := perf.GetBenchmarkServerPool()
	channelIDs := make([]string, numOfChannels)
	txs := make(map[string]*cb.Envelope)
	for i := 0; i < numOfChannels; i++ {
		id := perf.CreateChannel(benchmarkServers[0]) // We only need to create channel on one orderer
		channelIDs[i] = id
		txs[id] = perf.MakeNormalTx(id, msgSize)
	}

	// Wait for all the created channels to be ready
	perf.OrdererExecWithArgs(perf.WaitForChannels, stoi(channelIDs)...)

	// Broadcast loop
	broadcast := func(wg *sync.WaitGroup) {
		perf.OrdererExec(func(server *perf.BenchmarkServer) {
			var broadcastWG sync.WaitGroup
			// Since the submission and ordering of transactions are async, we need to
			// spawn a deliver client to track the progress of ordering. So there are
			// x broadcast clients and 1 deliver client per channel, so we should be
			// waiting for (numOfChannels * (x + 1)) goroutines here
			broadcastWG.Add(numOfChannels * (broadcastClientPerChannel + 1))

			for _, channelID := range channelIDs {
				go func(channelID string) {
					// Spawn a deliver instance per channel to track the progress of broadcast
					go func() {
						deliverClient := server.CreateDeliverClient()
						assert.NoError(
							t,
							perf.SeekAllBlocks(deliverClient, channelID, uint64(blkPerChannel)),
							"Expect deliver to succeed")
						broadcastWG.Done()
					}()

					for c := 0; c < broadcastClientPerChannel; c++ {
						go func() {
							broadcastClient := server.CreateBroadcastClient()
							defer func() {
								broadcastClient.Close()
								err := <-broadcastClient.Errors()
								assert.NoError(t, err, "Expect broadcast handler to shutdown gracefully")
							}()

							for i := 0; i < txPerClient; i++ {
								atomic.AddUint64(&txCount, 1)
								broadcastClient.SendRequest(txs[channelID])
								assert.Equal(t, cb.Status_SUCCESS, broadcastClient.GetResponse().Status, "Expect enqueue to succeed")
							}
							broadcastWG.Done()
						}()
					}
				}(channelID)
			}

			broadcastWG.Wait()
		})

		if wg != nil {
			wg.Done()
		}
	}

	// Deliver loop
	deliver := func(wg *sync.WaitGroup) {
		perf.OrdererExec(func(server *perf.BenchmarkServer) {
			var deliverWG sync.WaitGroup
			deliverWG.Add(deliverClientPerChannel * numOfChannels)
			for g := 0; g < deliverClientPerChannel; g++ {
				go func() {
					for _, channelID := range channelIDs {
						go func(channelID string) {
							deliverClient := server.CreateDeliverClient()
							assert.NoError(
								t,
								perf.SeekAllBlocks(deliverClient, channelID, uint64(blkPerChannel)),
								"Expect deliver to succeed")

							deliverWG.Done()
						}(channelID)
					}
				}()
			}
			deliverWG.Wait()
		})

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

	// Assert here to guard against programming error caused by miscalculation of message count.
	// Experiment shows that atomic counter is not bottleneck.
	assert.Equal(t, uint64(totalTx), txCount, "Expected to send %d msg, but actually sent %d", uint64(totalTx), txCount)

	fmt.Printf(
		"Message: %6d  Message Size: %3dKB  Channels: %3d Orderer: %2d | "+
			"Broadcast Clients: %3d  Write tps: %5.1f tx/s Elapsed Time: %0.2fs | "+
			"Deliver clients: %3d  Read tps: %8.1f blk/s Elapsed Time: %0.2fs\n",
		totalTx,
		msgSize,
		numOfChannels,
		numOfOrderer,
		broadcastClientPerChannel*numOfChannels*numOfOrderer,
		float64(totalTx)/btime.Seconds(),
		btime.Seconds(),
		deliverClientPerChannel*numOfChannels*numOfOrderer,
		float64(blkPerChannel*deliverClientPerChannel*numOfChannels)/dtime.Seconds(),
		dtime.Seconds())
}

func testArgs() <-chan factors {
	ch := make(chan factors)
	go func() {
		defer close(ch)
		for c := range combinations(args) {
			ch <- factors{
				numOfChannels:             c[0],
				totalTx:                   c[1],
				messageSize:               c[2],
				broadcastClientPerChannel: c[3],
				deliverClientPerChannel:   c[4],
				numOfOrderer:              c[5],
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
					ch <- append([]int{i}, j...)
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

func stoi(s []string) (ret []interface{}) {
	ret = make([]interface{}, len(s))
	for i, d := range s {
		ret[i] = d
	}
	return
}
