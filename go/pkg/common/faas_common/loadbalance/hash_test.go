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

// Package loadbalance provides consistent hash algorithm
package loadbalance

import (
	"fmt"
	"testing"
	"time"

	"github.com/smartystreets/goconvey/convey"
)

func TestSimpleCHGeneric_Next(t *testing.T) {
	convey.Convey("SimpleCHGeneric next", t, func() {
		generic := NewSimpleCHGeneric()

		generic.Add("node1", 0)
		generic.Add("node2", 0)
		next1 := generic.Next("function1", false)
		next2 := generic.Next("function1", false)
		convey.So(next1, convey.ShouldResemble, next2)
	})
}

func TestSimpleCHGenericVirtualNodes(t *testing.T) {
	nodes := []string{"scheduler-alpha", "scheduler-beta", "scheduler-gamma"}
	// ideal share per node is 1/3; a single-point ring skews beyond 2x
	// regularly, virtual nodes should stay within [minShare, maxShare]
	const (
		minShare = 0.25
		maxShare = 0.42
	)

	convey.Convey("empty ring returns empty owner", t, func() {
		generic := NewSimpleCHGeneric()
		convey.So(generic.Next("any-key", false), convey.ShouldResemble, "")
	})

	convey.Convey("different add orders produce identical routing", t, func() {
		forward := NewSimpleCHGeneric()
		reverse := NewSimpleCHGeneric()
		for _, n := range nodes {
			forward.Add(n, 0)
		}
		for i := len(nodes) - 1; i >= 0; i-- {
			reverse.Add(nodes[i], 0)
		}
		for i := 0; i < 2000; i++ {
			key := fmt.Sprintf("session-key-%d", i)
			convey.So(forward.Next(key, false), convey.ShouldResemble, reverse.Next(key, false))
		}
	})

	convey.Convey("duplicate add is idempotent", t, func() {
		generic := NewSimpleCHGeneric()
		generic.Add(nodes[0], 0)
		generic.Add(nodes[0], 0)
		convey.So(len(generic.nodes), convey.ShouldEqual, virtualNodeCount)
		convey.So(len(generic.owners), convey.ShouldEqual, 1)
	})

	convey.Convey("remove drops all virtual points of the node", t, func() {
		generic := NewSimpleCHGeneric()
		for _, n := range nodes {
			generic.Add(n, 0)
		}
		generic.Remove("scheduler-beta")
		convey.So(len(generic.nodes), convey.ShouldEqual, (len(nodes)-1)*virtualNodeCount)
		for i := 0; i < 2000; i++ {
			owner := generic.Next(fmt.Sprintf("session-key-%d", i), false)
			convey.So(owner, convey.ShouldNotEqual, "scheduler-beta")
		}
		generic.RemoveAll()
		convey.So(generic.Next("session-key-0", false), convey.ShouldResemble, "")
		convey.So(len(generic.nodes), convey.ShouldEqual, 0)
	})

	convey.Convey("virtual nodes spread traffic near-evenly", t, func() {
		generic := NewSimpleCHGeneric()
		for _, n := range nodes {
			generic.Add(n, 0)
		}
		const total = 100000
		counts := make(map[string]int, len(nodes))
		for i := 0; i < total; i++ {
			owner, ok := generic.Next(fmt.Sprintf("session-%d", i), false).(string)
			if !ok {
				t.Fatalf("Next returned a non-string owner for session-%d", i)
			}
			counts[owner]++
		}
		convey.So(len(counts), convey.ShouldEqual, len(nodes))
		for _, n := range nodes {
			share := float64(counts[n]) / float64(total)
			convey.So(share, convey.ShouldBeGreaterThan, minShare)
			convey.So(share, convey.ShouldBeLessThan, maxShare)
		}
	})
}

func TestCHGeneric_Previous(t *testing.T) {
	convey.Convey("CHGeneric previous", t, func() {
		generic := NewCHGeneric()
		generic.Add("node1", 0)
		generic.Add("node2", 0)
		generic.Add("node3", 0)

		previous := generic.Previous("node2", false)
		convey.So(previous, convey.ShouldEqual, "node1")

		previous = generic.Previous("node2", true)
		convey.So(previous, convey.ShouldEqual, "node3")
	})
}

func TestLimiterCHGeneric_DeleteBalancer(t *testing.T) {
	convey.Convey("LimiterCHGeneric_DeleteBalancer", t, func() {
		generic := NewLimiterCHGeneric(1 * time.Second)
		generic.Add("node1", 0)
		generic.Add("node2", 0)
		generic.Add("node3", 0)

		next1 := generic.Next("function1", false)
		convey.So(next1, convey.ShouldEqual, "node2")
		next2 := generic.Next("function2", false)
		convey.So(next2, convey.ShouldEqual, "node3")

		_, ok := generic.limiter["function1"]
		_, exist := generic.anchorPoint["function1"]
		convey.So(ok, convey.ShouldBeTrue)
		convey.So(exist, convey.ShouldBeTrue)

		generic.DeleteBalancer("function1")
		_, ok = generic.limiter["function1"]
		_, exist = generic.anchorPoint["function1"]
		convey.So(ok, convey.ShouldBeFalse)
		convey.So(exist, convey.ShouldBeFalse)
	})
}
