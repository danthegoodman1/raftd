package raft

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/danthegoodman1/raftd/utils"

	"github.com/danthegoodman1/raftd/env"
	"github.com/danthegoodman1/raftd/gologger"
	"github.com/lni/dragonboat/v4"
	"github.com/lni/dragonboat/v4/config"
	dragonlogger "github.com/lni/dragonboat/v4/logger"
	"github.com/lni/dragonboat/v4/statemachine"
	"github.com/rs/zerolog"
)

const replicaStatusFile = "replica_status.json"

type (
	RaftManager struct {
		nodeHost *dragonboat.NodeHost
		logger   zerolog.Logger
		Ready    *atomic.Uint64
	}

	raftReplicaStatus struct {
		Shards    map[uint64]struct{}
		ReplicaID uint64
	}
)

var (
	ErrInvalidPeer = errors.New("invalid peer")
)

func NewRaftManager(readyPtr *atomic.Uint64) (*RaftManager, error) {
	logger := gologger.NewLogger().With().Str("Service", "RaftManager").Logger()

	// Create raft storage directory if it doesn't exist
	if err := os.MkdirAll(env.RaftStorageDirectory, 0755); err != nil {
		return nil, fmt.Errorf("error creating raft storage directory: %w", err)
	}

	// Load or create replica status
	statusPath := filepath.Join(env.RaftStorageDirectory, replicaStatusFile)
	var status raftReplicaStatus

	if data, err := os.ReadFile(statusPath); err != nil {
		if os.IsNotExist(err) {
			// Initialize new status with shard 0
			status = raftReplicaStatus{
				Shards:    map[uint64]struct{}{0: {}},
				ReplicaID: env.ReplicaID,
			}
			// Save the initial status
			if data, err := json.Marshal(status); err != nil {
				return nil, fmt.Errorf("error marshaling initial replica status: %w", err)
			} else if err := utils.WriteFileAtomic(statusPath, data, 0644); err != nil {
				return nil, fmt.Errorf("error writing initial replica status: %w", err)
			}
		} else {
			return nil, fmt.Errorf("error reading replica status file: %w", err)
		}
	} else if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("error unmarshaling replica status: %w", err)
	}

	if env.ReplicaID != status.ReplicaID {
		logger.Fatal().Msgf("detected different replica IDs: %d != %d. If this is intentional, you must wipe the raft storage directory (you will lose all data on this replica!)", env.ReplicaID, status.ReplicaID)
	}

	// Validate the application url
	_, err := url.Parse(env.ApplicationURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing application url: %w", err)
	}

	// TODO poll application url to see if it's ready
	readyPtr.Store(1)

	// TODO allow customization of configuration options (esp snapshot entries)
	rc := config.Config{
		ReplicaID:          env.ReplicaID,
		ElectionRTT:        10,
		HeartbeatRTT:       1,
		CheckQuorum:        true,
		SnapshotEntries:    1000,
		CompactionOverhead: 5,
		ShardID:            0, // initial shard
	}
	datadir := filepath.Join(env.RaftStorageDirectory, fmt.Sprintf("node%d", env.ReplicaID))
	nhc := config.NodeHostConfig{
		WALDir:         datadir,
		NodeHostDir:    datadir,
		RTTMillisecond: 3,
		RaftAddress:    env.RaftListenAddr,
	}
	dragonlogger.SetLoggerFactory(CreateLogger)
	nh, err := dragonboat.NewNodeHost(nhc)
	if err != nil {
		panic(err)
	}

	initialMembers := map[uint64]dragonboat.Target{}
	for _, peerPair := range strings.Split(env.RaftInitialMembers, ",") {
		idAddrPair := strings.SplitN(peerPair, "=", 2)
		if len(idAddrPair) != 2 {
			return nil, fmt.Errorf("invalid peer pair '%s', should be in the format replicaID=addr: %w", peerPair, ErrInvalidPeer)
		}

		replicaID, err := strconv.ParseInt(idAddrPair[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing peer node ID: %w", err)
		}

		initialMembers[uint64(replicaID)] = idAddrPair[1]
	}

	if len(initialMembers) > 0 {
		logger.Debug().Interface("raft initial members", initialMembers).Msg("Using raft initial members")
	}

	// Check if we are one of the initial members or not
	join := false
	if !strings.Contains(env.RaftInitialMembers, env.RaftListenAddr) {
		// We are not an initial member, and are thus joining
		join = true
		initialMembers = nil
	}

	// TODO recover each shard
	err = nh.StartOnDiskReplica(initialMembers, join, func(shardID uint64, replicaID uint64) statemachine.IOnDiskStateMachine {
		return createStateMachine(shardID, replicaID, logger, readyPtr)
	}, rc)
	if err != nil {
		return nil, fmt.Errorf("error in StartOnDiskCluster: %w", err)
	}

	rm := &RaftManager{
		nodeHost: nh,
		logger:   logger,
		Ready:    readyPtr,
	}

	return rm, nil
}

func (rm *RaftManager) Shutdown() error {
	rm.nodeHost.Close()
	// todo stop processing new requests
	// todo stop/abandon any outgoing state machine operations
	return nil
}

func (rm *RaftManager) RecruitReplica(ctx context.Context, replicaID, shardID uint64, replicaAddr string) error {
	// Add deadline if not set
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
	}

	return rm.nodeHost.SyncRequestAddReplica(ctx, shardID, replicaID, replicaAddr, 0)
}

func (rm *RaftManager) RemoveReplica(ctx context.Context, replicaID, shardID uint64) error {
	// Add deadline if not set
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
	}

	return rm.nodeHost.SyncRequestDeleteReplica(ctx, shardID, replicaID, 0)
}
