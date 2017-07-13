/*
Copyright IBM Corp. 2016 All Rights Reserved.

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
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/orderer/localconfig"
	"github.com/hyperledger/fabric/orderer/metadata"
	"github.com/hyperledger/fabric/orderer/utils"
	ab "github.com/hyperledger/fabric/protos/orderer"

	"github.com/Shopify/sarama"
	"github.com/hyperledger/fabric/common/localmsp"
	logging "github.com/op/go-logging"
	"gopkg.in/alecthomas/kingpin.v2"
)

var logger = logging.MustGetLogger("orderer/main")

//command line flags
var (
	app = kingpin.New("orderer", "Hyperledger Fabric orderer node")

	start   = app.Command("start", "Start the orderer node").Default()
	version = app.Command("version", "Show version information")
)

func main() {

	kingpin.Version("0.0.1")
	switch kingpin.MustParse(app.Parse(os.Args[1:])) {

	// "start" command
	case start.FullCommand():
		logger.Infof("Starting %s", metadata.GetVersionInfo())
		conf := config.Load()
		initializeLoggingLevel(conf)
		initializeProfilingService(conf)
		grpcServer := utils.InitializeGrpcServer(conf)
		utils.InitializeLocalMsp(conf)
		signer := localmsp.NewSigner()
		manager := utils.InitializeMultiChainManager(conf, signer)
		server := utils.NewServer(manager, signer)
		ab.RegisterAtomicBroadcastServer(grpcServer.Server(), server)
		logger.Info("Beginning to serve requests")
		grpcServer.Start()
	// "version" command
	case version.FullCommand():
		fmt.Println(metadata.GetVersionInfo())
	}

}

// Set the logging level
func initializeLoggingLevel(conf *config.TopLevel) {
	flogging.InitFromSpec(conf.General.LogLevel)
	if conf.Kafka.Verbose {
		sarama.Logger = log.New(os.Stdout, "[sarama] ", log.Ldate|log.Lmicroseconds|log.Lshortfile)
	}
}

// Start the profiling service if enabled.
func initializeProfilingService(conf *config.TopLevel) {
	if conf.General.Profile.Enabled {
		go func() {
			logger.Info("Starting Go pprof profiling service on:", conf.General.Profile.Address)
			// The ListenAndServe() call does not return unless an error occurs.
			logger.Panic("Go pprof service failed:", http.ListenAndServe(conf.General.Profile.Address, nil))
		}()
	}
}
