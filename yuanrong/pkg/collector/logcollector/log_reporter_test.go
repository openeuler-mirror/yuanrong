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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"

	"yuanrong/pkg/collector/common"
	"yuanrong/pkg/common/faas_common/grpc/pb/logservice"
)

func TestGetRuntimeID(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"runtime-12345.out", "runtime-12345"},
		{"runtime-123e4567-e89b-12d3-a456-426614174000.err", "runtime-123e4567-e89b-12d3-a456-426614174000"},
		{"no-runtime.log", ""},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := getRuntimeID(tt.filename)
			if got != tt.want {
				t.Errorf("getRuntimeID(%v) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestParseLogFileName(t *testing.T) {
	tests := []struct {
		filePath string
		want     *logservice.LogItem
		wantOk   bool
	}{
		{
			filePath: "path/to/runtime-abc123-456.out",
			want: &logservice.LogItem{
				Filename:    "path/to/runtime-abc123-456.out",
				CollectorID: common.CollectorConfigs.CollectorID,
				Target:      logservice.LogTarget_USER_STD,
				RuntimeID:   "runtime-abc123-456",
			},
			wantOk: true,
		},
		{
			filePath: "/path/to/runtime-1c1.err",
			want: &logservice.LogItem{
				Filename:    "/path/to/runtime-1c1.err",
				CollectorID: common.CollectorConfigs.CollectorID,
				Target:      logservice.LogTarget_USER_STD,
				RuntimeID:   "runtime-1c1",
			},
			wantOk: true,
		},
		{
			filePath: "path/to/unknown.log",
			want:     nil,
			wantOk:   false,
		},
		{
			filePath: "/path/to/function-master.log",
			want:     nil,
			wantOk:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			got, ok := parseLogFileName(tt.filePath)
			assert.Equal(t, tt.wantOk, ok)
			if ok {
				assert.Equal(t, tt.want.Filename, got.Filename)
				assert.Equal(t, tt.want.CollectorID, got.CollectorID)
				assert.Equal(t, tt.want.Target, got.Target)
				assert.Equal(t, tt.want.RuntimeID, got.RuntimeID)
			}
		})
	}
}

type MockLogServiceClient struct {
	mock.Mock
}

func (m *MockLogServiceClient) ReportLog(ctx context.Context, req *logservice.ReportLogRequest, opts ...grpc.CallOption) (*logservice.ReportLogResponse, error) {
	args := m.Called(ctx, req, opts)
	return args.Get(0).(*logservice.ReportLogResponse), args.Error(1)
}

func (m *MockLogServiceClient) Register(ctx context.Context, req *logservice.RegisterRequest, opts ...grpc.CallOption) (*logservice.RegisterResponse, error) {
	args := m.Called(ctx, req, opts)
	return args.Get(0).(*logservice.RegisterResponse), args.Error(1)
}

