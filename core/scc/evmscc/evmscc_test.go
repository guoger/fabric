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

package evmscc

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	cutil "github.com/hyperledger/fabric/core/container/util"
	pb "github.com/hyperledger/fabric/protos/peer"
	"github.com/stretchr/testify/assert"
)

/*
Example Solidity code
```
pragma solidity ^0.4.0;

contract SimpleStorage {
  uint storedData;

	function set(uint x) public {
	  storedData = x;
	}

	function get() public constant returns (uint) {
	  return storedData;
	}
}
	```
*/
// It is compiled into following bytecode for testing purpose:
const BYTECODE = "6060604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806360fe47b114604e5780636d4ce63c14606e575b600080fd5b3415605857600080fd5b606c60048080359060200190919050506094565b005b3415607857600080fd5b607e609e565b6040518082815260200191505060405180910390f35b8060008190555050565b600080549050905600a165627a7a72305820f9fb166de37451069ab363703b6fb1256de8a554f03878295945727a6015f2480029"

// Keccak hash of `set` function is:
const SET = "60fe47b1"

// Keccak hash of `get` function is:
const GET = "6d4ce63c"

func TestInit(t *testing.T) {
	ccName := "testcc"
	evmscc := new(EvmChaincode)
	stub := shim.NewMockStub("evmscc", evmscc)

	bytecode := []byte(BYTECODE)

	packageCode, err := packBytecode(ccName, bytecode)
	assert.NoError(t, err)

	cdsBytes, err := proto.Marshal(&pb.ChaincodeDeploymentSpec{CodePackage: packageCode})
	assert.NoError(t, err)

	res := stub.MockInit("txid", [][]byte{[]byte(ccName), cdsBytes})
	assert.Equal(t, int32(shim.OK), res.Status, "expect evmscc init to be OK but got: %s", res.Message)

	bytes, err := stub.GetState(ccName)
	assert.NoError(t, err)
	assert.Equal(t, BYTECODE, hex.EncodeToString(bytes), "expect raw bytecode to be put into ledger")
}

func TestInitError(t *testing.T) {
	t.Run("InsufficientArgs", func(t *testing.T) {
		evmscc := new(EvmChaincode)
		stub := shim.NewMockStub("evmscc", evmscc)
		res := stub.MockInit("txid", [][]byte{[]byte("testcc")})
		assert.Equal(t, int32(shim.ERROR), res.Status, "expect evmscc init to fail")
		assert.Equal(t, "expects 2 args, got 1", res.Message)
	})

	t.Run("InvalidCDS", func(t *testing.T) {
		evmscc := new(EvmChaincode)
		stub := shim.NewMockStub("evmscc", evmscc)
		res := stub.MockInit("txid", [][]byte{[]byte("testcc"), []byte("invalid spec")})
		assert.Equal(t, int32(shim.ERROR), res.Status, "expect evmscc init to fail")
		assert.Contains(t, res.Message, "failed to unmarshal ChaincodeDeploymentSpec")
	})

	t.Run("NilPackageCode", func(t *testing.T) {
		evmscc := new(EvmChaincode)
		stub := shim.NewMockStub("evmscc", evmscc)

		invalidCDS, err := proto.Marshal(&pb.ChaincodeDeploymentSpec{})
		assert.NoError(t, err)

		res := stub.MockInit("txid", [][]byte{[]byte("testcc"), invalidCDS})
		assert.Equal(t, int32(shim.ERROR), res.Status, "expect evmscc init to fail")
		assert.Contains(t, res.Message, "failed to find codepackage")
	})

	t.Run("InvalidCDSPackageCode", func(t *testing.T) {
		evmscc := new(EvmChaincode)
		stub := shim.NewMockStub("evmscc", evmscc)

		invalidCDS, err := proto.Marshal(&pb.ChaincodeDeploymentSpec{CodePackage: []byte("Invalid package code")})
		assert.NoError(t, err)

		res := stub.MockInit("txid", [][]byte{[]byte("testcc"), invalidCDS})
		assert.Equal(t, int32(shim.ERROR), res.Status, "expect evmscc init to fail")
		assert.Contains(t, res.Message, "failed to decode bytecode")
	})

	t.Run("PutStateError", func(t *testing.T) {
		ccName := "testcc"
		evmscc := new(EvmChaincode)
		stub := shim.NewMockStub("evmscc", evmscc)

		bytecode := []byte(BYTECODE)

		packageCode, err := packBytecode(ccName, bytecode)
		assert.NoError(t, err)

		cdsBytes, err := proto.Marshal(&pb.ChaincodeDeploymentSpec{CodePackage: packageCode})
		assert.NoError(t, err)

		res := stub.MockInit("", [][]byte{[]byte(ccName), cdsBytes})
		assert.Equal(t, int32(shim.ERROR), res.Status, "expect evmscc init to fail")
		assert.Contains(t, res.Message, "failed to put bytecode into ledger")
	})
}

