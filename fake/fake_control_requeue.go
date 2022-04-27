// Code generated by counterfeiter. DO NOT EDIT.
package fake

import (
	"sync"

	"github.com/authzed/controller-idioms"
)

type FakeControlRequeue struct {
	RequeueStub        func()
	requeueMutex       sync.RWMutex
	requeueArgsForCall []struct {
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeControlRequeue) Requeue() {
	fake.requeueMutex.Lock()
	fake.requeueArgsForCall = append(fake.requeueArgsForCall, struct {
	}{})
	stub := fake.RequeueStub
	fake.recordInvocation("Requeue", []interface{}{})
	fake.requeueMutex.Unlock()
	if stub != nil {
		fake.RequeueStub()
	}
}

func (fake *FakeControlRequeue) RequeueCallCount() int {
	fake.requeueMutex.RLock()
	defer fake.requeueMutex.RUnlock()
	return len(fake.requeueArgsForCall)
}

func (fake *FakeControlRequeue) RequeueCalls(stub func()) {
	fake.requeueMutex.Lock()
	defer fake.requeueMutex.Unlock()
	fake.RequeueStub = stub
}

func (fake *FakeControlRequeue) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.requeueMutex.RLock()
	defer fake.requeueMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeControlRequeue) recordInvocation(key string, args []interface{}) {
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

var _ libctrl.ControlRequeue = new(FakeControlRequeue)
