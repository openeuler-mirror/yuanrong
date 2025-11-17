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

// Package crossclusterinvoke -
package crossclusterinvoke

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"
	"github.com/valyala/fasthttp"

	"huawei.com/wisesecurity/sts-sdk/pkg/stsgoapi"
	"yuanrong.org/kernel/runtime/faassdk/common/constants"
	"yuanrong.org/kernel/runtime/faassdk/sts"
	"yuanrong.org/kernel/runtime/faassdk/types"
	"yuanrong.org/kernel/runtime/faassdk/utils/signer"
	"yuanrong.org/kernel/runtime/faassdk/utils/urnutils"
	"yuanrong.org/kernel/runtime/libruntime/api"
	log "yuanrong.org/kernel/runtime/libruntime/common/faas/logger"
)

func Test_NewInvoker(t *testing.T) {
	convey.Convey("init Invoker", t, func() {
		convey.Convey("init invoker failed", func() {
			invoker := NewInvoker("")
			convey.So(invoker, convey.ShouldNotBeNil)
			convey.So(invoker.initErr, convey.ShouldNotBeNil)
		})
		convey.Convey("init invoker ok", func() {
			invoker := NewInvoker("sn:cn:yrk:12345678901234561234567890123456:function:helloworld:$latest")
			convey.So(invoker, convey.ShouldNotBeNil)
			convey.So(invoker.initErr, convey.ShouldBeNil)
		})
	})
}

func getMockInvoker() *Invoker {
	funcInfo, err := urnutils.GetFunctionInfo("sn:cn:yrk:12345678901234561234567890123456:" +
		"function:helloworld:$latest")
	return &Invoker{
		InvokeConfig: InvokeConfig{
			Enable:           true,
			CrossClusterAddr: "127.0.0.1:31222",
			ErrorCodes:       "1223,12345,345",
			ErrCodeMap: map[int]struct{}{
				1223:  {},
				12345: {},
				345:   {},
			},
			AcquireTimeout: 0,
		},
		AuthConfig: AuthConfig{
			AccessKey: "testAK",
			SecretKey: "testSK",
		},
		FuncInfo: funcInfo,
		initErr:  err,
	}
}

func Test_NeedCrossClusterInvoke(t *testing.T) {
	convey.Convey("test NeedCrossClusterInvoke", t, func() {
		convey.Convey("test NeedCrossClusterInvoke failed", func() {
			invoker := getMockInvoker()
			invoker.InvokeConfig.Enable = false
			convey.So(invoker.NeedCrossClusterInvoke(fmt.Errorf("fdafadsfda")), convey.ShouldBeFalse)
		})
		convey.Convey("test NeedCrossClusterInvoke failed1", func() {
			invoker := getMockInvoker()
			invoker.InvokeConfig.Enable = false
			convey.So(invoker.NeedCrossClusterInvoke(api.ErrorInfo{
				Code: 1111,
				Err:  errors.New(""),
			}), convey.ShouldBeFalse)
		})
		convey.Convey("test NeedCrossClusterInvoke ok", func() {
			invoker := getMockInvoker()
			invoker.InvokeConfig.Enable = true
			convey.So(invoker.NeedCrossClusterInvoke(api.ErrorInfo{
				Code: 12345,
				Err:  errors.New(""),
			}), convey.ShouldBeTrue)
		})
	})
}

func Test_buildCalleeFullFuncName(t *testing.T) {
	tt := []struct {
		name                     string
		callerFullFuncName       string
		calleeFuncName           string
		expectCalleeFullFuncName string
	}{
		{
			name:                     "test buildCalleeFullFuncName case0",
			callerFullFuncName:       "0@deafult@hello",
			calleeFuncName:           "world",
			expectCalleeFullFuncName: "0@deafult@world",
		},
		{
			name:                     "test buildCalleeFullFuncName case1",
			callerFullFuncName:       "0@service@hello",
			calleeFuncName:           "world",
			expectCalleeFullFuncName: "0@service@world",
		},
		{
			name:                     "test buildCalleeFullFuncName case2",
			callerFullFuncName:       "0@servicehello",
			calleeFuncName:           "world",
			expectCalleeFullFuncName: "0@default@world",
		},
		{
			name:                     "test buildCalleeFullFuncName case3",
			callerFullFuncName:       "1@service@hello",
			calleeFuncName:           "world",
			expectCalleeFullFuncName: "1@service@world",
		},
	}

	for _, ttt := range tt {
		convey.Convey(ttt.name, t, func() {
			actualCalleeFullFuncName := buildCalleeFullFuncName(ttt.callerFullFuncName, ttt.calleeFuncName)
			convey.So(actualCalleeFullFuncName, convey.ShouldEqual, ttt.expectCalleeFullFuncName)
		})
	}
}

