// Package metrics provides Prometheus instrumentation for all subsystems.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RaftCurrentTerm tracks the current term of the Raft node.
	RaftCurrentTerm = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "dkv_raft_current_term",
		Help: "Current term of the Raft node",
	}, []string{"node"})

	// RaftLeaderChanges tracks leader election changes.
	RaftLeaderChanges = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "dkv_raft_leader_changes_total",
		Help: "Total number of leader election changes observed",
	}, []string{"node"})

	// RaftLogEntries tracks current log length.
	RaftLogEntries = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "dkv_raft_log_entries",
		Help: "Current length of the Raft log",
	}, []string{"node"})

	// RaftReplicationLag tracks replication lag per follower.
	RaftReplicationLag = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "dkv_raft_replication_lag",
		Help: "Current replication lag per follower",
	}, []string{"node", "follower"})

	// RaftSnapshotSize tracks size of Raft snapshots in bytes.
	RaftSnapshotSize = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "dkv_raft_snapshot_size_bytes",
		Help: "Size of the Raft snapshot in bytes",
	}, []string{"node"})

	// RaftElectionDuration tracks election durations in seconds.
	RaftElectionDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "dkv_raft_election_duration_seconds",
		Help:    "Duration of Raft elections in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"node"})

	// KVOperations tracks key-value operations.
	KVOperations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "dkv_kv_operations_total",
		Help: "Total number of client key-value operations",
	}, []string{"node", "op_type"})

	// KVOperationDuration tracks latency of client key-value operations.
	KVOperationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "dkv_kv_operation_duration_seconds",
		Help:    "Latency of client key-value operations in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"node", "op_type"})

	// KVStoreSize tracks number of keys currently in the store.
	KVStoreSize = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "dkv_kv_store_size",
		Help: "Number of keys currently in the store",
	}, []string{"node"})

	// ShardConfigNum tracks current configuration number.
	ShardConfigNum = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "dkv_shard_config_num",
		Help: "Current configuration number of the shard controller",
	})

	// ShardMigrationDuration tracks duration of shard migrations in seconds.
	ShardMigrationDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "dkv_shard_migration_duration_seconds",
		Help:    "Duration of shard migrations in seconds",
		Buckets: prometheus.DefBuckets,
	})

	// ShardMigrations tracks shard migration phases.
	ShardMigrations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "dkv_shard_migrations_total",
		Help: "Total number of shard migrations",
	}, []string{"phase"})

	// RPCSent tracks total RPCs sent.
	RPCSent = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "dkv_rpc_sent_total",
		Help: "Total number of RPCs sent",
	}, []string{"method"})

	// RPCDuration tracks latency of RPCs in seconds.
	RPCDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "dkv_rpc_duration_seconds",
		Help:    "Latency of RPCs in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method"})

	// RPCErrors tracks RPC errors.
	RPCErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "dkv_rpc_errors_total",
		Help: "Total number of RPC errors",
	}, []string{"method", "error_type"})
)
