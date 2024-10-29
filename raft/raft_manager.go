package raft

import (
	"errors"
	"fmt"
	"github.com/danthegoodman1/raftd/gologger"
	"github.com/danthegoodman1/raftd/utils"
	"github.com/lni/dragonboat/v4"
	"github.com/lni/dragonboat/v4/config"
	dragonlogger "github.com/lni/dragonboat/v4/logger"
	"github.com/lni/dragonboat/v4/statemachine"
	"github.com/rs/zerolog"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

type (
	RaftManager struct {
		nodeHost *dragonboat.NodeHost
		logger   zerolog.Logger
	}
)

var (
	ErrInvalidPeer = errors.New("invalid peer")
)

func NewRaftManager() (*RaftManager, error) {
	logger := gologger.NewLogger().With().Str("Service", "RaftManager").Logger()
	// validate the application url
	_, err := url.Parse(utils.ApplicationURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing application url: %w", err)
	}

	// TODO poll application url to see if it's ready

	// TODO allow customization of configuration options (esp snapshot entries)
	rc := config.Config{
		ReplicaID:          utils.NodeID,
		ElectionRTT:        10,
		HeartbeatRTT:       1,
		CheckQuorum:        true,
		SnapshotEntries:    1000,
		CompactionOverhead: 5,
	}
	datadir := filepath.Join("_raft", fmt.Sprintf("node%d", utils.NodeID))
	nhc := config.NodeHostConfig{
		WALDir:         datadir,
		NodeHostDir:    datadir,
		RTTMillisecond: 3,
		RaftAddress:    utils.RaftListenAddr,
	}
	dragonlogger.SetLoggerFactory(CreateLogger)
	nh, err := dragonboat.NewNodeHost(nhc)
	if err != nil {
		panic(err)
	}

	var raftPeers map[uint64]dragonboat.Target
	for _, peerPair := range strings.Split(utils.RaftPeers, ",") {
		idAddrPair := strings.SplitN(peerPair, "=", 2)
		if len(idAddrPair) != 2 {
			return nil, fmt.Errorf("invalid peer pair '%s', should be in the format nodeID=addr: %w", peerPair, ErrInvalidPeer)
		}

		nodeID, err := strconv.ParseInt(idAddrPair[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing peer node ID: %w", err)
		}

		raftPeers[uint64(nodeID)] = idAddrPair[1]
	}

	err = nh.StartOnDiskReplica(raftPeers, false, func(shardID uint64, replicaID uint64) statemachine.IOnDiskStateMachine {
		return createStateMachine(shardID, replicaID, logger)
	}, rc)
	if err != nil {
		return nil, fmt.Errorf("error in StartOnDiskCluster: %w", err)
	}

	rm := &RaftManager{
		nodeHost: nh,
		logger:   logger,
	}

	return rm, nil
}

func (rm *RaftManager) Shutdown() error {
	rm.nodeHost.Close()
	// todo stop processing new requests
	// todo stop/abandon any outgoing state machine operations
	return nil
}
