package sts

import (
	"github.com/agiledragon/gomonkey/v2"
	"github.com/magiconair/properties"
	"github.com/smartystreets/goconvey/convey"
	"huawei.com/wisesecurity/sts-sdk/pkg/stsgoapi"
	"testing"
	"yuanrong.org/kernel/runtime/faassdk/types"
)

func TestInitStsSDK(t *testing.T) {
	convey.Convey("InitStsSDK", t, func() {
		convey.Convey("success", func() {
			defer gomonkey.ApplyFunc(stsgoapi.InitWith, func(property properties.Properties) error {
				return nil
			}).Reset()
			err := InitStsSDK(types.StsServerConfig{})
			convey.So(err, convey.ShouldBeNil)
		})
	})
}
