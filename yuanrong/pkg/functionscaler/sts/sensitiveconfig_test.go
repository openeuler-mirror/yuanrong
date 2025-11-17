package sts

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"
	"huawei.com/wisesecurity/sts-sdk/pkg/auth"
	"huawei.com/wisesecurity/sts-sdk/pkg/remote"
	"huawei.com/wisesecurity/sts-sdk/pkg/stsgoapi"
)

type mockHttpBody struct {
}

func (m *mockHttpBody) Read(p []byte) (n int, err error) {
	p = append(p, byte(1))
	return 1, nil
}

func (m *mockHttpBody) Close() error {
	return nil
}

func TestGetEnvMap(t *testing.T) {
	convey.Convey("GetEnvMap", t, func() {
		defer gomonkey.ApplyFunc(GetStsHTTPClient, func() *remote.StsKeyHttpClient {
			return &remote.StsKeyHttpClient{}
		}).Reset()
		defer gomonkey.ApplyFunc(stsgoapi.SignRequest, func(httpRequest *remote.StsHttpRequest, providerServiceMeta auth.StsFullServiceMeta) (
			*remote.StsHttpRequest, error) {
			return nil, nil
		}).Reset()
		defer gomonkey.ApplyFunc(io.ReadAll, func(r io.Reader) ([]byte, error) {
			return json.Marshal(SensitiveConfigResponse{ConfigItems: []ConfigItem{{
				ConfigID:    "config1",
				ConfigValue: "configValue1",
			}}})
		}).Reset()
		convey.Convey("failed", func() {
			p1 := gomonkey.ApplyMethod(reflect.TypeOf(&remote.StsKeyHttpClient{}), "SendMessage",
				func(_ *remote.StsKeyHttpClient, stsHttpRequest *remote.StsHttpRequest) (*http.Response, error) {
					return &http.Response{StatusCode: http.StatusBadRequest, Body: &mockHttpBody{}}, nil
				})
			configMap := make(map[string]string)
			configMap["configID1"] = "config1"
			_, err := GetEnvMap(configMap)
			convey.So(err, convey.ShouldNotBeNil)
			p1.Reset()

			p2 := gomonkey.ApplyMethod(reflect.TypeOf(&remote.StsKeyHttpClient{}), "SendMessage",
				func(_ *remote.StsKeyHttpClient, stsHttpRequest *remote.StsHttpRequest) (*http.Response, error) {
					return nil, errors.New("http error")
				})
			_, err = GetEnvMap(configMap)
			convey.So(err, convey.ShouldNotBeNil)
			p2.Reset()
		})
		convey.Convey("success", func() {
			defer gomonkey.ApplyMethod(reflect.TypeOf(&remote.StsKeyHttpClient{}), "SendMessage",
				func(_ *remote.StsKeyHttpClient, stsHttpRequest *remote.StsHttpRequest) (*http.Response, error) {
					return &http.Response{StatusCode: http.StatusOK, Body: &mockHttpBody{}}, nil
				}).Reset()
			configMap := make(map[string]string)
			configMap["configID1"] = "config1"
			envMap, err := GetEnvMap(configMap)
			convey.So(err, convey.ShouldBeNil)
			convey.So(envMap["configID1"], convey.ShouldEqual, "configValue1")
		})
	})
}