func TestReportLog(t *testing.T) {
	constant = &constTestImpl{}
	tests := []struct {
		mockResponse  *logservice.ReportLogResponse
		mockError     error
		expectedError error
	}{
		{
			mockResponse:  &logservice.ReportLogResponse{Code: 0, Message: "success"},
			mockError:     nil,
			expectedError: nil,
		},
		{
			mockResponse:  nil,
			mockError:     fmt.Errorf("network error"),
			expectedError: fmt.Errorf("failed to report log: exceeds max retry time: %d", constant.GetMaxRetryTimes()),
		},
		{
			mockResponse:  &logservice.ReportLogResponse{Code: -1, Message: "failure"},
			mockError:     nil,
			expectedError: fmt.Errorf("failed to report log: exceeds max retry time: %d", constant.GetMaxRetryTimes()),
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			mockClient := new(MockLogServiceClient)
			LogServiceClient = mockClient
			mockClient.On("ReportLog", mock.Anything, mock.Anything, mock.Anything).Return(tt.mockResponse, tt.mockError)
			err := reportLog(&logservice.LogItem{})
			mockClient.AssertExpectations(t)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestFailedReportLog(t *testing.T) {
	LogServiceClient = nil

	tests := []struct {
		mockResponse  *logservice.ReportLogResponse
		mockError     error
		expectedError error
	}{
		{
			mockResponse:  nil,
			mockError:     nil,
			expectedError: fmt.Errorf("failed to get log service client"),
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			item := &logservice.LogItem{}
			err := reportLog(item)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestTryReportLog(t *testing.T) {
	constant = &constTestImpl{}
	reportedLogFiles.Lock()
	reportedLogFiles.hashmap["runtime-123.out"] = struct{}{}
	reportedLogFiles.Unlock()
	tests := []struct {
		name         string
		mockResponse *logservice.ReportLogResponse
		mockError    error
		result       bool
	}{
		{
			name:         "runtime-123e4567-e89b-12d3-a456-426614174000.err",
			mockResponse: &logservice.ReportLogResponse{Code: 0, Message: "success"},
			mockError:    nil,
			result:       true,
		},
		{
			name:         "runtime-456.out",
			mockResponse: &logservice.ReportLogResponse{Code: -1, Message: "failure"},
			mockError:    nil,
			result:       false,
		},
		{
			name:         "runtime-123.out",
			mockResponse: &logservice.ReportLogResponse{Code: 0, Message: "success"},
			mockError:    nil,
			result:       false,
		},
		{
			name:         "function-master.out",
			mockResponse: &logservice.ReportLogResponse{Code: 0, Message: "success"},
			mockError:    nil,
			result:       false,
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			mockClient := new(MockLogServiceClient)
			LogServiceClient = mockClient
			mockClient.On("ReportLog", mock.Anything, mock.Anything, mock.Anything).Return(tt.mockResponse, tt.mockError)
			res := tryReportLog(tt.name)
			assert.Equal(t, tt.result, res)
		})
	}
}

func prepareDir(t *testing.T) string {
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "subdir")
	os.Mkdir(subDir, 0755)
	return subDir
}

func prepareFiles(subDir string) {
	reportedLogFiles.Lock()
	reportedLogFiles.hashmap = make(map[string]struct{})
	reportedLogFiles.Unlock()
	file1 := filepath.Join(subDir, "runtime-456-abc.err")
	file2 := filepath.Join(subDir, "runtime-abc.out")
	os.WriteFile(file1, []byte("log content"), 0644)
	os.WriteFile(file2, []byte("log content"), 0644)
}

func prepareClient() *MockLogServiceClient {
	mockClient := new(MockLogServiceClient)
	LogServiceClient = mockClient
	mockResponse := &logservice.ReportLogResponse{Code: 0, Message: "success"}
	mockClient.On("ReportLog", mock.Anything, mock.Anything, mock.Anything).Return(mockResponse, nil)
	return mockClient
}

func checkReportMsg(t *testing.T, mockClient *MockLogServiceClient) {
	{
		r := mockClient.Calls[0].Arguments[1].(*logservice.ReportLogRequest)
		assert.Equal(t, len(r.Items), 1)
		assert.Equal(t, r.Items[0].Filename, "runtime-456-abc.err")
		assert.Equal(t, r.Items[0].CollectorID, common.CollectorConfigs.CollectorID)
		assert.Equal(t, r.Items[0].Target, logservice.LogTarget_USER_STD)
		assert.Equal(t, r.Items[0].RuntimeID, "runtime-456-abc")
	}
	{
		r := mockClient.Calls[1].Arguments[1].(*logservice.ReportLogRequest)
		assert.Equal(t, len(r.Items), 1)
		assert.Equal(t, r.Items[0].Filename, "runtime-abc.out")
		assert.Equal(t, r.Items[0].CollectorID, common.CollectorConfigs.CollectorID)
		assert.Equal(t, r.Items[0].Target, logservice.LogTarget_USER_STD)
		assert.Equal(t, r.Items[0].RuntimeID, "runtime-abc")
	}

	mockClient.AssertExpectations(t)
}

func TestCreateLogReporter(t *testing.T) {
	mockClient := prepareClient()
	subDir := prepareDir(t)
	createLogReporter(subDir)
	time.Sleep(10 * time.Millisecond)
	prepareFiles(subDir)
	time.Sleep(10 * time.Millisecond)
	checkReportMsg(t, mockClient)
}

func TestScanUserLog(t *testing.T) {
	mockClient := prepareClient()
	subDir := prepareDir(t)
	prepareFiles(subDir)
	scanUserLog(subDir)
	checkReportMsg(t, mockClient)
}

func TestStartLogReporter(t *testing.T) {
	mockClient := prepareClient()
	subDir := prepareDir(t)
	prepareFiles(subDir)
	common.CollectorConfigs.UserLogPath = subDir
	StartLogReporter()
	checkReportMsg(t, mockClient)
}
