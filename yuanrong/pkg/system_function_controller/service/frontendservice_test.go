package service

import (
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"
	v1 "k8s.io/api/core/v1"

	"yuanrong/pkg/common/faas_common/k8sclient"
	"yuanrong/pkg/system_function_controller/config"
	"yuanrong/pkg/system_function_controller/types"
)

func TestCreateFrontendService(t *testing.T) {
	convey.Convey("CreateFrontendService", t, func() {
		defer gomonkey.ApplyMethod(reflect.TypeOf(&k8sclient.KubeClient{}), "CreateK8sService",
			func(_ *k8sclient.KubeClient, service *v1.Service) error {
				return nil
			}).Reset()
		defer gomonkey.ApplyFunc(config.GetFaaSControllerConfig, func() types.Config {
			return types.Config{NameSpace: "default"}
		}).Reset()
		err := CreateFrontendService()
		convey.So(err, convey.ShouldBeNil)
	})
}
