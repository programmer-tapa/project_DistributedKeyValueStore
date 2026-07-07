// Package shardctrler implements the shard controller (configuration manager).
package shardctrler

import (
	"encoding/json"
	"errors"
	"sort"

	"dkv/internal/core"
	"dkv/internal/kvsrv"
	"dkv/internal/shardgrp"
)

// ShardController manages shard-to-group assignments.
type ShardController struct {
	kvsrvClerk *kvsrv.Clerk
	groups     map[int][]string
	network    core.Network
}

// New creates a ShardController connected to the given kvsrv and shard groups.
func New(kvsrvAddr string, groups map[int][]string, network core.Network) *ShardController {
	return &ShardController{
		kvsrvClerk: kvsrv.NewClerk(kvsrvAddr, network),
		groups:     groups,
		network:    network,
	}
}

// InitConfig initializes the first configuration, assigning all shards
// evenly across available groups.
func (sc *ShardController) InitConfig(groups map[int][]string) error {
	lock := kvsrv.NewLock(sc.kvsrvClerk, "config_lock")
	lock.Acquire()
	defer lock.Release()

	_, _, err := sc.kvsrvClerk.Get("config")
	if err == core.OK {
		return nil
	}

	config := core.ShardConfig{
		Num:    0,
		Groups: groups,
	}

	var gids []int
	for gid := range groups {
		gids = append(gids, gid)
	}
	sort.Ints(gids)

	if len(gids) > 0 {
		for i := 0; i < core.NShards; i++ {
			config.Shards[i] = gids[i%len(gids)]
		}
	}

	for shardIdx := 0; shardIdx < core.NShards; shardIdx++ {
		dstGid := config.Shards[shardIdx]
		if dstGid != 0 {
			dstServers := config.Groups[dstGid]
			dstClerk := shardgrp.NewClerk(dstServers, sc.network)

			installArgs := core.InstallShardArgs{
				Shard:     shardIdx,
				ConfigNum: 0,
				Data:      nil,
			}
			installReply := dstClerk.InstallShard(&installArgs)
			if installReply.Err != core.OK {
				return errors.New("failed to install initial shard")
			}
		}
	}

	data, jsonErr := json.Marshal(config)
	if jsonErr != nil {
		return jsonErr
	}

	putErr := sc.kvsrvClerk.Put("config", string(data), 0)
	if putErr != core.OK {
		return errors.New("failed to write initial config")
	}

	return nil
}

// Query returns the current ShardConfig.
func (sc *ShardController) Query() (core.ShardConfig, error) {
	val, _, err := sc.kvsrvClerk.Get("config")
	if err == core.ErrNoKey {
		return core.ShardConfig{Num: -1}, nil
	}
	if err != core.OK {
		return core.ShardConfig{}, errors.New("failed to query config")
	}

	var config core.ShardConfig
	if jsonErr := json.Unmarshal([]byte(val), &config); jsonErr != nil {
		return core.ShardConfig{}, jsonErr
	}
	return config, nil
}

// ChangeConfigTo orchestrates a migration to the target configuration.
func (sc *ShardController) ChangeConfigTo(target core.ShardConfig) error {
	lock := kvsrv.NewLock(sc.kvsrvClerk, "config_lock")
	lock.Acquire()
	defer lock.Release()

	val, version, err := sc.kvsrvClerk.Get("config")
	if err != core.OK {
		return errors.New("failed to get current config")
	}

	var current core.ShardConfig
	if jsonErr := json.Unmarshal([]byte(val), &current); jsonErr != nil {
		return jsonErr
	}

	if target.Num <= current.Num {
		return nil
	}

	for shardIdx := 0; shardIdx < core.NShards; shardIdx++ {
		srcGid := current.Shards[shardIdx]
		dstGid := target.Shards[shardIdx]

		if srcGid != dstGid {
			if srcGid != 0 {
				srcServers := current.Groups[srcGid]
				srcClerk := shardgrp.NewClerk(srcServers, sc.network)

				freezeArgs := core.FreezeShardArgs{
					Shard:     shardIdx,
					ConfigNum: target.Num,
				}
				freezeReply := srcClerk.FreezeShard(&freezeArgs)
				if freezeReply.Err != core.OK {
					return errors.New("failed to freeze shard")
				}

				dstServers := target.Groups[dstGid]
				dstClerk := shardgrp.NewClerk(dstServers, sc.network)

				installArgs := core.InstallShardArgs{
					Shard:     shardIdx,
					ConfigNum: target.Num,
					Data:      freezeReply.Data,
				}
				installReply := dstClerk.InstallShard(&installArgs)
				if installReply.Err != core.OK {
					return errors.New("failed to install shard")
				}

				deleteArgs := core.DeleteShardArgs{
					Shard:     shardIdx,
					ConfigNum: target.Num,
				}
				deleteReply := srcClerk.DeleteShard(&deleteArgs)
				if deleteReply.Err != core.OK {
					return errors.New("failed to delete shard")
				}
			} else {
				dstServers := target.Groups[dstGid]
				dstClerk := shardgrp.NewClerk(dstServers, sc.network)

				installArgs := core.InstallShardArgs{
					Shard:     shardIdx,
					ConfigNum: target.Num,
					Data:      nil,
				}
				installReply := dstClerk.InstallShard(&installArgs)
				if installReply.Err != core.OK {
					return errors.New("failed to install shard")
				}
			}
		}
	}

	data, jsonErr := json.Marshal(target)
	if jsonErr != nil {
		return jsonErr
	}

	putErr := sc.kvsrvClerk.Put("config", string(data), version)
	if putErr != core.OK {
		return errors.New("failed to publish new config")
	}

	return nil
}
