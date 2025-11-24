package dynamicconfigmanager

import (
	"errors"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"
	v1 "k8s.io/api/core/v1"

	"yuanrong.org/kernel/pkg/common/faas_common/k8sclient"
	commonTypes "yuanrong.org/kernel/pkg/common/faas_common/types"
	"yuanrong.org/kernel/pkg/functionscaler/types"
)

func TestHandleUpdateFunctionEvent(t *testing.T) {
	convey.Convey("HandleUpdateFunctionEvent", t, func() {
		defer gomonkey.ApplyMethod(reflect.TypeOf(&k8sclient.KubeClient{}), "CreateOrUpdateConfigMap",
			func(_ *k8sclient.KubeClient, c *v1.ConfigMap) error {
				return errors.New("CreateOrUpdateConfigMap configmap error")
			}).Reset()
		HandleUpdateFunctionEvent(&types.FunctionSpecification{
			ExtendedMetaData: commonTypes.ExtendedMetaData{DynamicConfig: commonTypes.DynamicConfigEvent{
				Enabled:       true,
				ConfigContent: []commonTypes.KV{{Name: "config1", Value: "value1"}},
			}},
		})
	})
}

func TestHandleDeleteFunctionEvent(t *testing.T) {
	convey.Convey("HandleDeleteFunctionEvent", t, func() {
		defer gomonkey.ApplyMethod(reflect.TypeOf(&k8sclient.KubeClient{}), "DeleteK8sConfigMap",
			func(_ *k8sclient.KubeClient, namespace string, configMapName string) error {
				return errors.New("delete configmap error")
			}).Reset()
		HandleDeleteFunctionEvent(&types.FunctionSpecification{})
	})
}