func TestInvoker_DoInvoke(t *testing.T) {
	defer gomonkey.ApplyMethod(reflect.TypeOf(&stsInitOnce), "Do", func(_ *sync.Once, f func()) {
		f()
	}).Reset()
	convey.Convey("invoker init err", t, func() {
		invoker := getMockInvoker()
		invoker.initErr = fmt.Errorf("error is error")
		resp := &types.GetFutureResponse{}
		invoker.DoInvoke(types.InvokeRequest{}, resp, 0, log.GetLogger())
		convey.So(resp.StatusCode, convey.ShouldEqual, constants.FaaSError)
		convey.So(resp.ErrorMessage, convey.ShouldEqual, "crossClusterInvoker not ready, err: "+invoker.initErr.Error())
	})

	convey.Convey("timeout < 0 err", t, func() {
		invoker := getMockInvoker()
		resp := &types.GetFutureResponse{}
		invoker.DoInvoke(types.InvokeRequest{}, resp, -1, log.GetLogger())
		convey.So(resp.StatusCode, convey.ShouldEqual, constants.FaaSError)
		convey.So(resp.ErrorMessage, convey.ShouldEqual, "do cross cluster invoke failed, no time left")
	})

	convey.Convey("init sts failed", t, func() {
		defer gomonkey.ApplyFunc(sts.InitStsSDK, func(serverCfg types.StsServerConfig) error {
			return errors.New("mock sts init error")
		}).Reset()
		invoker := getMockInvoker()
		resp := &types.GetFutureResponse{}

		invoker.DoInvoke(types.InvokeRequest{}, resp, 100, log.GetLogger())

		convey.So(resp.StatusCode, convey.ShouldEqual, constants.FaaSError)
		convey.So(strings.Contains(resp.ErrorMessage, "mock sts init error"), convey.ShouldBeTrue)
	})

	convey.Convey("ak/sk is empty", t, func() {
		defer gomonkey.ApplyFunc(sts.InitStsSDK, func(serverCfg types.StsServerConfig) error {
			return nil
		}).Reset()
		defer gomonkey.ApplyFunc(stsgoapi.DecryptSensitiveConfig,
			func(rawConfigValue string) (plainBytes []byte, err error) {
				return []byte{}, errors.New("mock sts error 1111")
			}).Reset()
		invoker := getMockInvoker()
		invoker.AccessKey = ""
		invoker.SecretKey = ""
		resp := &types.GetFutureResponse{}
		invoker.DoInvoke(types.InvokeRequest{}, resp, 100, log.GetLogger())
		convey.So(resp.StatusCode, convey.ShouldEqual, constants.FaaSError)
		convey.So(strings.Contains(resp.ErrorMessage, "AK or SK is nil"), convey.ShouldBeTrue)
	})

	convey.Convey("decrypt ak/sk failed", t, func() {
		defer gomonkey.ApplyFunc(sts.InitStsSDK, func(serverCfg types.StsServerConfig) error {
			return nil
		}).Reset()
		defer gomonkey.ApplyFunc(stsgoapi.DecryptSensitiveConfig,
			func(rawConfigValue string) (plainBytes []byte, err error) {
				return []byte{}, errors.New("mock sts error")
			}).Reset()
		invoker := getMockInvoker()
		resp := &types.GetFutureResponse{}
		invoker.DoInvoke(types.InvokeRequest{}, resp, 100, log.GetLogger())
		convey.So(resp.StatusCode, convey.ShouldEqual, constants.FaaSError)
		convey.So(strings.Contains(resp.ErrorMessage, "mock sts error"), convey.ShouldBeTrue)
	})

	convey.Convey("do invoke cases", t, func() {
		invoker := getMockInvoker()
		type bodyStruct struct {
			Code         int             `json:"code"`
			Message      string          `json:"message"`
			UserResponse json.RawMessage `json:"userResponse"`
		}
		mockResponse := struct {
			response bodyStruct
			err      error
		}{}
		patches := []*gomonkey.Patches{
			gomonkey.ApplyMethod(reflect.TypeOf(&fasthttp.Client{}), "DoTimeout",
				func(_ *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response) error {
					body, _ := json.Marshal(mockResponse.response)
					resp.SetBody(body)
					return mockResponse.err
				}),
			gomonkey.ApplyFunc(sts.InitStsSDK, func(serverCfg types.StsServerConfig) error {
				return nil
			}),
			gomonkey.ApplyFunc(stsgoapi.DecryptSensitiveConfig, func(rawConfigValue string) (plainBytes []byte, err error) {
				return []byte{}, nil
			}),
		}
		defer func() {
			for _, patch := range patches {
				patch.Reset()
			}
		}()
		resp := &types.GetFutureResponse{}
		convey.Convey("do invoke ok", func() {
			mockResponse.response = bodyStruct{
				UserResponse: json.RawMessage("{\"key\":\"hello world\"}"),
				Code:         0,
				Message:      "",
			}
			mockResponse.err = nil
			invoker.DoInvoke(types.InvokeRequest{
				FuncUrn: "aaa",
			}, resp, 100*time.Second, log.GetLogger())
			convey.So(resp.StatusCode, convey.ShouldEqual, constants.NoneError)
			convey.So(resp.Content, convey.ShouldEqual, "{\"key\":\"hello world\"}")
		})
		convey.Convey("do invoke failed 0", func() {
			mockResponse.response = bodyStruct{
				UserResponse: json.RawMessage("{\"key\": \"hello world\"}"),
				Code:         0,
				Message:      "",
			}
			mockResponse.err = fmt.Errorf("error is error")
			invoker.DoInvoke(types.InvokeRequest{}, resp, 100*time.Second, log.GetLogger())
			convey.So(resp.StatusCode, convey.ShouldEqual, constants.FaaSError)
			convey.So(strings.Contains(resp.ErrorMessage, "error is error"), convey.ShouldBeTrue)
		})
		convey.Convey("do invoke failed 1", func() {
			mockResponse.response = bodyStruct{
				UserResponse: json.RawMessage("{\"key\":\"hello world\"}"),
				Code:         constants.FunctionRunError,
				Message:      "errorMsg is errorMsg",
			}
			mockResponse.err = nil
			invoker.DoInvoke(types.InvokeRequest{}, resp, 100*time.Second, log.GetLogger())
			convey.So(resp.StatusCode, convey.ShouldEqual, constants.FunctionRunError)
			convey.So(strings.Contains(resp.ErrorMessage, "errorMsg is errorMsg"), convey.ShouldBeTrue)
		})
	})
}

