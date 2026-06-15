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

package api

import (
	"errors"
	"fmt"
	"testing"
)

func TestRouteUpdateErrorRequiresRetryableNonEmptyRoute(t *testing.T) {
	if _, ok := AsRouteUpdateError(NewRouteUpdateError("inst", "", "proxy")); ok {
		t.Fatal("empty route should not be retryable")
	}
	nonRetryable := &RouteUpdateError{InstanceID: "inst", RouteAddress: "route", ProxyID: "proxy", Retryable: false}
	if _, ok := AsRouteUpdateError(nonRetryable); ok {
		t.Fatal("retryable=false hint should not be retryable")
	}
	if hint, ok := AsRouteUpdateError(NewRouteUpdateError("inst", "route", "proxy")); !ok ||
		hint.InstanceID != "inst" || hint.RouteAddress != "route" || hint.ProxyID != "proxy" {
		t.Fatalf("hint=%+v ok=%v", hint, ok)
	}
}

func TestRouteUpdateErrorWrapsAndUnwraps(t *testing.T) {
	base := errors.New("base")
	err := NewRouteUpdateError("inst", "route", "proxy", base)
	if !errors.Is(err, base) {
		t.Fatal("expected wrapped base error")
	}
	var hint *RouteUpdateError
	if !errors.As(fmt.Errorf("wrap: %w", err), &hint) {
		t.Fatal("expected errors.As to find RouteUpdateError")
	}
	if hint.Unwrap() != base {
		t.Fatalf("unwrap=%v", hint.Unwrap())
	}
}

func TestRouteUpdateErrorNilWrappedError(t *testing.T) {
	err := NewRouteUpdateError("inst", "route", "proxy")
	if err == nil {
		t.Fatal("expected error")
	}
	var hint *RouteUpdateError
	if !errors.As(err, &hint) || hint == nil {
		t.Fatal("expected route update error")
	}
	if hint.Unwrap() != nil {
		t.Fatalf("unwrap=%v", hint.Unwrap())
	}
}

func TestAsRouteUpdateErrorTypedNilSafe(t *testing.T) {
	var typedNil *RouteUpdateError
	var err error = typedNil
	if _, ok := AsRouteUpdateError(err); ok {
		t.Fatal("typed nil should not be accepted")
	}
}
