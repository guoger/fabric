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
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/hyperledger/fabric/bccsp/factory"
	"github.com/hyperledger/fabric/common/localmsp"
	coreconfig "github.com/hyperledger/fabric/core/config"
	config "github.com/hyperledger/fabric/orderer/localconfig"
	logging "github.com/op/go-logging"
	"github.com/stretchr/testify/assert"
)

const (
	goodFile = "util.go"
	badFile  = "does_not_exist"
)

func TestCreateLedgerFactory(t *testing.T) {
	testCases := []struct {
		name            string
		ledgerType      string
		ledgerDir       string
		ledgerDirPrefix string
		expectPanic     bool
	}{
		{"RAM", "ram", "", "", false},
		{"JSONwithPathSet", "json", "test-dir", "", false},
		{"JSONwithPathUnset", "json", "", "test-prefix", false},
		{"FilewithPathSet", "file", "test-dir", "", false},
		{"FilewithPathUnset", "file", "", "test-prefix", false},
	}

	conf := config.Load()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tc.expectPanic && r == nil {
					t.Fatal("Should have panicked")
				}
				if !tc.expectPanic && r != nil {
					t.Fatal("Should not have panicked")
				}
			}()

			conf.General.LedgerType = tc.ledgerType
			conf.FileLedger.Location = tc.ledgerDir
			conf.FileLedger.Prefix = tc.ledgerDirPrefix
			lf, ld := CreateLedgerFactory(conf)

			defer func() {
				if ld != "" {
					os.RemoveAll(ld)
					t.Log("Removed temp dir:", ld)
				}
			}()
			lf.ChainIDs()
		})
	}
}

func TestCreateSubDir(t *testing.T) {
	testCases := []struct {
		name          string
		count         int
		expectCreated bool
		expectPanic   bool
	}{
		{"CleanDir", 1, true, false},
		{"HasSubDir", 2, false, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tc.expectPanic && r == nil {
					t.Fatal("Should have panicked")
				}
				if !tc.expectPanic && r != nil {
					t.Fatal("Should not have panicked")
				}
			}()

			parentDirPath := createTempDir("test-dir")

			var created bool
			for i := 0; i < tc.count; i++ {
				_, created = createSubDir(parentDirPath, "test-sub-dir")
			}

			if created != tc.expectCreated {
				t.Fatalf("Sub dir created = %v, but expectation was = %v", created, tc.expectCreated)
			}
		})
	}
	t.Run("ParentDirNotExists", func(t *testing.T) {
		assert.Panics(t, func() { createSubDir(os.TempDir(), "foo/name") })
	})
}

func TestCreateTempDir(t *testing.T) {
	t.Run("Good", func(t *testing.T) {
		tempDir := createTempDir("foo")
		if _, err := os.Stat(tempDir); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Bad", func(t *testing.T) {
		assert.Panics(t, func() {
			createTempDir("foo/bar")
		})
	})
}

func TestInitializeBootstrapChannel(t *testing.T) {
	testCases := []struct {
		genesisMethod string
		ledgerType    string
		panics        bool
	}{
		{"provisional", "ram", false},
		{"provisional", "file", false},
		{"provisional", "json", false},
		{"invalid", "ram", true},
		{"file", "ram", true},
	}

	for _, tc := range testCases {

		t.Run(tc.genesisMethod+"/"+tc.ledgerType, func(t *testing.T) {

			fileLedgerLocation, _ := ioutil.TempDir("", "test-ledger")
			ledgerFactory, _ := CreateLedgerFactory(
				&config.TopLevel{
					General: config.General{LedgerType: tc.ledgerType},
					FileLedger: config.FileLedger{
						Location: fileLedgerLocation,
					},
				},
			)

			bootstrapConfig := &config.TopLevel{
				General: config.General{
					GenesisMethod:  tc.genesisMethod,
					GenesisProfile: "SampleSingleMSPSolo",
					GenesisFile:    "genesisblock",
				},
			}

			if tc.panics {
				assert.Panics(t, func() {
					initializeBootstrapChannel(bootstrapConfig, ledgerFactory)
				})
			} else {
				initializeBootstrapChannel(bootstrapConfig, ledgerFactory)
			}

		})
	}
}

