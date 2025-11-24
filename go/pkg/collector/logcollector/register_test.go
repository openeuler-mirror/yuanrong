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

package logcollector

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"yuanrong.org/kernel/pkg/common/faas_common/grpc/pb/logservice"
)

func TestRegister(t *testing.T) {
	constant = &constTestImpl{}
	tests := []struct {
		mockResponse  *logservice.RegisterResponse
		mockError     error
		expectedError error
	}{
		{
			mockResponse:  &logservice.RegisterResponse{Code: 0, Message: "success"},
			mockError:     nil,
			expectedError: nil,
		},
		{
			mockResponse:  &logservice.RegisterResponse{Code: -1, Message: "failure"},
			mockError:     nil,
			expectedError: fmt.Errorf("failed to register: exceeds max retry time: %d", constant.GetMaxRetryTimes()),
		},
		{
			mockResponse:  &logservice.RegisterResponse{Code: -1, Message: "failure"},
			mockError:     fmt.Errorf("failed"),
			expectedError: fmt.Errorf("failed to register: exceeds max retry time: %d", constant.GetMaxRetryTimes()),
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			mockClient := new(MockLogServiceClient)
			LogServiceClient = mockClient
			mockClient.On("Register", mock.Anything, mock.Anything, mock.Anything).Return(tt.mockResponse, tt.mockError)
			err := Register()
			mockClient.AssertExpectations(t)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestFailedClientRegister(t *testing.T) {
	LogServiceClient = nil
	err := Register()
	expectedError := fmt.Errorf("failed to get log service client")
	assert.Equal(t, expectedError, err)
}
