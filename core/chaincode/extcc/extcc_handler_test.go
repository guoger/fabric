/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package extcc_test

import (
	"net"
	"time"

	"github.com/hyperledger/fabric/core/chaincode/extcc"
	"github.com/hyperledger/fabric/core/chaincode/mock"
	"github.com/hyperledger/fabric/core/comm"
	"github.com/hyperledger/fabric/core/container/ccintf"

	"google.golang.org/grpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Extcc", func() {
	var (
		i        *extcc.ExternalChaincodeRuntime
		ccinfo   *ccintf.ChaincodeServerInfo
		shandler *mock.ChaincodeStreamHandler
	)

	BeforeEach(func() {
		shandler = &mock.ChaincodeStreamHandler{}
		i = &extcc.ExternalChaincodeRuntime{}
		ccinfo = &ccintf.ChaincodeServerInfo{
			Address: "ccaddress:12345",
			ClientConfig: comm.ClientConfig{
				SecOpts: comm.SecureOptions{
					UseTLS:            true,
					RequireClientCert: true,
					Certificate:       []byte("fake-cert"),
					Key:               []byte("fake-key"),
					ServerRootCAs:     [][]byte{[]byte("fake-root-cert")},
				},
				Timeout: 10 * time.Second,
			},
		}
	})

	Context("Start", func() {
		When("chaincode is running", func() {
			var (
				cclist net.Listener
				ccserv *grpc.Server
			)
			BeforeEach(func() {
				var err error
				cclist, err = net.Listen("tcp", "127.0.0.1:0")
				Expect(err).To(BeNil())
				Expect(cclist).To(Not(BeNil()))
				ccserv = grpc.NewServer([]grpc.ServerOption{}...)
				go ccserv.Serve(cclist)
			})

			AfterEach(func() {
				if ccserv != nil {
					ccserv.Stop()
				}
				if cclist != nil {
					cclist.Close()
				}
			})

			It("runs to completion", func() {
				ccinfo = &ccintf.ChaincodeServerInfo{
					Address: cclist.Addr().String(),
					ClientConfig: comm.ClientConfig{
						KaOpts:  comm.DefaultKeepaliveOptions,
						Timeout: 10 * time.Second,
					},
				}
				err := i.Run("ccid", ccinfo, shandler)
				Expect(err).To(BeNil())
			})
		})
		When("chaincode info is nil", func() {
			It("returns an error", func() {
				ccinfo = nil
				err := i.Run("ccid", ccinfo, shandler)
				//Expect(err).To(MatchError(ContainSubstring("cannot start connector without connection properties")))
				Expect(err).To(MatchError(ContainSubstring("cannot start connector without connection properties")))
			})
		})
		Context("chaincode info incorrect", func() {
			When("address is bad", func() {
				It("returns an error", func() {
					ccinfo.ClientConfig.SecOpts.UseTLS = false
					ccinfo.Address = "<badaddress>"
					err := i.Run("ccid", ccinfo, shandler)
					Expect(err).To(MatchError(ContainSubstring("error creating grpc connection to <badaddress>")))
				})
			})
			When("unspecified client spec", func() {
				It("returns an error", func() {
					ccinfo.ClientConfig.SecOpts.Key = nil
					err := i.Run("ccid", ccinfo, shandler)
					Expect(err).To(MatchError(ContainSubstring("both Key and Certificate are required when using mutual TLS")))
				})
			})
		})
	})
})
