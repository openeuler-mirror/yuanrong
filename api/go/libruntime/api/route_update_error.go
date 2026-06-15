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
)

// RouteUpdateError carries a structured stale-route repair hint.
type RouteUpdateError struct {
	InstanceID   string
	RouteAddress string
	ProxyID      string
	Retryable    bool
	Reason       string
	ModRevision  int64
	Err          error
}

func (e *RouteUpdateError) Error() string {
	if e == nil {
		return "route update required"
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	if e.InstanceID != "" {
		return fmt.Sprintf("route update required for instance %s", e.InstanceID)
	}
	return "route update required"
}

func (e *RouteUpdateError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// NewRouteUpdateError returns a structured route-update hint error.
func NewRouteUpdateError(instanceID, routeAddress, proxyID string, err ...error) error {
	var wrapped error
	if len(err) > 0 {
		wrapped = err[0]
	}
	return &RouteUpdateError{
		InstanceID:   instanceID,
		RouteAddress: routeAddress,
		ProxyID:      proxyID,
		Retryable:    true,
		Err:          wrapped,
	}
}

// AsRouteUpdateError returns a route-update error only when it carries a non-empty route address.
func AsRouteUpdateError(err error) (*RouteUpdateError, bool) {
	var out *RouteUpdateError
	if errors.As(err, &out) && out != nil && out.RouteAddress != "" && out.Retryable {
		return out, true
	}
	return nil, false
}
