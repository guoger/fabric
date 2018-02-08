package evmscc

import (
	"github.com/hyperledger/burrow/account"
	"github.com/hyperledger/burrow/binary"
	"github.com/hyperledger/fabric/core/chaincode/shim"
)

type StateManager interface {
	GetAccount(address account.Address) (account.Account, error)
	GetStorage(address account.Address, key binary.Word256) (binary.Word256, error)
	UpdateAccount(updatedAccount account.Account) error
	RemoveAccount(address account.Address) error
	SetStorage(address account.Address, key, value binary.Word256) error
}

type stateManager struct {
	stub shim.ChaincodeStubInterface
}

func NewStateManager(stub shim.ChaincodeStubInterface) StateManager {
	return &stateManager{stub: stub}
}

func (s *stateManager) GetAccount(address account.Address) (account.Account, error) {
	convAddr := address.String()
	code, err := s.stub.GetState(convAddr)

	if err != nil {
		return account.ConcreteAccount{}.Account(), err
	}

	if code == nil {
		return account.ConcreteAccount{}.Account(), nil
	}

	return account.ConcreteAccount{
		Address: address,
		Code:    code,
	}.Account(), nil
}

func (s *stateManager) GetStorage(address account.Address, key binary.Word256) (binary.Word256, error) {

	compKey := string(address.Bytes()) + string(key.Bytes())

	val, err := s.stub.GetState(compKey)
	if err != nil {
		return binary.Word256{}, err
	}

	var convVal binary.Word256
	copy(convVal[:], val[:])

	return convVal, nil
}

func (s *stateManager) UpdateAccount(updatedAccount account.Account) error {
	convAddr := string(updatedAccount.Address().Bytes())

	return s.stub.PutState(convAddr, updatedAccount.Code().Bytes())
}

func (s *stateManager) RemoveAccount(address account.Address) error {
	convAddr := string(address.Bytes())

	return s.stub.DelState(convAddr)
}

func (s *stateManager) SetStorage(address account.Address, key, value binary.Word256) error {
	compKey := string(address.Bytes()) + string(key.Bytes())

	return s.stub.PutState(compKey, value.Bytes())
}

//we may want to use some of the functions that are in burrow
func stringToAddress(addr string) account.Address {

	var convAddr account.Address
	copy(convAddr[:], []byte(addr)[:])

	return convAddr
}
