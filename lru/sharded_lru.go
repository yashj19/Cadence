package lru

type ShardedLRU struct {
	shards []*LRUCache
}

func NewShardedLRU(capacityPerShard int, shardCount int) ShardedLRU {
	slru := ShardedLRU{shards: make([]*LRUCache, 0, shardCount)}
	for i := 0; i < shardCount; i++ {
		slru.shards[i] = NewLRUCache(capacityPerShard)
	}
	return slru
}

func (slru ShardedLRU) getLRU(key string) *LRUCache {
	// perform simple hash for now
	sum := 0
	for i, c := range key {
		sum += 13*i%97 + int(c)*5
	}
	return slru.shards[sum%len(slru.shards)]
}

func (slru *ShardedLRU) Get(key string) (string, bool) {
	return slru.getLRU(key).Get(key)
}

func (slru *ShardedLRU) Set(key string, value string, duration int) {
	slru.getLRU(key).Set(key, value, duration)
}

func (slru *ShardedLRU) Delete(key string) {
	slru.getLRU(key).Delete(key)
}

func (slru *ShardedLRU) Cleanup() {
	for _, lru := range slru.shards {
		lru.Cleanup()
	}
}