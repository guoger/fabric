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
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/hyperledger/fabric/common/configtx"
	genesisconfig "github.com/hyperledger/fabric/common/configtx/tool/localconfig"
	"github.com/hyperledger/fabric/msp"
	mspmgmt "github.com/hyperledger/fabric/msp/mgmt"
	"github.com/hyperledger/fabric/orderer/localconfig"
	cb "github.com/hyperledger/fabric/protos/common"
	ab "github.com/hyperledger/fabric/protos/orderer"
	protosutils "github.com/hyperledger/fabric/protos/utils"

	"github.com/hyperledger/fabric/common/localmsp"
)

const (
	chainIDLength = 10
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

func randomChainID() string {
	b := make([]rune, chainIDLength)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func MakeNormalTx(chainID string, size int) *cb.Envelope {
	env, err := protosutils.CreateSignedEnvelope(
		cb.HeaderType_ENDORSER_TRANSACTION,
		chainID,
		localmsp.NewSigner(),
		&cb.Envelope{Payload: make([]byte, size*1000)},
		0,
		0,
	)
	if err != nil {
		panic(fmt.Errorf("Failed to create signed envelope because: %s", err))
	}

	return env
}

func CreateChain() string {
	benchmarkServer := GetBenchmarkServer()
	client := benchmarkServer.CreateBroadcastClient()
	defer client.Close()

	chainID := randomChainID()
	createChainTx, _ := configtx.MakeChainCreationTransaction(
		chainID,
		genesisconfig.SampleConsortiumName,
		signer,
		genesisconfig.SampleOrgName)
	client.SendRequest(createChainTx)
	if client.GetResponse().Status != cb.Status_SUCCESS {
		logger.Panicf("Failed to create chain: %s", chainID)
	}
	return chainID
}

func WaitForChains(chainIDs ...string) {
	benchmarkServer := GetBenchmarkServer()

	var scoutWG sync.WaitGroup
	scoutWG.Add(len(chainIDs))
	for _, chainID := range chainIDs {
		go func(chainID string) {
			logger.Infof("Scouting for chain: %s", chainID)
			for {
				scoutClient := benchmarkServer.CreateBroadcastClient()
				defer scoutClient.Close()

				scoutClient.SendRequest(MakeNormalTx(chainID, 1))
				if scoutClient.GetResponse().Status == cb.Status_SUCCESS {
					break
				}

				time.Sleep(time.Second)
			}
			scoutWG.Done()
		}(chainID)
	}
	scoutWG.Wait()
}

var seekOldest = &ab.SeekPosition{Type: &ab.SeekPosition_Oldest{Oldest: &ab.SeekOldest{}}}

func SeekAllBlocks(c *DeliverClient, chainID string, number uint64) error {
	env, err := protosutils.CreateSignedEnvelope(
		cb.HeaderType_DELIVER_SEEK_INFO,
		chainID,
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
