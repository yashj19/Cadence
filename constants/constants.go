package constants

import "time"

const (
	DefaultPort          = "6380"
	MaxInstructionBuffer = 5
	SHARD_COUNT          = 16 // TODO: take this as input later
	CAPACITY_PER_SHARD   = 100
	SNAPSHOT_INTERVAL    = time.Minute * 5
)
