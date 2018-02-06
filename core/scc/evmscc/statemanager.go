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
	"github.com/hyperledger/burrow/account"
	"github.com/hyperledger/burrow/binary"
	"github.com/hyperledger/fabric/core/chaincode/shim"
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
