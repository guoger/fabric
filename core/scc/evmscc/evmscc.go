package evmscc

import (
	"fmt"
	"encoding/hex"

	"github.com/hyperledger/burrow/execution/evm"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos/peer"
	acm "github.com/hyperledger/burrow/account"
	"github.com/hyperledger/burrow/binary"
	"github.com/hyperledger/burrow/logging/lifecycle"
	"github.com/hyperledger/fabric/core/scc/lscc"
	"github.com/golang/protobuf/proto"
)

type EvmChaincode struct {
}

var logger, _ = lifecycle.NewStdErrLogger()

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

func (evmcc *EvmChaincode) Init(stub shim.ChaincodeStubInterface) pb.Response {
	return shim.Success(nil)
}

func (evmcc *EvmChaincode) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	// We always expect 2 args: chaincode name, input data
	args := stub.GetArgs()
	if len(args) != 2 {
		return shim.Error(fmt.Sprintf("expects 2 args, got %d", len(args)))
	}

	ccName := args[0]
	call, err := hex.DecodeString(string(args[1]))
	if err != nil {
		shim.Error(err.Error())
	}

	res := stub.InvokeChaincode("lscc", [][]byte{[]byte(lscc.GETDEPSPEC), []byte(stub.GetChannelID()), ccName}, stub.GetChannelID())

	cds := &pb.ChaincodeDeploymentSpec{}
	if err := proto.Unmarshal(res.Payload, cds); err != nil {
		return shim.Error(err.Error())
	}

	vm := evm.NewVM(&stateWriter{stub}, evm.DefaultDynamicMemoryProvider, newParams(), acm.ZeroAddress, nil, logger)

	// Create accounts
	account1 := ccNameToAccount([]byte("evmscc"))
	account2 := ccNameToAccount(ccName)

	// hard-code 100000 gas for now
	var gas uint64 = 100000
	output, err := vm.Call(account1, account2, cds.CodePackage, call, 0, &gas)
	if err != nil {
		shim.Error(err.Error())
	}

	return shim.Success(output)
}