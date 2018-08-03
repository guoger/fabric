// Code generated by counterfeiter. DO NOT EDIT.
package raftfakes

import (
	"sync"
	"time"

	"github.com/hyperledger/fabric/orderer/consensus/raft"
)

type FakeTicker struct {
	SignalStub        func() <-chan time.Time
	signalMutex       sync.RWMutex
	signalArgsForCall []struct{}
	signalReturns     struct {
		result1 <-chan time.Time
	}
	signalReturnsOnCall map[int]struct {
		result1 <-chan time.Time
	}
	StopStub         func()
	stopMutex        sync.RWMutex
	stopArgsForCall  []struct{}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeTicker) Signal() <-chan time.Time {
	fake.signalMutex.Lock()
	ret, specificReturn := fake.signalReturnsOnCall[len(fake.signalArgsForCall)]
	fake.signalArgsForCall = append(fake.signalArgsForCall, struct{}{})
	fake.recordInvocation("Signal", []interface{}{})
	fake.signalMutex.Unlock()
	if fake.SignalStub != nil {
		return fake.SignalStub()
	}
	if specificReturn {
		return ret.result1
	}
	return fake.signalReturns.result1
}

func (fake *FakeTicker) SignalCallCount() int {
	fake.signalMutex.RLock()
	defer fake.signalMutex.RUnlock()
	return len(fake.signalArgsForCall)
}

func (fake *FakeTicker) SignalReturns(result1 <-chan time.Time) {
	fake.SignalStub = nil
	fake.signalReturns = struct {
		result1 <-chan time.Time
	}{result1}
}

func (fake *FakeTicker) SignalReturnsOnCall(i int, result1 <-chan time.Time) {
	fake.SignalStub = nil
	if fake.signalReturnsOnCall == nil {
		fake.signalReturnsOnCall = make(map[int]struct {
			result1 <-chan time.Time
		})
	}
	fake.signalReturnsOnCall[i] = struct {
		result1 <-chan time.Time
	}{result1}
}

func (fake *FakeTicker) Stop() {
	fake.stopMutex.Lock()
	fake.stopArgsForCall = append(fake.stopArgsForCall, struct{}{})
	fake.recordInvocation("Stop", []interface{}{})
	fake.stopMutex.Unlock()
	if fake.StopStub != nil {
		fake.StopStub()
	}
}

func (fake *FakeTicker) StopCallCount() int {
	fake.stopMutex.RLock()
	defer fake.stopMutex.RUnlock()
	return len(fake.stopArgsForCall)
}

func (fake *FakeTicker) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.signalMutex.RLock()
	defer fake.signalMutex.RUnlock()
	fake.stopMutex.RLock()
	defer fake.stopMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeTicker) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ raft.Ticker = new(FakeTicker)