func TestInitializeLocalMsp(t *testing.T) {
	t.Run("Happy", func(t *testing.T) {
		assert.NotPanics(t, func() {
			localMSPDir, _ := coreconfig.GetDevMspDir()
			InitializeLocalMsp(
				&config.TopLevel{
					General: config.General{
						LocalMSPDir: localMSPDir,
						LocalMSPID:  "DEFAULT",
						BCCSP: &factory.FactoryOpts{
							ProviderName: "SW",
							SwOpts: &factory.SwOpts{
								HashFamily: "SHA2",
								SecLevel:   256,
								Ephemeral:  true,
							},
						},
					},
				})
		})
	})
	t.Run("Error", func(t *testing.T) {
		logger.SetBackend(logging.AddModuleLevel(newPanicOnCriticalBackend()))
		defer func() {
			logger = logging.MustGetLogger("orderer/main")
		}()
		assert.Panics(t, func() {
			InitializeLocalMsp(
				&config.TopLevel{
					General: config.General{
						LocalMSPDir: "",
						LocalMSPID:  "",
					},
				})
		})
	})
}

func TestInitializeSecureServerConfig(t *testing.T) {
	initializeSecureServerConfig(
		&config.TopLevel{
			General: config.General{
				TLS: config.TLS{
					Enabled:           true,
					ClientAuthEnabled: true,
					Certificate:       goodFile,
					PrivateKey:        goodFile,
					RootCAs:           []string{goodFile},
					ClientRootCAs:     []string{goodFile},
				},
			},
		})

	logger.SetBackend(logging.AddModuleLevel(newPanicOnCriticalBackend()))
	defer func() {
		logger = logging.MustGetLogger("orderer/main")
	}()

	testCases := []struct {
		name              string
		certificate       string
		privateKey        string
		rootCA            string
		clientCertificate string
	}{
		{"BadCertificate", badFile, goodFile, goodFile, goodFile},
		{"BadPrivateKey", goodFile, badFile, goodFile, goodFile},
		{"BadRootCA", goodFile, goodFile, badFile, goodFile},
		{"BadClientCertificate", goodFile, goodFile, goodFile, badFile},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Panics(t, func() {
				initializeSecureServerConfig(
					&config.TopLevel{
						General: config.General{
							TLS: config.TLS{
								Enabled:           true,
								ClientAuthEnabled: true,
								Certificate:       tc.certificate,
								PrivateKey:        tc.privateKey,
								RootCAs:           []string{tc.rootCA},
								ClientRootCAs:     []string{tc.clientCertificate},
							},
						},
					})
			},
			)
		})
	}
}

func TestInitializeMultiChainManager(t *testing.T) {
	localMSPDir, _ := coreconfig.GetDevMspDir()
	conf := &config.TopLevel{
		General: config.General{
			LedgerType:     "ram",
			GenesisMethod:  "provisional",
			GenesisProfile: "SampleSingleMSPSolo",
			LocalMSPDir:    localMSPDir,
			LocalMSPID:     "DEFAULT",
			BCCSP: &factory.FactoryOpts{
				ProviderName: "SW",
				SwOpts: &factory.SwOpts{
					HashFamily: "SHA2",
					SecLevel:   256,
					Ephemeral:  true,
				},
			},
		},
	}
	assert.NotPanics(t, func() {
		InitializeLocalMsp(conf)
		InitializeMultiChainManager(conf, localmsp.NewSigner())
	})
}

func TestInitializeGrpcServer(t *testing.T) {
	// get a free random port
	listenAddr := func() string {
		l, _ := net.Listen("tcp", "localhost:0")
		l.Close()
		return l.Addr().String()
	}()
	host := strings.Split(listenAddr, ":")[0]
	port, _ := strconv.ParseUint(strings.Split(listenAddr, ":")[1], 10, 16)
	assert.NotPanics(t, func() {
		grpcServer := InitializeGrpcServer(
			&config.TopLevel{
				General: config.General{
					ListenAddress: host,
					ListenPort:    uint16(port),
					TLS: config.TLS{
						Enabled:           false,
						ClientAuthEnabled: false,
					},
				},
			})
		grpcServer.Listener().Close()
	})
}

func newPanicOnCriticalBackend() *panicOnCriticalBackend {
	return &panicOnCriticalBackend{
		backend: logging.AddModuleLevel(logging.NewLogBackend(os.Stderr, "", log.LstdFlags)),
	}
}

type panicOnCriticalBackend struct {
	backend logging.Backend
}

func (b *panicOnCriticalBackend) Log(level logging.Level, calldepth int, record *logging.Record) error {
	err := b.backend.Log(level, calldepth, record)
	if level == logging.CRITICAL {
		panic(record.Formatted(calldepth))
	}
	return err
}
