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

package routecache

import (
	"fmt"
	"sync"
	"testing"
)

func TestCacheEvictsLeastRecentlyUsedEntry(t *testing.T) {
	cache := New(2)
	cache.Put("instance-a", Entry{RouteAddress: "10.0.0.1:7788", ProxyID: "proxy-a"})
	cache.Put("instance-b", Entry{RouteAddress: "10.0.0.2:7788", ProxyID: "proxy-b"})

	if _, ok := cache.Get("instance-a"); !ok {
		t.Fatalf("expected instance-a to be cached")
	}
	cache.Put("instance-c", Entry{RouteAddress: "10.0.0.3:7788", ProxyID: "proxy-c"})

	if _, ok := cache.Get("instance-b"); ok {
		t.Fatalf("expected least recently used instance-b to be evicted")
	}
	entry, ok := cache.Get("instance-a")
	if !ok {
		t.Fatalf("expected refreshed instance-a to remain cached")
	}
	if entry.RouteAddress != "10.0.0.1:7788" || entry.ProxyID != "proxy-a" {
		t.Fatalf("unexpected refreshed entry: %+v", entry)
	}
	if _, ok := cache.Get("instance-c"); !ok {
		t.Fatalf("expected newest instance-c to remain cached")
	}
}

func TestCacheSkipsEmptyRoutesAndRemoveWorks(t *testing.T) {
	cache := New(2)
	cache.Put("instance-empty", Entry{RouteAddress: "", ProxyID: "proxy-empty"})
	if cache.Len() != 0 {
		t.Fatalf("expected empty route not to be cached, got len %d", cache.Len())
	}
	if _, ok := cache.Get("instance-empty"); ok {
		t.Fatalf("expected empty route entry to be absent")
	}

	cache.Put("instance-a", Entry{RouteAddress: "10.0.0.1:7788", ProxyID: "proxy-a"})
	cache.Put("instance-a", Entry{RouteAddress: "", ProxyID: "proxy-empty"})
	entry, ok := cache.Get("instance-a")
	if !ok {
		t.Fatalf("expected empty route update to leave existing entry cached")
	}
	if entry.RouteAddress != "10.0.0.1:7788" || entry.ProxyID != "proxy-a" {
		t.Fatalf("expected empty route update not to replace existing entry, got %+v", entry)
	}

	cache.Remove("instance-a")
	if cache.Len() != 0 {
		t.Fatalf("expected remove to drop entry, got len %d", cache.Len())
	}
	if _, ok := cache.Get("instance-a"); ok {
		t.Fatalf("expected removed entry to be absent")
	}
}

func TestConcurrentPutGetDoesNotExceedCapacity(t *testing.T) {
	const capacity = 64
	const goroutines = 16
	const iterations = 500
	cache := New(capacity)

	var wg sync.WaitGroup
	for worker := 0; worker < goroutines; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				key := fmt.Sprintf("instance-%d-%d", worker, i)
				cache.Put(key, Entry{RouteAddress: fmt.Sprintf("10.0.%d.%d:7788", worker, i%255), ProxyID: "proxy"})
				cache.Get(key)
				cache.Get(fmt.Sprintf("instance-%d-%d", worker, i/2))
			}
		}(worker)
	}
	wg.Wait()

	if got := cache.Len(); got > capacity {
		t.Fatalf("cache len %d exceeds capacity %d", got, capacity)
	}
}

func TestCapacityFromEnv(t *testing.T) {
	t.Setenv(CapacityEnv, "")
	if got := CapacityFromEnv(); got != DefaultCapacity {
		t.Fatalf("expected default capacity for empty env, got %d", got)
	}

	t.Setenv(CapacityEnv, "4")
	if got := CapacityFromEnv(); got != 4 {
		t.Fatalf("expected configured capacity 4, got %d", got)
	}

	for _, value := range []string{"0", "-1", "invalid"} {
		t.Setenv(CapacityEnv, value)
		if got := CapacityFromEnv(); got != DefaultCapacity {
			t.Fatalf("expected default capacity for %q, got %d", value, got)
		}
	}
}