func TestNeedTryLocalCluster(t *testing.T) {
	convey.Convey(
		"Test needTryLocalCluster", t, func() {
			convey.Convey("needTryLocalCluster success", func() {
				err := errors.New("")
				resp := fasthttp.AcquireResponse()
				flag := needTryLocalCluster(err, resp, nil)
				convey.So(flag, convey.ShouldBeFalse)
				flag = needTryLocalCluster(nil, nil, nil)
				convey.So(flag, convey.ShouldBeFalse)
				flag = needTryLocalCluster(nil, resp, nil)
				convey.So(flag, convey.ShouldBeFalse)
			})
		},
	)
}

func TestHandleHttpResponse(t *testing.T) {
	convey.Convey(
		"Test handleHttpResponse", t, func() {
			convey.Convey("handleHttpResponse success", func() {
				resp := fasthttp.AcquireResponse()
				convey.So(func() {
					handleHttpResponse(resp, &types.GetFutureResponse{})
				}, convey.ShouldNotPanic)
			})
		},
	)
}

func Test_getCrossClusterInvokeConfig(t *testing.T) {
	convey.Convey("getCrossClusterInvokeConfig", t, func() {
		defer gomonkey.ApplyFunc(os.ReadFile, func(name string) ([]byte, error) {
			return []byte(`{"crossClusterInvokeConfig": {"crossClusterAddr": "127.0.0.1:8080", "errorCodes": "150420,150421"}}`), nil
		}).Reset()
		config := getCrossClusterInvokeConfig()
		_, exist := config.ErrCodeMap[150420]
		convey.So(exist, convey.ShouldBeTrue)
	})
}

func Test_getCrossClusterAuthConfig(t *testing.T) {
	convey.Convey("getCrossClusterInvokeConfig", t, func() {
		defer gomonkey.ApplyFunc(os.Getenv, func(key string) string {
			return `{"accessKey":"testAk","secretKey":"testSK"}`
		}).Reset()
		config := getCrossClusterAuthConfig()
		convey.So(config.AccessKey, convey.ShouldEqual, "testAk")
		convey.So(config.SecretKey, convey.ShouldEqual, "testSK")
	})
}

func Test_SetHeader(t *testing.T) {
	convey.Convey("Test SetHeader", t, func() {
		patches := []*gomonkey.Patches{
			gomonkey.ApplyFunc(sts.InitStsSDK, func(serverCfg types.StsServerConfig) error {
				return nil
			}),
			gomonkey.ApplyFunc(stsgoapi.DecryptSensitiveConfig,
				func(rawConfigValue string) (plainBytes []byte, err error) {
					return []byte(base64.StdEncoding.EncodeToString([]byte("aaa"))), nil
				}),
		}
		defer func() {
			for _, patch := range patches {
				patch.Reset()
			}
		}()
		invoker := getMockInvoker()
		httpReq := &fasthttp.Request{}
		req := types.InvokeRequest{
			Payload: "hello",
		}
		httpReq.SetBody([]byte(req.Payload))
		err := invoker.setHeader(httpReq, req, log.GetLogger())
		timeStamp := "1736864093"
		signature := buildSignature(timeStamp, []byte(req.Payload), invoker.AccessKey)
		sign := signer.Sign([]byte(invoker.SecretKey), signature)
		signStr := signer.EncodeHex(sign)
		buildAuth := signer.BuildAuthorization(invoker.AccessKey, timeStamp, signStr)
		convey.So(err, convey.ShouldBeNil)
		convey.So(buildAuth, convey.ShouldEqual, "SDK-HMAC-SHA256 accessId=testAK,timestamp="+timeStamp+",signature=0b3558036eaf229573c9390016e2adaaa4b39403bde85e6f075bb0b224658b7f")
	})
}