// Invoke and query the example bytecode
func TestInvoke(t *testing.T) {
	ccName := "testcc"
	evmscc := new(EvmChaincode)
	stub := shim.NewMockStub("evmscc", evmscc)

	bytecode := []byte(BYTECODE)

	packageCode, err := packBytecode(ccName, bytecode)
	assert.NoError(t, err)

	cdsBytes, err := proto.Marshal(&pb.ChaincodeDeploymentSpec{CodePackage: packageCode})
	assert.NoError(t, err)

	res := stub.MockInit("txid", [][]byte{[]byte(ccName), cdsBytes})
	assert.Equal(t, int32(shim.OK), res.Status, "expect evmscc init to be OK but got: %s", res.Message)

	i, err := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000003")
	assert.NoError(t, err)

	// Invoke
	invokeArg := []byte(SET + hex.EncodeToString(i))
	ir := stub.MockInvoke("invoketxid", [][]byte{[]byte(ccName), invokeArg})
	assert.Equal(t, int32(shim.OK), ir.Status, "expect invoke to be OK")
	assert.Equal(t, []byte(nil), ir.Payload, "expect nil payload")

	// Query
	qr := stub.MockInvoke("querytxid", [][]byte{[]byte(ccName), []byte(GET)})
	assert.Equal(t, int32(shim.OK), qr.Status, "expect query to be OK")
	assert.Equal(t, i, qr.Payload, "expect query result to match invoke arg")
}

func TestInvokeError(t *testing.T) {
	t.Run("InsufficientArgs", func(t *testing.T) {
		evmscc := new(EvmChaincode)
		stub := shim.NewMockStub("evmscc", evmscc)
		res := stub.MockInvoke("txid", [][]byte{[]byte("testcc")})
		assert.Equal(t, int32(shim.ERROR), res.Status, "expect evmscc invoke to fail")
		assert.Equal(t, "expects 2 args, got 1", res.Message)
	})

	t.Run("InvalidArg", func(t *testing.T) {
		evmscc := new(EvmChaincode)
		stub := shim.NewMockStub("evmscc", evmscc)
		res := stub.MockInvoke("txid", [][]byte{[]byte("testcc"), []byte("0")})
		assert.Equal(t, int32(shim.ERROR), res.Status, "expect evmscc invoke to fail")
		assert.Equal(t, hex.ErrLength.Error(), res.Message)
	})

	t.Run("FailedToFetchCode", func(t *testing.T) {
		evmscc := new(EvmChaincode)
		stub := shim.NewMockStub("evmscc", evmscc)
		res := stub.MockInvoke("txid", [][]byte{[]byte("testcc"), []byte("00")})
		assert.Equal(t, int32(shim.ERROR), res.Status, "expect invoke to fail")
		assert.Contains(t, res.Message, "bytecode for chaincode 'testcc' is nil")
	})
}

func packBytecode(ccName string, bytecode []byte) ([]byte, error) {
	payload := bytes.NewBuffer(nil)
	gw := gzip.NewWriter(payload)
	tw := tar.NewWriter(gw)

	if err := cutil.WriteBytesToPackage(ccName, bytecode, tw); err != nil {
		return nil, fmt.Errorf("failed to write bytes to tar: %s", err)
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("failed closing tar writer: %s", err)
	}

	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("failed closing gzip writer: %s", err)
	}

	return payload.Bytes(), nil
}
