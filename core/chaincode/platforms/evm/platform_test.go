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

package evm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"testing"

	pb "github.com/hyperledger/fabric/protos/peer"
	"github.com/stretchr/testify/assert"
)

var platform = &Platform{}

func TestGetDeploymentPayload(t *testing.T) {
	t.Run("FileNotExist", func(t *testing.T) {
		spec := &pb.ChaincodeSpec{
			Type:        pb.ChaincodeSpec_EVM,
			ChaincodeId: &pb.ChaincodeID{Path: "this/should/not/exist"},
			Input:       &pb.ChaincodeInput{Args: [][]byte{[]byte("invoke")}},
		}

		_, err := platform.GetDeploymentPayload(spec)
		assert.Error(t, err, "expect GetDeploymentPayload to fail")
	})

	t.Run("HexDecodingError", func(t *testing.T) {
		file, err := ioutil.TempFile("", "evm-chaincode-test")
		assert.NoError(t, err)
		defer os.Remove(file.Name())

		// Odd number of chars
		code := []byte("AAAAA")

		_, err = file.Write(code)
		assert.NoError(t, err)

		spec := &pb.ChaincodeSpec{
			Type:        pb.ChaincodeSpec_EVM,
			ChaincodeId: &pb.ChaincodeID{Path: file.Name()},
			Input:       &pb.ChaincodeInput{Args: [][]byte{[]byte("invoke")}},
		}
		_, err = platform.GetDeploymentPayload(spec)
		assert.Error(t, err)
	})

	t.Run("Good", func(t *testing.T) {
		file, err := ioutil.TempFile("", "evm-chaincode-test")
		assert.NoError(t, err)
		defer os.Remove(file.Name())

		ccName := "testcc"
		code := []byte("AAAAAA")
		_, err = file.Write(code)
		assert.NoError(t, err)

		spec := &pb.ChaincodeSpec{
			Type:        pb.ChaincodeSpec_EVM,
			ChaincodeId: &pb.ChaincodeID{Name: ccName, Path: file.Name()},
			Input:       &pb.ChaincodeInput{Args: [][]byte{[]byte("invoke")}},
		}
		payload, err := platform.GetDeploymentPayload(spec)
		assert.NoError(t, err)

		r := bytes.NewReader(payload)
		zr, err := gzip.NewReader(r)
		assert.NoError(t, err)

		tr := tar.NewReader(zr)

		// Read first entry
		hdr, err := tr.Next()
		assert.NoError(t, err)

		assert.Equal(t, ccName, hdr.Name, "expect header name to be %s", ccName)

		p := make([]byte, len(code))
		n, err := tr.Read(p)
		assert.NoError(t, err)

		assert.Equal(t, n, len(code), "expect to read %d bytes", n)
		assert.Equal(t, code, p, "expect to decode bytecode from source file")

		// Expect EOF
		_, err = tr.Next()
		assert.Equal(t, io.EOF, err, "expect to reach end of file")
	})
}

func TestValidateSpec(t *testing.T) {
	t.Run("FileNotExist", func(t *testing.T) {
		spec := &pb.ChaincodeSpec{
			Type:        pb.ChaincodeSpec_EVM,
			ChaincodeId: &pb.ChaincodeID{Path: "this/should/not/exist"},
			Input:       &pb.ChaincodeInput{Args: [][]byte{[]byte("invoke")}},
		}

		assert.Error(t, platform.ValidateSpec(spec), "expect spec to be invalid")
	})

	t.Run("IsDir", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "evm-chaincode-test")
		assert.NoError(t, err)
		defer os.RemoveAll(dir)

		spec := &pb.ChaincodeSpec{
			Type:        pb.ChaincodeSpec_EVM,
			ChaincodeId: &pb.ChaincodeID{Path: dir},
			Input:       &pb.ChaincodeInput{Args: [][]byte{[]byte("invoke")}},
		}

		assert.Error(t, platform.ValidateSpec(spec), "expect spec to be invalid")
	})

	t.Run("Valid", func(t *testing.T) {
		file, err := ioutil.TempFile("", "evm-chaincode-test")
		assert.NoError(t, err)
		defer os.Remove(file.Name())

		spec := &pb.ChaincodeSpec{
			Type:        pb.ChaincodeSpec_EVM,
			ChaincodeId: &pb.ChaincodeID{Path: file.Name()},
			Input:       &pb.ChaincodeInput{Args: [][]byte{[]byte("invoke")}},
		}

		assert.NoError(t, platform.ValidateSpec(spec), "expect spec to be valid")
	})
}
