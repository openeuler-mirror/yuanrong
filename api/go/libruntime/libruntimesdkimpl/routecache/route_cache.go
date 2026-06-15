/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2025. All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package routecache provides a bounded local route cache for direct routing.
package routecache

import (
	"container/list"
	"os"
	"strconv"
	"sync"
)

// DefaultCapacity is the default maximum number of cached local routes.
const DefaultCapacity = 1024

// CapacityEnv is the optional test/deploy override for direct-route cache size.
const CapacityEnv = "YR_DIRECT_ROUTE_CACHE_CAPACITY"

// Entry is the cached routing data for an allocated instance.
type Entry struct {
	RouteAddress string
	ProxyID      string
}

type cacheEntry struct {
	instanceID string
	entry      Entry
}

// Cache is a mutex-protected LRU cache keyed by instance ID.
type Cache struct {
	mu       sync.Mutex
	capacity int
	entries  map[string]*list.Element
	lru      *list.List
}

// CapacityFromEnv returns the configured route-cache capacity, falling back to DefaultCapacity.
func CapacityFromEnv() int {
	value := os.Getenv(CapacityEnv)
	if value == "" {
		return DefaultCapacity
	}
	capacity, err := strconv.Atoi(value)
	if err != nil || capacity <= 0 {
		return DefaultCapacity
	}
	return capacity
}

// New creates a route cache with the given capacity. Non-positive capacities use DefaultCapacity.
func New(capacity int) *Cache {
	if capacity <= 0 {
		capacity = DefaultCapacity
	}
	return &Cache{
		capacity: capacity,
		entries:  make(map[string]*list.Element),
		lru:      list.New(),
	}
}

// Put stores or refreshes an instance route. Empty instance IDs or route addresses are not cached.
func (c *Cache) Put(instanceID string, entry Entry) {
	if c == nil || instanceID == "" || entry.RouteAddress == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.entries[instanceID]; ok {
		elem.Value.(*cacheEntry).entry = entry
		c.lru.MoveToFront(elem)
		return
	}

	elem := c.lru.PushFront(&cacheEntry{instanceID: instanceID, entry: entry})
	c.entries[instanceID] = elem
	if len(c.entries) > c.capacity {
		c.removeOldestLocked()
	}
}

// Get returns the cached route and refreshes its recency when present.
func (c *Cache) Get(instanceID string) (Entry, bool) {
	if c == nil || instanceID == "" {
		return Entry{}, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.entries[instanceID]
	if !ok {
		return Entry{}, false
	}
	c.lru.MoveToFront(elem)
	return elem.Value.(*cacheEntry).entry, true
}

// Remove deletes a cached route.
func (c *Cache) Remove(instanceID string) {
	if c == nil || instanceID == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.entries[instanceID]; ok {
		c.removeElementLocked(elem)
	}
}

// Len returns the current number of cached routes.
func (c *Cache) Len() int {
	if c == nil {
		return 0
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

func (c *Cache) removeOldestLocked() {
	if elem := c.lru.Back(); elem != nil {
		c.removeElementLocked(elem)
	}
}

func (c *Cache) removeElementLocked(elem *list.Element) {
	c.lru.Remove(elem)
	delete(c.entries, elem.Value.(*cacheEntry).instanceID)
}
