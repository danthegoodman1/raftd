package raft

import (
	"context"
	"fmt"
)

type (
	Membership struct {
		Leader  Member   `json:"leader"`
		Members []Member `json:"members"`
	}

	Member struct {
		NodeID uint64 `json:"nodeID"`
		Addr   string `json:"addr"`
	}
)

func (rm *RaftManager) GetMembership(ctx context.Context, shard uint64) (*Membership, error) {
	leader, _, available, err := rm.nodeHost.GetLeaderID(shard)
	if err != nil {
		return nil, fmt.Errorf("error getting membership: %e", err)
	}

	if !available {
		return nil, fmt.Errorf("raft membership not avilable")
	}

	membership, err := rm.nodeHost.SyncGetShardMembership(ctx, shard)
	if err != nil {
		return nil, fmt.Errorf("error in nodeHost.SyncGetShardMembership: %w", err)
	}

	m := &Membership{}
	for id, node := range membership.Nodes {
		if id == leader {
			m.Leader = Member{
				NodeID: id,
				Addr:   node,
			}
		}
		m.Members = append(m.Members, Member{
			NodeID: id,
			Addr:   node,
		})
	}

	return m, nil
}
