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
	"encoding/hex"
	"testing"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/stretchr/testify/assert"
	"github.com/hyperledger/burrow/account"
	"github.com/hyperledger/fabric/protos/msp"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/require"
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
const DEPLOY_BYTECODE = "6060604052341561000f57600080fd5b60d38061001d6000396000f3006060604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806360fe47b114604e5780636d4ce63c14606e575b600080fd5b3415605857600080fd5b606c60048080359060200190919050506094565b005b3415607857600080fd5b607e609e565b6040518082815260200191505060405180910390f35b8060008190555050565b600080549050905600a165627a7a72305820122f55f799d70b5f6dbfd4312efb65cdbfaacddedf7c36249b8b1e915a8dd85b0029"
const RUNTIME_BYTECODE = "6060604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806360fe47b114604e5780636d4ce63c14606e575b600080fd5b3415605857600080fd5b606c60048080359060200190919050506094565b005b3415607857600080fd5b607e609e565b6040518082815260200191505060405180910390f35b8060008190555050565b600080549050905600a165627a7a72305820122f55f799d70b5f6dbfd4312efb65cdbfaacddedf7c36249b8b1e915a8dd85b0029"

// Keccak hash of `set` function is:
const SET = "60fe47b1"

// Keccak hash of `get` function is:
const GET = "6d4ce63c"

var callerCert = `-----BEGIN CERTIFICATE-----
MIIB/zCCAaWgAwIBAgIRAKaex32sim4PQR6kDPEPVnwwCgYIKoZIzj0EAwIwaTEL
MAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBG
cmFuY2lzY28xFDASBgNVBAoTC2V4YW1wbGUuY29tMRcwFQYDVQQDEw5jYS5leGFt
cGxlLmNvbTAeFw0xNzA3MjYwNDM1MDJaFw0yNzA3MjQwNDM1MDJaMEoxCzAJBgNV
BAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1TYW4gRnJhbmNp
c2NvMQ4wDAYDVQQDEwVwZWVyMDBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABPzs
BSdIIB0GrKmKWn0N8mMfxWs2s1D6K+xvTvVJ3wUj3znNBxj+k2j2tpPuJUExt61s
KbpP3GF9/crEahpXXRajTTBLMA4GA1UdDwEB/wQEAwIHgDAMBgNVHRMBAf8EAjAA
MCsGA1UdIwQkMCKAIEvLfQX685pz+rh2q5yCA7e0a/a5IGDuJVHRWfp++HThMAoG
CCqGSM49BAMCA0gAMEUCIH5H9W3tsCrti6tsN9UfY1eeTKtExf/abXhfqfVeRChk
AiEA0GxTPOXVHo0gJpMbHc9B73TL5ZfDhujoDyjb8DToWPQ=
-----END CERTIFICATE-----`

func TestInit(t *testing.T) {
	evmscc := new(EvmChaincode)
	stub := shim.NewMockStub("evmscc", evmscc)
	res := stub.MockInit("txid", nil)
	assert.Equal(t, int32(shim.OK), res.Status, "expect evmscc init to be OK")
}

// Invoke and query the example bytecode
func TestEVM(t *testing.T) {
	evmscc := new(EvmChaincode)
	stub := shim.NewMockStub("evmscc", evmscc)
	creator, err := marshalCreator("TestOrg", []byte(callerCert))
	require.NoError(t, err)
	require.NotNil(t, creator)
	stub.Creator = creator

	deployCode := []byte(DEPLOY_BYTECODE)

	// Install
	installRes := stub.MockInvoke("installtxid", [][]byte{[]byte(account.ZeroAddress.String()), deployCode})
	assert.Equal(t, int32(shim.OK), installRes.Status, "expect OK, got: %s", installRes.Message)

	contractAddr, err := account.AddressFromHexString(string(installRes.Payload))
	assert.NoError(t, err)

	runtimeCode, err := stub.GetState(contractAddr.String())
	assert.NoError(t, err)

	// Contract runtime bytecode should be stored at returned address
	assert.Equal(t, RUNTIME_BYTECODE, hex.EncodeToString(runtimeCode))

	// Invoke `set`
	setRes := stub.MockInvoke("invoketxid", [][]byte{[]byte(contractAddr.String()), []byte("60fe47b1000000000000000000000000000000000000000000000000000000000000002a")})
	assert.Equal(t, int32(shim.OK), setRes.Status, "expect OK, got: %s", setRes.Message)

	// Invoke `get`
	getRes := stub.MockInvoke("querytxid", [][]byte{[]byte(contractAddr.String()), []byte("6d4ce63c")})
	assert.Equal(t, int32(shim.OK), getRes.Status, "expect OK, got: %s", getRes.Message)
	assert.Equal(t, "000000000000000000000000000000000000000000000000000000000000002a", hex.EncodeToString(getRes.Payload))
}

func TestIdToAddress(t *testing.T) {
	addr, err := identityToAddr([]byte(callerCert))
	assert.NoError(t, err)
	assert.NotEqual(t, account.ZeroAddress, addr)
}

func marshalCreator(mspId string, certByte []byte) ([]byte, error) {
	b, err := proto.Marshal(&msp.SerializedIdentity{Mspid: mspId, IdBytes: certByte})
	if err != nil {
		return nil, err
	}
	return b, nil
}
