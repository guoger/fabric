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
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/orderer/localconfig"
	"github.com/stretchr/testify/assert"
)

func TestInitializeLoggingLevel(t *testing.T) {
	initializeLoggingLevel(
		&config.TopLevel{
			General: config.General{LogLevel: "debug"},
			Kafka:   config.Kafka{Verbose: true},
		},
	)
	assert.Equal(t, flogging.GetModuleLevel("orderer/main"), "DEBUG")
	assert.NotNil(t, sarama.Logger)
}

func TestInitializeProfilingService(t *testing.T) {
	// get a free random port
	listenAddr := func() string {
		l, _ := net.Listen("tcp", "localhost:0")
		l.Close()
		return l.Addr().String()
	}()
	initializeProfilingService(
		&config.TopLevel{
			General: config.General{
				LogLevel: "debug",
				Profile: config.Profile{
					Enabled: true,
					Address: listenAddr,
				}},
			Kafka: config.Kafka{Verbose: true},
		},
	)
	time.Sleep(500 * time.Millisecond)
	if _, err := http.Get("http://" + listenAddr + "/" + "/debug/"); err != nil {
		t.Logf("Expected pprof to be up (will retry again in 3 seconds): %s", err)
		time.Sleep(3 * time.Second)
		if _, err := http.Get("http://" + listenAddr + "/" + "/debug/"); err != nil {
			t.Fatalf("Expected pprof to be up: %s", err)
		}
	}
}
