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
	"io"

	"github.com/golang/protobuf/proto"
	acm "github.com/hyperledger/burrow/account"
	"github.com/hyperledger/burrow/binary"
	"github.com/hyperledger/burrow/execution/evm"
	"github.com/hyperledger/burrow/logging/lifecycle"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos/peer"
)

var logger = flogging.MustGetLogger("evmscc")
var evmLogger, _ = lifecycle.NewStdErrLogger()

type EvmChaincode struct {
}

func (evmcc *EvmChaincode) Init(stub shim.ChaincodeStubInterface) pb.Response {
	// We always expect 2 args: chaincode name, chaincode deployment spec
	args := stub.GetArgs()
	if len(args) != 2 {
		return shim.Error(fmt.Sprintf("expects 2 args, got %d", len(args)))
	}

	ccName := args[0]

	logger.Debugf("[channel: %s] instantiate EVM chaincode '%s'", stub.GetChannelID(), ccName)
	cds := &pb.ChaincodeDeploymentSpec{}
	if err := proto.Unmarshal(args[1], cds); err != nil {
		return shim.Error(fmt.Sprintf("failed to unmarshal ChaincodeDeploymentSpec: %s", err.Error()))
	}

	if cds.CodePackage == nil {
		return shim.Error(fmt.Sprintf("failed to find codepackage in '%s' chaincode deployment spec", ccName))
	}

	bytecode, err := decodeBytecode(string(ccName), cds.CodePackage)
	if err != nil {
		return shim.Error(fmt.Sprintf("failed to decode bytecode for %s from package: %s", ccName, err.Error()))
	}

	if err = stub.PutState(string(ccName), bytecode); err != nil {
		return shim.Error(fmt.Sprintf("failed to put bytecode into ledger: %s", err.Error()))
	}

	logger.Debugf("[channel: %s] put evm chaincode '%s' (%d bytes) into evmscc ledger", stub.GetChannelID(), ccName, len(bytecode))

	return shim.Success(nil)
}

func (evmcc *EvmChaincode) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	// We always expect 2 args: chaincode name, input data
	args := stub.GetArgs()
	if len(args) != 2 {
		return shim.Error(fmt.Sprintf("expects 2 args, got %d", len(args)))
	}

	ccName := args[0]
	call := make([]byte, hex.DecodedLen(len(args[1])))
	if _, err := hex.Decode(call, args[1]); err != nil {
		return shim.Error(err.Error())
	}

	logger.Debugf("[channel: %s] invoke EVM chaincode '%s' with arg: '%x'",stub.GetChannelID(), ccName, call)

	bytecode, err := stub.GetState(string(ccName))
	if err != nil {
		return shim.Error(fmt.Sprintf("failed to retrieve bytecode of chaincode '%s': %s", ccName, err.Error()))
	}

	if bytecode == nil {
		return shim.Error(fmt.Sprintf("bytecode for chaincode '%s' is nil", ccName))
	}

	logger.Debugf("[channel: %s] retrieved %d bytes for chaincode %s from ledger",stub.GetChannelID(), len(bytecode), ccName)

	vm := evm.NewVM(&stateWriter{stub}, evm.DefaultDynamicMemoryProvider, newParams(), acm.ZeroAddress, nil, evmLogger)

	// Create accounts
	account1 := ccNameToAccount([]byte("evmscc"))
	account2 := ccNameToAccount(ccName)

	// hard-code 100000 gas for now
	var gas uint64 = 100000
	output, err := vm.Call(account1, account2, bytecode, call, 0, &gas)
	if err != nil {
		return shim.Error(fmt.Sprintf("evm execution failed: %s", err.Error()))
	}

	return shim.Success(output)
}

func newParams() evm.Params {
	return evm.Params{
		BlockHeight: 0,
		BlockHash:   binary.Zero256,
		BlockTime:   0,
		GasLimit:    0,
	}
}

func ccNameToAccount(ccName []byte) acm.MutableAccount {
	return acm.ConcreteAccount{
		Address: acm.AddressFromWord256(binary.LeftPadWord256(ccName)),
	}.MutableAccount()
}

func decodeBytecode(filename string, src []byte) ([]byte, error) {
	r := bytes.NewReader(src)
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	tr := tar.NewReader(zr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		if hdr.Name == filename {
			buf := new(bytes.Buffer)
			if _, err := io.Copy(buf, tr); err != nil {
				return nil, err
			}

			raw := buf.Bytes()
			bytecode := make([]byte, hex.DecodedLen(len(raw)))
			if _, err = hex.Decode(bytecode, raw); err != nil {
				return nil, err
			}

			return bytecode, nil
		}
	}

	return nil, fmt.Errorf("failed to find bytecode '%s' in package", filename)
}
