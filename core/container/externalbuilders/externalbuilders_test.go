/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package externalbuilders_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/core/chaincode/persistence"
	"github.com/hyperledger/fabric/core/comm"
	"github.com/hyperledger/fabric/core/container/ccintf"
	"github.com/hyperledger/fabric/core/container/externalbuilders"
	"github.com/hyperledger/fabric/core/peer"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var _ = Describe("Externalbuilders", func() {
	var (
		codePackage *os.File
		md          *persistence.ChaincodePackageMetadata
	)

	BeforeEach(func() {
		var err error
		codePackage, err = os.Open("testdata/normal_archive.tar.gz")
		Expect(err).NotTo(HaveOccurred())

		md = &persistence.ChaincodePackageMetadata{
			Path: "fake-path",
			Type: "fake-type",
		}
	})

	AfterEach(func() {
		if codePackage != nil {
			codePackage.Close()
		}
	})

	Describe("NewBuildContext()", func() {
		It("creates a new context, including temporary locations", func() {
			buildContext, err := externalbuilders.NewBuildContext("fake-package-id", md, codePackage)
			Expect(err).NotTo(HaveOccurred())
			defer buildContext.Cleanup()

			Expect(buildContext.ScratchDir).NotTo(BeEmpty())
			Expect(buildContext.ScratchDir).To(BeADirectory())

			Expect(buildContext.SourceDir).NotTo(BeEmpty())
			Expect(buildContext.SourceDir).To(BeADirectory())

			Expect(buildContext.ReleaseDir).NotTo(BeEmpty())
			Expect(buildContext.ReleaseDir).To(BeADirectory())

			Expect(buildContext.BldDir).NotTo(BeEmpty())
			Expect(buildContext.BldDir).To(BeADirectory())

			Expect(filepath.Join(buildContext.SourceDir, "a/test.file")).To(BeARegularFile())
		})

		Context("when the archive cannot be extracted", func() {
			It("returns an error", func() {
				codePackage, err := os.Open("testdata/archive_with_absolute.tar.gz")
				Expect(err).NotTo(HaveOccurred())
				defer codePackage.Close()
				_, err = externalbuilders.NewBuildContext("fake-package-id", md, codePackage)
				Expect(err).To(MatchError(ContainSubstring("could not untar source package")))
			})
		})

		Context("when package id contains inappropriate chars", func() {
			It("replaces them with dash", func() {
				buildContext, err := externalbuilders.NewBuildContext("i&am/pkg:id", md, codePackage)
				Expect(err).NotTo(HaveOccurred())
				Expect(buildContext.ScratchDir).To(ContainSubstring("fabric-i-am-pkg-id"))
			})
		})
	})

	Describe("Detector", func() {
		var (
			durablePath string
			detector    *externalbuilders.Detector
		)

		BeforeEach(func() {
			var err error
			durablePath, err = ioutil.TempDir("", "detect-test")
			Expect(err).NotTo(HaveOccurred())

			detector = &externalbuilders.Detector{
				Builders: externalbuilders.CreateBuilders([]peer.ExternalBuilder{
					{
						Path: "bad1",
						Name: "bad1",
					},
					{
						Path: "testdata/goodbuilder",
						Name: "goodbuilder",
					},
					{
						Path: "bad2",
						Name: "bad2",
					},
				}),
				DurablePath: durablePath,
			}
		})

		AfterEach(func() {
			if durablePath != "" {
				err := os.RemoveAll(durablePath)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		Describe("Build", func() {
			It("iterates over all detectors and chooses the one that matches", func() {
				instance, err := detector.Build("fake-package-id", md, codePackage)
				Expect(err).NotTo(HaveOccurred())
				Expect(instance.Builder.Name).To(Equal("goodbuilder"))
			})

			Context("when no builder can be found", func() {
				BeforeEach(func() {
					detector.Builders = nil
				})

				It("returns a nil instance", func() {
					i, err := detector.Build("fake-package-id", md, codePackage)
					Expect(err).NotTo(HaveOccurred())
					Expect(i).To(BeNil())
				})
			})

			It("persists the build output", func() {
				_, err := detector.Build("fake-package-id", md, codePackage)
				Expect(err).NotTo(HaveOccurred())

				Expect(filepath.Join(durablePath, "fake-package-id", "bld")).To(BeADirectory())
				Expect(filepath.Join(durablePath, "fake-package-id", "release")).To(BeADirectory())
				Expect(filepath.Join(durablePath, "fake-package-id", "build-info.json")).To(BeARegularFile())
			})

			Context("when the durable path cannot be created", func() {
				BeforeEach(func() {
					detector.DurablePath = "fake/path/to/nowhere"
				})

				It("wraps and returns the error", func() {
					_, err := detector.Build("fake-package-id", md, codePackage)
					Expect(err).To(MatchError("could not create dir 'fake/path/to/nowhere/fake-package-id' to persist build ouput: mkdir fake/path/to/nowhere/fake-package-id: no such file or directory"))
				})
			})
		})

		Describe("CachedBuild", func() {
			var (
				existingInstance *externalbuilders.Instance
			)

			BeforeEach(func() {
				var err error
				existingInstance, err = detector.Build("fake-package-id", md, codePackage)
				Expect(err).NotTo(HaveOccurred())

				// ensure the builder will fail if invoked
				detector.Builders[0].Location = "bad-path"
			})

			It("returns the existing built instance", func() {
				newInstance, err := detector.Build("fake-package-id", md, codePackage)
				Expect(err).NotTo(HaveOccurred())
				Expect(existingInstance).To(Equal(newInstance))
			})

			When("the build-info is missing", func() {
				BeforeEach(func() {
					err := os.RemoveAll(filepath.Join(durablePath, "fake-package-id", "build-info.json"))
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns an error", func() {
					_, err := detector.Build("fake-package-id", md, codePackage)
					Expect(err).To(MatchError(ContainSubstring("existing build could not be restored: could not read '")))
				})
			})

			When("the build-info is corrupted", func() {
				BeforeEach(func() {
					err := ioutil.WriteFile(filepath.Join(durablePath, "fake-package-id", "build-info.json"), []byte("{corrupted"), 0600)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns an error", func() {
					_, err := detector.Build("fake-package-id", md, codePackage)
					Expect(err).To(MatchError(ContainSubstring("invalid character 'c' looking for beginning of object key string")))
				})
			})

			When("the builder is no longer available", func() {
				BeforeEach(func() {
					detector.Builders = detector.Builders[:1]
				})

				It("returns an error", func() {
					_, err := detector.Build("fake-package-id", md, codePackage)
					Expect(err).To(MatchError("existing build could not be restored: chaincode 'fake-package-id' was already built with builder 'goodbuilder', but that builder is no longer available"))
				})
			})
		})
	})

	Describe("Builders", func() {
		var (
			builder      *externalbuilders.Builder
			buildContext *externalbuilders.BuildContext
		)

		BeforeEach(func() {
			builder = &externalbuilders.Builder{
				Location: "testdata/goodbuilder",
				Name:     "goodbuilder",
				Logger:   flogging.MustGetLogger("builder.test"),
			}

			var err error
			buildContext, err = externalbuilders.NewBuildContext("fake-package-id", md, codePackage)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			buildContext.Cleanup()
		})

		Describe("Detect", func() {
			It("detects when the package is handled by the external builder", func() {
				result := builder.Detect(buildContext)
				Expect(result).To(BeTrue())
			})

			Context("when the detector exits with a non-zero status", func() {
				BeforeEach(func() {
					builder.Location = "testdata/failbuilder"
				})

				It("returns false", func() {
					result := builder.Detect(buildContext)
					Expect(result).To(BeFalse())
				})
			})
		})

		Describe("Build", func() {
			It("builds the package by invoking external builder", func() {
				err := builder.Build(buildContext)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the builder exits with a non-zero status", func() {
				BeforeEach(func() {
					builder.Location = "testdata/failbuilder"
					builder.Name = "failbuilder"
				})

				It("returns an error", func() {
					err := builder.Build(buildContext)
					Expect(err).To(MatchError("external builder 'failbuilder' failed: exit status 1"))
				})
			})
		})

		Describe("Release", func() {
			It("releases the package by invoking external builder", func() {
				err := builder.Release(buildContext)
				Expect(err).NotTo(HaveOccurred())
			})

			When("the release binary is not in the builder", func() {
				BeforeEach(func() {
					builder.Location = "bad-builder-location"
				})

				It("returns no error as release is optional", func() {
					err := builder.Release(buildContext)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			When("the builder exits with a non-zero status", func() {
				BeforeEach(func() {
					builder.Location = "testdata/failbuilder"
					builder.Name = "failbuilder"
				})

				It("returns an error", func() {
					err := builder.Release(buildContext)
					Expect(err).To(MatchError("builder 'failbuilder' release failed: exit status 1"))
				})
			})
		})

		Describe("Run", func() {
			var (
				fakeConnection *ccintf.PeerConnection
				bldDir         string
			)

			BeforeEach(func() {
				var err error
				bldDir, err = ioutil.TempDir("", "run-test")
				Expect(err).NotTo(HaveOccurred())

				fakeConnection = &ccintf.PeerConnection{
					Address: "fake-peer-address",
					TLSConfig: &ccintf.TLSConfig{
						ClientCert: []byte("fake-client-cert"),
						ClientKey:  []byte("fake-client-key"),
						RootCert:   []byte("fake-root-cert"),
					},
				}
			})

			AfterEach(func() {
				if bldDir != "" {
					err := os.RemoveAll(bldDir)
					Expect(err).NotTo(HaveOccurred())
				}
			})

			It("runs the package by invoking external builder", func() {
				rs, err := builder.Run("test-ccid", bldDir, fakeConnection)
				Expect(err).NotTo(HaveOccurred())
				Eventually(rs.Done()).Should(BeClosed())
				Expect(rs.Err()).NotTo(HaveOccurred())
			})

			Context("when the run exits with a non-zero status", func() {
				BeforeEach(func() {
					builder.Location = "testdata/failbuilder"
					builder.Name = "failbuilder"
				})

				It("returns an error", func() {
					rs, err := builder.Run("test-ccid", bldDir, fakeConnection)
					Expect(err).NotTo(HaveOccurred())
					Eventually(rs.Done()).Should(BeClosed())
					Expect(rs.Err()).To(MatchError("builder 'failbuilder' run failed: exit status 1"))
				})
			})
		})
	})

	Describe("NewCommand", func() {
		It("only propagates expected variables", func() {
			var expectedEnv []string
			for _, key := range externalbuilders.DefaultEnvWhitelist {
				if val, ok := os.LookupEnv(key); ok {
					expectedEnv = append(expectedEnv, fmt.Sprintf("%s=%s", key, val))
				}
			}

			cmd := externalbuilders.NewCommand("/usr/bin/env", externalbuilders.DefaultEnvWhitelist)
			Expect(cmd.Env).To(ConsistOf(expectedEnv))

			output, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			env := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
			Expect(env).To(ConsistOf(expectedEnv))
		})
	})

	Describe("RunCommand", func() {
		var (
			logger *flogging.FabricLogger
			buf    *bytes.Buffer
		)

		BeforeEach(func() {
			buf = &bytes.Buffer{}
			enc := zapcore.NewConsoleEncoder(zapcore.EncoderConfig{MessageKey: "msg"})
			core := zapcore.NewCore(enc, zapcore.AddSync(buf), zap.NewAtomicLevel())
			logger = flogging.NewFabricLogger(zap.New(core).Named("logger"))
		})

		It("runs the command, directs stderr to the logger, and includes the command name", func() {
			cmd := exec.Command("/bin/sh", "-c", `echo stdout && echo stderr >&2`)
			err := externalbuilders.RunCommand(logger, cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(buf.String()).To(Equal("stderr\t" + `{"command": "sh"}` + "\n"))
		})

		Context("when start fails", func() {
			It("returns the error", func() {
				cmd := exec.Command("nonsense-program")
				err := externalbuilders.RunCommand(logger, cmd)
				Expect(err).To(HaveOccurred())

				execError, ok := err.(*exec.Error)
				Expect(ok).To(BeTrue())
				Expect(execError.Name).To(Equal("nonsense-program"))
			})
		})

		Context("when the process exits with a non-zero return", func() {
			It("returns the exec.ExitErr for the command", func() {
				cmd := exec.Command("false")
				err := externalbuilders.RunCommand(logger, cmd)
				Expect(err).To(HaveOccurred())

				exitErr, ok := err.(*exec.ExitError)
				Expect(ok).To(BeTrue())
				Expect(exitErr.ExitCode()).To(Equal(1))
			})
		})
	})

	Describe("RunStatus", func() {
		var (
			rs *externalbuilders.RunStatus
		)

		BeforeEach(func() {
			rs = externalbuilders.NewRunStatus()
		})

		It("has a blocking ready channel", func() {
			Consistently(rs.Done()).ShouldNot(BeClosed())
		})

		When("notify is called with an error", func() {
			BeforeEach(func() {
				rs.Notify(fmt.Errorf("fake-status-error"))
			})

			It("closes the blocking ready channel", func() {
				Expect(rs.Done()).To(BeClosed())
			})

			It("sets the error value", func() {
				Expect(rs.Err()).To(MatchError("fake-status-error"))
			})
		})
	})

	Describe("Instance", func() {
		var (
			i *externalbuilders.Instance
		)

		BeforeEach(func() {
			i = &externalbuilders.Instance{
				PackageID: "test-ccid",
				Builder: &externalbuilders.Builder{
					Location: "testdata/goodbuilder",
					Logger:   flogging.MustGetLogger("builder.test"),
				},
			}
		})

		Describe("ChaincodeServerInfo", func() {
			BeforeEach(func() {
				var err error
				i.ReleaseDir, err = ioutil.TempDir("", "cc-conn-test")
				Expect(err).NotTo(HaveOccurred())

				err = os.MkdirAll(filepath.Join(i.ReleaseDir, "chaincode", "server"), 0755)
				Expect(err).NotTo(HaveOccurred())
				//initiaze with a well-formed, all fields set, connection.json file
				ccdata := `{"address": "ccaddress:12345", "tls_required": true, "dial_timeout": "10s", "client_auth_required": true, "key_path": "key.pem", "cert_path": "cert.pem", "root_cert_path": "root.pem"}`
				err = ioutil.WriteFile(filepath.Join(i.ChaincodeServerReleaseDir(), "connection.json"), []byte(ccdata), 0600)
				Expect(err).NotTo(HaveOccurred())
				err = ioutil.WriteFile(filepath.Join(i.ChaincodeServerReleaseDir(), "key.pem"), []byte("fake-key"), 0600)
				Expect(err).NotTo(HaveOccurred())
				err = ioutil.WriteFile(filepath.Join(i.ChaincodeServerReleaseDir(), "cert.pem"), []byte("fake-cert"), 0600)
				Expect(err).NotTo(HaveOccurred())
				err = ioutil.WriteFile(filepath.Join(i.ChaincodeServerReleaseDir(), "root.pem"), []byte("fake-root-cert"), 0600)
				Expect(err).NotTo(HaveOccurred())
			})
			AfterEach(func() {
				os.RemoveAll(i.ReleaseDir)
			})
			It("returns chaincode connection", func() {
				ccinfo, err := i.ChaincodeServerInfo()
				Expect(err).NotTo(HaveOccurred())
				Expect(ccinfo).To(Equal(&ccintf.ChaincodeServerInfo{
					Address: "ccaddress:12345",
					ClientConfig: comm.ClientConfig{
						SecOpts: comm.SecureOptions{
							UseTLS:            true,
							RequireClientCert: true,
							Certificate:       []byte("fake-cert"),
							Key:               []byte("fake-key"),
							ServerRootCAs:     [][]byte{[]byte("fake-root-cert")},
						},
						KaOpts:  comm.DefaultKeepaliveOptions,
						Timeout: 10 * time.Second,
					},
				}))
			})
			When("connection.json is not provided", func() {
				BeforeEach(func() {
					err := os.Remove(filepath.Join(i.ChaincodeServerReleaseDir(), "connection.json"))
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns nil server info", func() {
					ccinfo, err := i.ChaincodeServerInfo()
					Expect(err).NotTo(HaveOccurred())
					Expect(ccinfo).To(BeNil())
				})
			})
			When("chaincode info is badly formed", func() {
				BeforeEach(func() {
					ccdata := `{"badly formed chaincode"}`
					err := ioutil.WriteFile(filepath.Join(i.ChaincodeServerReleaseDir(), "connection.json"), []byte(ccdata), 0600)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns a malformed chaincode error", func() {
					_, err := i.ChaincodeServerInfo()
					Expect(err).To(MatchError(ContainSubstring("malformed chaincode info")))
				})
			})
		})
		Describe("Start", func() {
			It("invokes the builder's run command and sets the run status", func() {
				err := i.Start(&ccintf.PeerConnection{
					Address: "fake-peer-address",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(i.RunStatus).NotTo(BeNil())
				Eventually(i.RunStatus.Done()).Should(BeClosed())
			})
		})

		Describe("Stop", func() {
			It("statically returns an error", func() {
				err := i.Stop()
				Expect(err).To(MatchError("stop is not implemented for external builders yet"))
			})
		})

		Describe("Wait", func() {
			BeforeEach(func() {
				err := i.Start(&ccintf.PeerConnection{
					Address: "fake-peer-address",
					TLSConfig: &ccintf.TLSConfig{
						ClientCert: []byte("fake-client-cert"),
						ClientKey:  []byte("fake-client-key"),
						RootCert:   []byte("fake-root-cert"),
					},
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the exit status of the run", func() {
				code, err := i.Wait()
				Expect(err).NotTo(HaveOccurred())
				Expect(code).To(Equal(0))
			})

			When("run exits with a non-zero status", func() {
				BeforeEach(func() {
					i.Builder.Location = "testdata/failbuilder"
					i.Builder.Name = "failbuilder"
					err := i.Start(&ccintf.PeerConnection{
						Address: "fake-peer-address",
					})
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the exit status of the run and accompanying error", func() {
					code, err := i.Wait()
					Expect(err).To(MatchError("builder 'failbuilder' run failed: exit status 1"))
					Expect(code).To(Equal(1))
				})
			})
		})
	})

	Describe("ChaincodeServerUserData", func() {
		var (
			ccuserdata *externalbuilders.ChaincodeServerUserData
			releaseDir string
		)

		BeforeEach(func() {
			var err error
			releaseDir, err = ioutil.TempDir("", "cc-conn-test")
			Expect(err).NotTo(HaveOccurred())

			err = os.MkdirAll(filepath.Join(releaseDir, "chaincode", "server"), 0755)
			Expect(err).NotTo(HaveOccurred())
			err = ioutil.WriteFile(filepath.Join(releaseDir, "key.pem"), []byte("fake-key"), 0600)
			Expect(err).NotTo(HaveOccurred())
			err = ioutil.WriteFile(filepath.Join(releaseDir, "cert.pem"), []byte("fake-cert"), 0600)
			Expect(err).NotTo(HaveOccurred())
			err = ioutil.WriteFile(filepath.Join(releaseDir, "root.pem"), []byte("fake-root-cert"), 0600)
			Expect(err).NotTo(HaveOccurred())

			ccuserdata = &externalbuilders.ChaincodeServerUserData{
				Address:            "ccaddress:12345",
				DialTimeout:        externalbuilders.Duration{10 * time.Second},
				TlsRequired:        true,
				ClientAuthRequired: true,
				KeyPath:            "key.pem",
				CertPath:           "cert.pem",
				RootCertPath:       "root.pem",
			}
		})
		AfterEach(func() {
			os.RemoveAll(releaseDir)
		})
		When("chaincode does not provide all info", func() {
			Context("tls is not provided", func() {
				It("returns TLS without client auth information", func() {
					//"tls" missing
					ccuserdata.TlsRequired = false

					ccinfo, err := ccuserdata.ChaincodeServerInfo(releaseDir)
					Expect(err).NotTo(HaveOccurred())
					Expect(ccinfo).To(Equal(&ccintf.ChaincodeServerInfo{
						Address: "ccaddress:12345",
						ClientConfig: comm.ClientConfig{
							Timeout: 10 * time.Second,
							KaOpts:  comm.DefaultKeepaliveOptions,
						},
					}))
				})
			})
			Context("client auth is not provided", func() {
				It("returns TLS without client auth information", func() {
					//"client_auth_required" missing
					ccuserdata.ClientAuthRequired = false

					ccinfo, err := ccuserdata.ChaincodeServerInfo(releaseDir)
					Expect(err).NotTo(HaveOccurred())
					Expect(ccinfo).To(Equal(&ccintf.ChaincodeServerInfo{
						Address: "ccaddress:12345",
						ClientConfig: comm.ClientConfig{
							SecOpts: comm.SecureOptions{
								UseTLS:        true,
								ServerRootCAs: [][]byte{[]byte("fake-root-cert")},
							},
							KaOpts:  comm.DefaultKeepaliveOptions,
							Timeout: 10 * time.Second,
						},
					}))
				})
			})
			Context("dial timeout not provided", func() {
				It("returns default dial timeout without dialtimeout", func() {
					//"dial_timeout" missing
					ccuserdata.DialTimeout = externalbuilders.Duration{}

					ccinfo, err := ccuserdata.ChaincodeServerInfo(releaseDir)
					Expect(err).NotTo(HaveOccurred())
					Expect(ccinfo).To(Equal(&ccintf.ChaincodeServerInfo{
						Address: "ccaddress:12345",
						ClientConfig: comm.ClientConfig{
							SecOpts: comm.SecureOptions{
								UseTLS:            true,
								RequireClientCert: true,
								Certificate:       []byte("fake-cert"),
								Key:               []byte("fake-key"),
								ServerRootCAs:     [][]byte{[]byte("fake-root-cert")},
							},
							KaOpts:  comm.DefaultKeepaliveOptions,
							Timeout: 3 * time.Second,
						},
					}))
				})
			})
			Context("address is not provided", func() {
				It("returns missing address error", func() {
					//"address" missing
					ccuserdata.Address = ""

					_, err := ccuserdata.ChaincodeServerInfo(releaseDir)
					Expect(err).To(MatchError("chaincode address not provided"))
				})
			})
			Context("key is not provided", func() {
				It("returns missing key error", func() {
					//"key" missing
					ccuserdata.KeyPath = ""

					_, err := ccuserdata.ChaincodeServerInfo(releaseDir)
					Expect(err).To(MatchError("chaincode tls key not provided"))
				})
			})
			Context("cert is not provided", func() {
				It("returns missing key error", func() {
					//"cert" missing
					ccuserdata.CertPath = ""

					_, err := ccuserdata.ChaincodeServerInfo(releaseDir)
					Expect(err).To(MatchError("chaincode tls cert not provided"))
				})
			})
			Context("root cert is not provided", func() {
				It("returns missing root cert error", func() {
					//"root" missing
					ccuserdata.RootCertPath = ""

					_, err := ccuserdata.ChaincodeServerInfo(releaseDir)
					Expect(err).To(MatchError("chaincode tls root cert not provided"))
				})
			})
			Context("cert file is missing", func() {
				It("returns missing cert file error", func() {
					//cert file is missing
					err := os.Remove(filepath.Join(releaseDir, "cert.pem"))
					Expect(err).NotTo(HaveOccurred())

					_, err = ccuserdata.ChaincodeServerInfo(releaseDir)
					Expect(err).To(MatchError(ContainSubstring("error reading cert file")))
				})
			})
			Context("key file is missing", func() {
				It("returns missing key file error", func() {
					//key file is missing
					err := os.Remove(filepath.Join(releaseDir, "key.pem"))
					Expect(err).NotTo(HaveOccurred())

					_, err = ccuserdata.ChaincodeServerInfo(releaseDir)
					Expect(err).To(MatchError(ContainSubstring("error reading key file")))
				})
			})
			Context("root cert file is missing", func() {
				It("returns missing root cert file error", func() {
					//key file is missing
					err := os.Remove(filepath.Join(releaseDir, "root.pem"))
					Expect(err).NotTo(HaveOccurred())

					_, err = ccuserdata.ChaincodeServerInfo(releaseDir)
					Expect(err).To(MatchError(ContainSubstring("error reading root cert file")))
				})
			})
		})
	})
})

var _ = Describe("SanitizeCCIDPath", func() {
	It("forbids the set of forbidden windows characters", func() {
		sanitizedPath := externalbuilders.SanitizeCCIDPath(`<>:"/\|?*&`)
		Expect(sanitizedPath).To(Equal("----------"))
	})
})
