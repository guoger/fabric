package evmscc

import (
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/burrow/account"
	"github.com/hyperledger/burrow/binary"
	"github.com/pkg/errors"
	"github.com/tmthrgd/go-hex"
)

type stateWriter struct {
	stub shim.ChaincodeStubInterface
}

func (s *stateWriter) GetAccount(address account.Address) (account.Account, error) {
	code, err := s.stub.GetState(address.String())
	if err != nil {
		return nil, err
	}

	acc := account.ConcreteAccount{
		Address: address,
		Balance: 0,
		Code:    convertToBytecode(code),
	}

	return acc.Account(), nil

}

func (s *stateWriter) GetStorage(address account.Address, key binary.Word256) (binary.Word256, error) {
	compositeKey, err := s.stub.CreateCompositeKey(address.String(), []string{hex.EncodeUpperToString(key.Bytes())})
	if err != nil {
		return binary.Word256{}, err
	}
	val, err := s.stub.GetState(compositeKey)
	if err != nil {
		return binary.Word256{}, err
	}

	if len(val) != binary.Word256Length {
		return binary.Word256{}, errors.New("Value is greater than 256 bits")
	}
	return convertToWord256(val), nil
}

func (s *stateWriter) UpdateAccount(updatedAccount account.Account) error {
	return s.stub.PutState(updatedAccount.Address().String(), updatedAccount.Code().Bytes())
}

func (s *stateWriter) RemoveAccount(address account.Address) error {
	return s.stub.DelState(address.String())
}

func (s *stateWriter) SetStorage(address account.Address, key, value binary.Word256) error {
	compositeKey, err := s.stub.CreateCompositeKey(address.String(), []string{hex.EncodeUpperToString(key.Bytes())})
	if err != nil {
		return err
	}

	return s.stub.PutState(compositeKey, value.Bytes())
}

func convertToWord256(value []byte) binary.Word256 {
	convertedVal := binary.Word256{}
	copy(convertedVal[:], value)
	return convertedVal
}

func convertToBytecode(value []byte) account.Bytecode {
	convertedVal := account.Bytecode{}
	copy(value[:], convertedVal[:])
	return convertedVal
}
