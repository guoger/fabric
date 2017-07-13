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

package utils

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"

	genesisconfig "github.com/hyperledger/fabric/common/configtx/tool/localconfig"
	"github.com/hyperledger/fabric/common/configtx/tool/provisional"
	"github.com/hyperledger/fabric/common/crypto"
	"github.com/hyperledger/fabric/common/ledger/blkstorage/fsblkstorage"
	"github.com/hyperledger/fabric/core/comm"
	mspmgmt "github.com/hyperledger/fabric/msp/mgmt"
	"github.com/hyperledger/fabric/orderer/common/bootstrap/file"
	"github.com/hyperledger/fabric/orderer/kafka"
	"github.com/hyperledger/fabric/orderer/ledger"
	fileledger "github.com/hyperledger/fabric/orderer/ledger/file"
	jsonledger "github.com/hyperledger/fabric/orderer/ledger/json"
	ramledger "github.com/hyperledger/fabric/orderer/ledger/ram"
	config "github.com/hyperledger/fabric/orderer/localconfig"
	"github.com/hyperledger/fabric/orderer/multichain"
	"github.com/hyperledger/fabric/orderer/solo"
	cb "github.com/hyperledger/fabric/protos/common"
	protosutils "github.com/hyperledger/fabric/protos/utils"
	logging "github.com/op/go-logging"
)

var logger = logging.MustGetLogger("orderer/utils")

func CreateLedgerFactory(conf *config.TopLevel) (ledger.Factory, string) {
	var lf ledger.Factory
	var ld string
	switch conf.General.LedgerType {
	case "file":
		ld = conf.FileLedger.Location
		if ld == "" {
			ld = createTempDir(conf.FileLedger.Prefix)
		}
		logger.Debug("Ledger dir:", ld)
		lf = fileledger.New(ld)
		// The file-based ledger stores the blocks for each channel
		// in a fsblkstorage.ChainsDir sub-directory that we have
		// to create separately. Otherwise the call to the ledger
		// Factory's ChainIDs below will fail (dir won't exist).
		createSubDir(ld, fsblkstorage.ChainsDir)
	case "json":
		ld = conf.FileLedger.Location
		if ld == "" {
			ld = createTempDir(conf.FileLedger.Prefix)
		}
		logger.Debug("Ledger dir:", ld)
		lf = jsonledger.New(ld)
	case "ram":
		fallthrough
	default:
		lf = ramledger.New(int(conf.RAMLedger.HistorySize))
	}
	return lf, ld
}

func createTempDir(dirPrefix string) string {
	dirPath, err := ioutil.TempDir("", dirPrefix)
	if err != nil {
		logger.Panic("Error creating temp dir:", err)
	}
	return dirPath
}

func createSubDir(parentDirPath string, subDir string) (string, bool) {
	var created bool
	subDirPath := filepath.Join(parentDirPath, subDir)
	if _, err := os.Stat(subDirPath); err != nil {
		if os.IsNotExist(err) {
			if err = os.Mkdir(subDirPath, 0755); err != nil {
				logger.Panic("Error creating sub dir:", err)
			}
			created = true
		}
	} else {
		logger.Debugf("Found %s sub-dir and using it", fsblkstorage.ChainsDir)
	}
	return subDirPath, created
}

func InitializeMultiChainManager(conf *config.TopLevel, signer crypto.LocalSigner) multichain.Manager {
	lf, _ := CreateLedgerFactory(conf)
	// Are we bootstrapping?
	if len(lf.ChainIDs()) == 0 {
		initializeBootstrapChannel(conf, lf)
	} else {
		logger.Info("Not bootstrapping because of existing chains")
	}

	consenters := make(map[string]multichain.Consenter)
	consenters["solo"] = solo.New()
	consenters["kafka"] = kafka.New(conf.Kafka.TLS, conf.Kafka.Retry, conf.Kafka.Version)

	return multichain.NewManagerImpl(lf, consenters, signer)
}

