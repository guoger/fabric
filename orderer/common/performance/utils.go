/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package performance

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/hyperledger/fabric/common/configtx"
	genesisconfig "github.com/hyperledger/fabric/common/configtx/tool/localconfig"
	"github.com/hyperledger/fabric/common/localmsp"
	"github.com/hyperledger/fabric/msp"
	mspmgmt "github.com/hyperledger/fabric/msp/mgmt"
	"github.com/hyperledger/fabric/orderer/common/localconfig"
	cb "github.com/hyperledger/fabric/protos/common"
	ab "github.com/hyperledger/fabric/protos/orderer"
	protosutils "github.com/hyperledger/fabric/protos/utils"
)

const (
	// Kilo = 1000
	Kilo = 1e3 // TODO(jay_guo) consider adding an unit pkg
)

var conf *config.TopLevel
var signer msp.SigningIdentity

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz")

func init() {
	rand.Seed(time.Now().UnixNano())

	conf = config.Load()

	// Load local MSP
	err := mspmgmt.LoadLocalMsp(conf.General.LocalMSPDir, conf.General.BCCSP, conf.General.LocalMSPID)
	if err != nil {
		panic(fmt.Errorf("Failed to initialize local MSP: %s", err))
	}

	msp := mspmgmt.GetLocalMSP()
	signer, err = msp.GetDefaultSigningIdentity()
	if err != nil {
		panic(fmt.Errorf("Failed to initialize get default signer: %s", err))
	}
}

// MakeNormalTx creates a properly signed transaction that could be used against `broadcast` API
func MakeNormalTx(channelID string, size int) *cb.Envelope {
	env, err := protosutils.CreateSignedEnvelope(
		cb.HeaderType_ENDORSER_TRANSACTION,
		channelID,
		localmsp.NewSigner(),
		&cb.Envelope{Payload: make([]byte, size*Kilo)},
		0,
		0,
	)
	if err != nil {
		panic(fmt.Errorf("Failed to create signed envelope because: %s", err))
	}

	return env
}

// OrdererExecWithArgs executes func for each orderer in parallel
func OrdererExecWithArgs(f func(s *BenchmarkServer, i ...interface{}), i ...interface{}) {
	servers := GetBenchmarkServerPool()
	var wg sync.WaitGroup
	wg.Add(len(servers))
	for _, server := range servers {
		go func(server *BenchmarkServer) {
			f(server, i...)
			wg.Done()
		}(server)
	}
	wg.Wait()
}

// OrdererExec executes func for each orderer in parallel
func OrdererExec(f func(s *BenchmarkServer)) {
	servers := GetBenchmarkServerPool()
	var wg sync.WaitGroup
	wg.Add(len(servers))
	for _, server := range servers {
		go func(server *BenchmarkServer) {
			f(server)
			wg.Done()
		}(server)
	}
	wg.Wait()
}

// RandomID generates a random string of num chars
func RandomID(num int) string {
	b := make([]rune, num)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

// CreateChannel creates a channel with randomly generated ID of length 10
func CreateChannel(server *BenchmarkServer) string {
	client := server.CreateBroadcastClient()
	defer client.Close()

	channelID := RandomID(10)
	createChannelTx, _ := configtx.MakeChainCreationTransaction(
		channelID,
		genesisconfig.SampleConsortiumName,
		signer,
		genesisconfig.SampleOrgName)
	client.SendRequest(createChannelTx)
	if client.GetResponse().Status != cb.Status_SUCCESS {
		logger.Panicf("Failed to create channel: %s", channelID)
	}
	return channelID
}

// WaitForChannels probes a channel till it's ready
func WaitForChannels(server *BenchmarkServer, channelIDs ...interface{}) {
	var scoutWG sync.WaitGroup
	scoutWG.Add(len(channelIDs))
	for _, channelID := range channelIDs {
		id, ok := channelID.(string)
		if !ok {
			panic("Expect a string as channelID")
		}
		go func(channelID string) {
			logger.Infof("Scouting for channel: %s", channelID)
			for {
				scoutClient := server.CreateBroadcastClient()
				defer scoutClient.Close()

				scoutClient.SendRequest(MakeNormalTx(channelID, 1))
				if scoutClient.GetResponse().Status == cb.Status_SUCCESS {
					break
				}

				time.Sleep(time.Second * 5)
			}
			logger.Infof("Channel %s is ready", channelID)
			scoutWG.Done()
		}(id)
	}
	scoutWG.Wait()
}

var seekOldest = &ab.SeekPosition{Type: &ab.SeekPosition_Oldest{Oldest: &ab.SeekOldest{}}}

// SeekAllBlocks seeks block from oldest to specified number
func SeekAllBlocks(c *DeliverClient, channelID string, number uint64) error {
	env, err := protosutils.CreateSignedEnvelope(
		cb.HeaderType_DELIVER_SEEK_INFO,
		channelID,
		localmsp.NewSigner(),
		&ab.SeekInfo{Start: seekOldest, Stop: seekSpecified(number), Behavior: ab.SeekInfo_BLOCK_UNTIL_READY},
		0,
		0,
	)
	if err != nil {
		panic(fmt.Errorf("Failed to create signed envelope because: %s", err))
	}

	c.SendRequest(env)

	for {
		select {
		case reply := <-c.ResponseChan:
			if reply.GetBlock() == nil && reply.GetStatus() == cb.Status_SUCCESS {
				c.Close()
			}
		case err := <-c.ResultChan:
			return err
		}
	}
}

func seekSpecified(number uint64) *ab.SeekPosition {
	return &ab.SeekPosition{Type: &ab.SeekPosition_Specified{Specified: &ab.SeekSpecified{Number: number}}}
}
