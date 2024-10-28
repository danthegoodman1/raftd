package raft

import (
	"github.com/lni/dragonboat/v4/statemachine"
	"io"
)

type (
	OnDiskStateMachine struct {
	}
)

func createStateMachine(shardID, replicaID uint64) statemachine.IOnDiskStateMachine {
	return OnDiskStateMachine{}
}

func (o OnDiskStateMachine) Open(stopc <-chan struct{}) (uint64, error) {
	// TODO implement me
	panic("implement me")
}

func (o OnDiskStateMachine) Update(entries []statemachine.Entry) ([]statemachine.Entry, error) {
	// TODO implement me
	panic("implement me")
}

func (o OnDiskStateMachine) Lookup(i interface{}) (interface{}, error) {
	// TODO implement me
	panic("implement me")
}

func (o OnDiskStateMachine) Sync() error {
	// TODO implement me
	panic("implement me")
}

func (o OnDiskStateMachine) PrepareSnapshot() (interface{}, error) {
	// TODO implement me
	panic("implement me")
}

func (o OnDiskStateMachine) SaveSnapshot(i interface{}, writer io.Writer, i2 <-chan struct{}) error {
	// TODO implement me
	panic("implement me")
}

func (o OnDiskStateMachine) RecoverFromSnapshot(reader io.Reader, i <-chan struct{}) error {
	// TODO implement me
	panic("implement me")
}

func (o OnDiskStateMachine) Close() error {
	// TODO implement me
	panic("implement me")
}