func InitializeLocalMsp(conf *config.TopLevel) {
	// Load local MSP
	err := mspmgmt.LoadLocalMsp(conf.General.LocalMSPDir, conf.General.BCCSP, conf.General.LocalMSPID)
	if err != nil { // Handle errors reading the config file
		logger.Fatal("Failed to initialize local MSP:", err)
	}
}

func initializeBootstrapChannel(conf *config.TopLevel, lf ledger.Factory) {
	var genesisBlock *cb.Block

	// Select the bootstrapping mechanism
	switch conf.General.GenesisMethod {
	case "provisional":
		genesisBlock = provisional.New(genesisconfig.Load(conf.General.GenesisProfile)).GenesisBlock()
	case "file":
		genesisBlock = file.New(conf.General.GenesisFile).GenesisBlock()
	default:
		logger.Panic("Unknown genesis method:", conf.General.GenesisMethod)
	}

	chainID, err := protosutils.GetChainIDFromBlock(genesisBlock)
	if err != nil {
		logger.Fatal("Failed to parse chain ID from genesis block:", err)
	}
	gl, err := lf.GetOrCreate(chainID)
	if err != nil {
		logger.Fatal("Failed to create the system chain:", err)
	}

	err = gl.Append(genesisBlock)
	if err != nil {
		logger.Fatal("Could not write genesis block to ledger:", err)
	}
}

func initializeSecureServerConfig(conf *config.TopLevel) comm.SecureServerConfig {
	// secure server config
	secureConfig := comm.SecureServerConfig{
		UseTLS:            conf.General.TLS.Enabled,
		RequireClientCert: conf.General.TLS.ClientAuthEnabled,
	}
	// check to see if TLS is enabled
	if secureConfig.UseTLS {
		logger.Info("Starting orderer with TLS enabled")
		// load crypto material from files
		serverCertificate, err := ioutil.ReadFile(conf.General.TLS.Certificate)
		if err != nil {
			logger.Fatalf("Failed to load ServerCertificate file '%s' (%s)",
				conf.General.TLS.Certificate, err)
		}
		serverKey, err := ioutil.ReadFile(conf.General.TLS.PrivateKey)
		if err != nil {
			logger.Fatalf("Failed to load PrivateKey file '%s' (%s)",
				conf.General.TLS.PrivateKey, err)
		}
		var serverRootCAs, clientRootCAs [][]byte
		for _, serverRoot := range conf.General.TLS.RootCAs {
			root, err := ioutil.ReadFile(serverRoot)
			if err != nil {
				logger.Fatalf("Failed to load ServerRootCAs file '%s' (%s)",
					err, serverRoot)
			}
			serverRootCAs = append(serverRootCAs, root)
		}
		if secureConfig.RequireClientCert {
			for _, clientRoot := range conf.General.TLS.ClientRootCAs {
				root, err := ioutil.ReadFile(clientRoot)
				if err != nil {
					logger.Fatalf("Failed to load ClientRootCAs file '%s' (%s)",
						err, clientRoot)
				}
				clientRootCAs = append(clientRootCAs, root)
			}
		}
		secureConfig.ServerKey = serverKey
		secureConfig.ServerCertificate = serverCertificate
		secureConfig.ServerRootCAs = serverRootCAs
		secureConfig.ClientRootCAs = clientRootCAs
	}
	return secureConfig
}

func InitializeGrpcServer(conf *config.TopLevel) comm.GRPCServer {
	secureConfig := initializeSecureServerConfig(conf)

	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", conf.General.ListenAddress, conf.General.ListenPort))
	if err != nil {
		logger.Fatal("Failed to listen:", err)
	}

	// Create GRPC server - return if an error occurs
	grpcServer, err := comm.NewGRPCServerFromListener(lis, secureConfig)
	if err != nil {
		logger.Fatal("Failed to return new GRPC server:", err)
	}

	return grpcServer
}
