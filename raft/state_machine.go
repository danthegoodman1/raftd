package raft

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lni/dragonboat/v4/statemachine"
	"github.com/samber/lo"
	"io"
	"net/http"
	"net/url"
	"time"
)

type (
	OnDiskStateMachine struct {
		APPUrl    *url.URL
		shardID   uint64
		replicaID uint64
	}
)

func createStateMachine(shardID, replicaID uint64, appURL *url.URL) statemachine.IOnDiskStateMachine {
	return &OnDiskStateMachine{
		shardID:   shardID,
		replicaID: replicaID,
		APPUrl:    appURL,
	}
}

var (
	ErrHighStatusCode = errors.New("high status code")
)

const (
	timeout         = time.Second // todo make customizable
	snapshotTimeout = time.Minute // todo make customizable
)

func genHighStatusCodeError(statusCode int, body io.Reader) error {
	allBytes, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("error in io.ReadAll: %w", err)
	}

	return fmt.Errorf("%w (%d): %s", ErrHighStatusCode, statusCode, string(allBytes)[:100])
}

func doReqWithContext[T any](ctx context.Context, url string, body io.Reader) (T, error) {
	var defaultResponse T

	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return defaultResponse, fmt.Errorf("error in NewRequestWithContext: %w", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return defaultResponse, fmt.Errorf("error in http.Do: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode > 299 {
		return defaultResponse, genHighStatusCodeError(res.StatusCode, res.Body)
	}

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
	}](ctx, o.APPUrl.String()+"/LastLogIndex", nil)
	if err != nil {
		return -1, fmt.Errorf("error in NewRequestWithContext: %w", err)
	}

	return res.LastLogIndex, nil
}

type (
	updateEntry struct {
		Index uint64
		Cmd   []byte
	}
	updateResponse struct {
		Results []statemachine.Result
	}
)

func (o *OnDiskStateMachine) Update(entries []statemachine.Entry) ([]statemachine.Entry, error) {
	jsonBytes, err := json.Marshal(map[string]any{
		"Entries": lo.Map(entries, func(entry statemachine.Entry, index int) updateEntry {
			return updateEntry{
				Index: entry.Index,
				Cmd:   entry.Cmd,
			}
		}),
	})
	if err != nil {
		return entries, fmt.Errorf("error in json.Marshal: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	res, err := doReqWithContext[updateResponse](ctx, o.APPUrl.String()+"/UpdateEntries", bytes.NewReader(jsonBytes))
	if err != nil {
		return entries, fmt.Errorf("error in NewRequestWithContext: %w", err)
	}

	// If we have a body with the same length, we will use that those responses
	if len(res.Results) == len(entries) {
		for i := range res.Results {
			entries[i].Result = res.Results[i]
		}
	}

	return entries, nil
}

func (o *OnDiskStateMachine) Lookup(i interface{}) (interface{}, error) {
	jsonBytes, err := json.Marshal(i)
	if err != nil {
		return nil, fmt.Errorf("error in json.Marshal: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	res, err := doReqWithContext[any](ctx, o.APPUrl.String()+"/Read", bytes.NewReader(jsonBytes))
	if err != nil {
		return -1, fmt.Errorf("error in NewRequestWithContext: %w", err)
	}

	return res, nil
}

func (o *OnDiskStateMachine) Sync() error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, err := doReqWithContext[any](ctx, o.APPUrl.String()+"/Sync", nil)
	if err != nil {
		return fmt.Errorf("error in NewRequestWithContext: %w", err)
	}

	return nil
}

func (o *OnDiskStateMachine) PrepareSnapshot() (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	res, err := doReqWithContext[any](ctx, o.APPUrl.String()+"/PrepareSnapshot", nil)
	if err != nil {
		return -1, fmt.Errorf("error in NewRequestWithContext: %w", err)
	}

	return res, nil
}

func (o *OnDiskStateMachine) SaveSnapshot(i interface{}, writer io.Writer, i2 <-chan struct{}) error {
	jsonBytes, err := json.Marshal(i)
	if err != nil {
		return fmt.Errorf("error in json.Marshal: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), snapshotTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", o.APPUrl.String()+"/Snapshot", bytes.NewReader(jsonBytes))
	if err != nil {
		return fmt.Errorf("error in NewRequestWithContext: %w", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error in http.Do: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode > 299 {
		return genHighStatusCodeError(res.StatusCode, res.Body)
	}

	_, err = io.Copy(writer, res.Body)
	if err != nil {
		return fmt.Errorf("error in io.Copy: %w", err)
	}

	return nil
}

func (o *OnDiskStateMachine) RecoverFromSnapshot(reader io.Reader, i <-chan struct{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), snapshotTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", o.APPUrl.String()+"/RecoverFromSnapshot", reader)
	if err != nil {
		return fmt.Errorf("error in NewRequestWithContext: %w", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error in http.Do: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode > 299 {
		return genHighStatusCodeError(res.StatusCode, res.Body)
	}

	return nil
}

func (o *OnDiskStateMachine) Close() error {
	// TODO implement me
	panic("implement me")
}
