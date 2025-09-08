package server

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

// single entry of cache
type Entry struct {
	value      string
	expiryTime time.Time
	index int
	accessTime int
}

// Approximate LRU Cache -------------------------------------------------------------------------
const SAMPLE_SIZE = 32
const WORKER_INTERVALS = 5 * time.Second

type LRUCache struct {
	cache map[string] Entry
	capacity int
	keys []string
	clock int
	stopJob chan struct{}
	rng *rand.Rand
	mutex sync.Mutex
}

func NewLRUCache(capacity int) *LRUCache {
	if capacity <= 0 {
		panic("LRU Cache capacity must be greater than 0.")
	}

	lru := LRUCache{
		cache: make(map[string]Entry), 
		capacity: capacity, 
		keys: []string{}, 
		clock: 0,
		stopJob: make(chan struct{}),
		rng: rand.New(rand.NewSource(time.Now().UnixNano())), // TODO: maybe use a seed?
	}

	lru.startJanitor()

	return &lru
}

// private methods -------------
func (lru *LRUCache) deleteEntry(key string) {
	entry, exists := lru.cache[key]
	if exists {
		delete(lru.cache, key)
		
		// swap key index with last index and pop
		lastInd := len(lru.keys) - 1
		lru.keys[entry.index], lru.keys[lastInd] = lru.keys[lastInd], lru.keys[entry.index]
		lru.keys = lru.keys[:lastInd]

		// update swapped keys entry
		swappedKey := lru.keys[entry.index]
		swappedEntry := lru.cache[swappedKey]
		swappedEntry.index = entry.index
		lru.cache[swappedKey] = swappedEntry
	}
}

func (lru *LRUCache) getClock() int {
	// each clock access increments clock - mod clock by int max
	if lru.clock == math.MaxInt {
		lru.clock = 0
	} else {
		lru.clock += 1
	}
	return lru.clock
}

func (lru *LRUCache) sampleEviction() {
	// this will AT LEAST do one deletion if over capacity, but won't guarantee anymore
	// randomly sample up to SAMPLE_SIZE keys, drop expired ones, keep track of oldest access time key
	cacheSize := len(lru.keys)
	oldest := ""
	oldestTime := lru.clock

	for i := 0; i < min(cacheSize, SAMPLE_SIZE); i++ {
		// draw
		key := lru.keys[lru.rng.Intn(len(lru.keys))]
		entry, exists := lru.cache[key]

		if !exists {
			panic("messed up sample eviction logic somewhere.")
		}

		// if expired, kick
		if !entry.expiryTime.IsZero() && !time.Now().Before(entry.expiryTime) {
			lru.deleteEntry(key)
			continue
		}
		
		// if older than all not kicked, set - maybe bad to do here to actually keep approximate LRU (idk)
		if (lru.clock - entry.accessTime)%math.MaxInt > (lru.clock - oldestTime)%math.MaxInt {
			oldest, oldestTime = key, entry.accessTime
		}
	}

	if len(lru.keys) > lru.capacity {
		lru.deleteEntry(oldest)
	}
}

func (lru *LRUCache) startJanitor() {
	// start background worker
    t := time.NewTicker(WORKER_INTERVALS)
    go func() {
        defer t.Stop()
        for {
            select {
            case <-t.C:
				lru.mutex.Lock()
				for len(lru.keys) > lru.capacity {
					lru.sampleEviction()
				}
				lru.mutex.Unlock()
            case <-lru.stopJob:
                return
            }
        }
    }()
}

// public methods -------------
func (lru *LRUCache) Get(key string) (string, bool) {
	// set lock
	lru.mutex.Lock()
	defer lru.mutex.Unlock()

	// retrieve entry
	entry, exists := lru.cache[key]
	if exists {
		if entry.expiryTime.IsZero() || time.Now().Before(entry.expiryTime) {
			// make more recent
			entry.accessTime = lru.getClock()
			lru.cache[key] = entry

			return entry.value, true
		} else {
			lru.deleteEntry(key)
			return "", false
		}
	}
	return "", false
}

func (lru *LRUCache) Set(key string, value string, duration int) {
	// set lock
	lru.mutex.Lock()
	defer lru.mutex.Unlock()

	entry, exists := lru.cache[key]

	// create new/updated entry - if duration negative, count as infinite
	// set index later (depends on if exists or not)
	var newEntry Entry
	if duration >= 0 {
		newEntry = Entry{
			value: value,
			expiryTime: time.Now().Add(time.Duration(duration) * time.Millisecond),
			accessTime: lru.getClock(),			
		}
	} else {
		newEntry = Entry{
			value: value,
			expiryTime: time.Time{},
			accessTime: lru.getClock(),
		}
	}

	if exists {
		// just update entry
		newEntry.index = entry.index
		lru.cache[key] = newEntry
	} else {
		// set entry index, add key to keys, add entry
		newEntry.index = len(lru.keys)
		lru.keys = append(lru.keys, key)
		lru.cache[key] = newEntry

		// if exceeding capacity, perform sample removal
		if len(lru.keys) > lru.capacity {
			lru.sampleEviction()
		}
	}
}

func (lru *LRUCache) Delete(key string) {
	// set lock
	lru.mutex.Lock()
	defer lru.mutex.Unlock()

	// delete key
	lru.deleteEntry(key)
}

func (lru *LRUCache) Cleanup() {
	close(lru.stopJob)
}