package raft

import (
	"bytes"
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

func doReqWithContext[T any](ctx context.Context, method string, url string, body io.Reader) (T, error) {
	var defaultResponse T

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return defaultResponse, fmt.Errorf("error in NewRequestWithContext: %w", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return defaultResponse, fmt.Errorf("error in http.Do: %w", err)
	}
	defer res.Body.Close()

	var resBody T
	resBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return defaultResponse, fmt.Errorf("error in io.ReadAll: %w", err)
	}

	err = json.Unmarshal(resBytes, &resBody)
	if err != nil {
		return defaultResponse, fmt.Errorf("error in json.Unmarshal: %w", err)
	}

	return resBody, nil
}

func (o *OnDiskStateMachine) Open(stopc <-chan struct{}) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Create a goroutine to cancel the context if stopc is triggered
	go func() {
		select {
		case <-stopc:
			cancel()
		case <-ctx.Done():
			return
		}
	}()

	res, err := doReqWithContext[struct {
		LastLogIndex uint64
	}](ctx, "POST", o.APPUrl.String()+"/LastLogIndex", nil)
	if err != nil {
		return -1, fmt.Errorf("error in NewRequestWithContext: %w", err)
	}

	return res.LastLogIndex, nil
}

func (o *OnDiskStateMachine) Update(entries []statemachine.Entry) ([]statemachine.Entry, error) {
	// TODO implement me
	panic("implement me")
}

func (o *OnDiskStateMachine) Lookup(i interface{}) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	jsonBytes, err := json.Marshal(i)
	if err != nil {
		return nil, fmt.Errorf("error in json.Marshal: %w", err)
	}

	res, err := doReqWithContext[any](ctx, "POST", o.APPUrl.String()+"/Lookup", bytes.NewReader(jsonBytes))
	if err != nil {
		return -1, fmt.Errorf("error in NewRequestWithContext: %w", err)
	}

	return res, nil
}

func (o *OnDiskStateMachine) Sync() error {
	// TODO implement me
	panic("implement me")
}

func (o *OnDiskStateMachine) PrepareSnapshot() (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	res, err := doReqWithContext[any](ctx, "POST", o.APPUrl.String()+"/PrepareSnapshot", nil)
	if err != nil {
		return -1, fmt.Errorf("error in NewRequestWithContext: %w", err)
	}

	return res, nil
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
