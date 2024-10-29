package raft

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/lni/dragonboat/v4/statemachine"
	"io"
	"net/http"
	"net/url"
	"time"
)

type (
	OnDiskStateMachine struct {
		APPUrl url.URL
	}
)

func createStateMachine(shardID, replicaID uint64, appURL url.URL) statemachine.IOnDiskStateMachine {
	return &OnDiskStateMachine{
		APPUrl: appURL,
	}
}

const timeout = time.Second // todo make customizable

type openResponse struct {
	LastLogIndex uint64
}

func (o *OnDiskStateMachine) Open(stopc <-chan struct{}) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", o.APPUrl.String()+"/LastLogIndex", nil)
	if err != nil {
		return -1, fmt.Errorf("error in NewRequestWithContext: %w", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return -1, fmt.Errorf("error in http.Do: %w", err)
	}
	defer res.Body.Close()

	var resBody openResponse
	resBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return -1, fmt.Errorf("error in io.ReadAll: %w", err)
	}

	err = json.Unmarshal(resBytes, &resBody)
	if err != nil {
		return -1, fmt.Errorf("error in json.Unmarshal: %w", err)
	}

	return resBody.LastLogIndex, nil
}

func (o *OnDiskStateMachine) Update(entries []statemachine.Entry) ([]statemachine.Entry, error) {
	// TODO implement me
	panic("implement me")
}

func (o *OnDiskStateMachine) Lookup(i interface{}) (interface{}, error) {
	// TODO implement me
	panic("implement me")
}

func (o *OnDiskStateMachine) Sync() error {
	// TODO implement me
	panic("implement me")
}

func (o *OnDiskStateMachine) PrepareSnapshot() (interface{}, error) {
	// TODO implement me
	panic("implement me")
}

func (o *OnDiskStateMachine) SaveSnapshot(i interface{}, writer io.Writer, i2 <-chan struct{}) error {
	// TODO implement me
	panic("implement me")
}

func (o *OnDiskStateMachine) RecoverFromSnapshot(reader io.Reader, i <-chan struct{}) error {
	// TODO implement me
	panic("implement me")
}

func (o *OnDiskStateMachine) Close() error {
	// TODO implement me
	panic("implement me")
}
