package raft

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/danthegoodman1/raftd/utils"
	"github.com/lni/dragonboat/v4/statemachine"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
	"io"
	"net/http"
	"time"
)

type (
	OnDiskStateMachine struct {
		APPUrl     string
		shardID    uint64
		replicaID  uint64
		shouldSync bool
		closed     bool
		logger     zerolog.Logger
	}
)

func createStateMachine(shardID, replicaID uint64, logger zerolog.Logger) statemachine.IOnDiskStateMachine {
	childLogger := logger.With().Uint64("NodeID", shardID).Uint64("ReplicaID", replicaID).Str("Service", "RaftStateMachine").Logger()
	return &OnDiskStateMachine{
		shardID:    shardID,
		replicaID:  replicaID,
		APPUrl:     utils.ApplicationURL,
		shouldSync: utils.RaftSync,
		logger:     childLogger,
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

func doReqWithContext[T any](ctx context.Context, shardID, replicaID uint64, url string, body io.Reader) (T, error) {
	var defaultResponse T

	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return defaultResponse, fmt.Errorf("error in NewRequestWithContext: %w", err)
	}
	req.Header.Set("raftd-node-id", fmt.Sprint(shardID))
	req.Header.Set("raftd-replica-id", fmt.Sprint(replicaID))

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
	o.logger.Debug().Msg("calling open")
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
	}](ctx, o.shardID, o.replicaID, o.APPUrl+"/LastLogIndex", nil)
	if err != nil {
		return 0, fmt.Errorf("error in NewRequestWithContext: %w", err)
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
	o.logger.Debug().Msg("calling update")
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

	res, err := doReqWithContext[updateResponse](ctx, o.shardID, o.replicaID, o.APPUrl+"/UpdateEntries", bytes.NewReader(jsonBytes))
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
	o.logger.Debug().Msg("calling lookup")
	jsonBytes, err := json.Marshal(i)
	if err != nil {
		return nil, fmt.Errorf("error in json.Marshal: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	res, err := doReqWithContext[any](ctx, o.shardID, o.replicaID, o.APPUrl+"/Read", bytes.NewReader(jsonBytes))
	if err != nil {
		return 0, fmt.Errorf("error in NewRequestWithContext: %w", err)
	}

	return res, nil
}

func (o *OnDiskStateMachine) Sync() error {
	o.logger.Debug().Msg("calling sync")
	if o.closed {
		o.logger.Fatal().Msg("Sync called after close!")
	}
	if !o.shouldSync {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, err := doReqWithContext[any](ctx, o.shardID, o.replicaID, o.APPUrl+"/Sync", nil)
	if err != nil {
		return fmt.Errorf("error in NewRequestWithContext: %w", err)
	}

	return nil
}

func (o *OnDiskStateMachine) PrepareSnapshot() (interface{}, error) {
	o.logger.Info().Msg("calling PrepareSnapshot")
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	res, err := doReqWithContext[any](ctx, o.shardID, o.replicaID, o.APPUrl+"/PrepareSnapshot", nil)
	if err != nil {
		return 0, fmt.Errorf("error in NewRequestWithContext: %w", err)
	}

	return res, nil
}

func (o *OnDiskStateMachine) SaveSnapshot(i interface{}, writer io.Writer, i2 <-chan struct{}) error {
	o.logger.Info().Msg("calling SaveSnapshot")
	jsonBytes, err := json.Marshal(i)
	if err != nil {
		return fmt.Errorf("error in json.Marshal: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), snapshotTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", o.APPUrl+"/Snapshot", bytes.NewReader(jsonBytes))
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
	o.logger.Info().Msg("calling RecoverFromSnapshot")
	ctx, cancel := context.WithTimeout(context.Background(), snapshotTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", o.APPUrl+"/RecoverFromSnapshot", reader)
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
	// We do nothing here, since we want to force the application to be resilient to crashes
	o.logger.Info().Msg("calling Close")
	o.closed = true
	return nil
}
