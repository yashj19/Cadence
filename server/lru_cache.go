package server

import (
	"sync"
	"time"
)

// single entry of cache
type Entry struct {
	value      string
	expiryTime time.Time
}

// node struct for linked list
type Node struct {
	key string
	entry Entry
	next *Node
	prev *Node
}

// LRU Cache -------------------------------------------------------------------------
type LRUCache struct {
	cache map[string] *Node
	capacity int
	head *Node
	tail *Node
	cacheMutex sync.RWMutex
	listMutex sync.Mutex
}

func NewLRUCache(capacity int) LRUCache {
	if capacity <= 0 {
		panic("LRU Cache capacity must be greater than 0.")
	}

	// init linked list ends (head followed by tail)
	head := &Node{}
	tail := &Node{"", Entry{}, nil, head}
	head.next = tail

	return LRUCache{cache: make(map[string]*Node), capacity: capacity, head: head, tail: tail}
}

func (lru *LRUCache) removeNode(node *Node) {
	// locking list
	lru.listMutex.Lock()
	defer lru.listMutex.Unlock()

	next, prev := node.next, node.prev

	// cannot operate on head or tail
	if next == nil || prev == nil {
		panic("cannot remove head or tail on LRU Cache linked list.")
	}

	// set prev's next to next and next's prev to prev
	prev.next = next
	next.prev = prev

	node.next = nil
	node.prev = nil
}

func (lru *LRUCache) appendTail(node *Node) {
	// locking list
	lru.listMutex.Lock()
	defer lru.listMutex.Unlock()

	prev := lru.tail.prev

	// add node
	lru.tail.prev = node
	prev.next = node

	// fix node
	node.next = lru.tail
	node.prev = prev
}

func (lru *LRUCache) Get(key string) (string, bool) {
	// read lock cache
	lru.cacheMutex.RLock()
	
	// fetch node
	node, exists := lru.cache[key]
	if exists {
		if node.entry.expiryTime.IsZero() || time.Now().Before(node.entry.expiryTime) {
			// move this node to end of the list, read the val
			lru.removeNode(node)
			lru.appendTail(node)
			returnVal := node.entry.value

			// remove lock
			lru.cacheMutex.RUnlock()

			return returnVal, true
		} else {
			// remove the lock
			lru.cacheMutex.RUnlock()

			// lazy deletion - remove it
			lru.removeNode(node)

			// write lock cache during deletion
			lru.cacheMutex.Lock()
			delete(lru.cache, key)
			lru.cacheMutex.Unlock()

			return "", false
		}
	}
	return "", false
}

func (lru *LRUCache) Set(key string, value string, duration int) {
	// lock cache for read
	lru.cacheMutex.RLock()
	node, exists := lru.cache[key]
	lru.cacheMutex.RUnlock()

	// create new/updated entry - if duration negative, count as infinite
	var newEntry Entry
	if duration >= 0 {
		newEntry = Entry{value: value, expiryTime: time.Now().Add(time.Duration(duration) * time.Millisecond)}
	} else {
		newEntry = Entry{value: value, expiryTime: time.Time{}}
	}

	if exists {
		// move to end and update entry
		lru.removeNode(node)
		lru.appendTail(node)

		lru.cacheMutex.Lock()
		node.entry = newEntry
		lru.cacheMutex.Unlock()
	} else {
		// create new node, add to cache and list
		node = &Node{key: key, entry: newEntry}

		lru.cacheMutex.Lock()
		lru.cache[key] = node
		lru.capacity += 1
		lru.cacheMutex.Unlock()

		lru.appendTail(node)

		// if exceeding capacity, remove head
		lru.cacheMutex.Lock()
		if len(lru.cache) > lru.capacity {
			delete(lru.cache, lru.head.next.key)
			lru.removeNode(lru.head.next)
			lru.capacity -= 1
		}
		lru.cacheMutex.Unlock()
	}
}