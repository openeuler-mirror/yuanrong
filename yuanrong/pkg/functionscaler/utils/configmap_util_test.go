package utils

import (
	"encoding/base64"
	"testing"

	"github.com/smartystreets/goconvey/convey"

	commonTypes "yuanrong/pkg/common/faas_common/types"
	"yuanrong/pkg/functionscaler/types"
)

func TestDeleteConfigMapByFuncInfo(t *testing.T) {
	convey.Convey("DeleteConfigMapByFuncInfo", t, func() {
		DeleteConfigMapByFuncInfo(&types.FunctionSpecification{
			FuncMetaData: commonTypes.FuncMetaData{FuncName: "func-name", Version: "latest"},
			ExtendedMetaData: commonTypes.ExtendedMetaData{
				CustomFilebeatConfig: commonTypes.CustomFilebeatConfig{
					ImageAddress: "images",
					SidecarConfigInfo: &commonTypes.SidecarConfigInfo{
						ConfigFiles: []commonTypes.CustomLogConfigFile{{Path: "path",
							Data: base64.StdEncoding.EncodeToString([]byte("data"))}},
					},
				},
			}})
	})
}

func TestIsNeedRaspSideCar(t *testing.T) {
	convey.Convey("IsNeedRaspSideCar", t, func() {
		isNeed := IsNeedRaspSideCar(&types.FunctionSpecification{
			ExtendedMetaData: commonTypes.ExtendedMetaData{
				RaspConfig: commonTypes.RaspConfig{
					RaspImage:      "someImage",
					InitImage:      "someImage",
					RaspServerIP:   "1.2.3.4",
					RaspServerPort: "1234",
				},
			},
		})
		convey.So(isNeed, convey.ShouldEqual, true)
		isNeed = IsNeedRaspSideCar(&types.FunctionSpecification{
			ExtendedMetaData: commonTypes.ExtendedMetaData{
				RaspConfig: commonTypes.RaspConfig{
					InitImage:      "someImage",
					RaspServerIP:   "1.2.3.4",
					RaspServerPort: "1234",
				},
			},
		})
		convey.So(isNeed, convey.ShouldEqual, false)
		isNeed = IsNeedRaspSideCar(&types.FunctionSpecification{
			ExtendedMetaData: commonTypes.ExtendedMetaData{
				RaspConfig: commonTypes.RaspConfig{
					RaspImage:      "someImage",
					InitImage:      "someImage",
					RaspServerPort: "1234",
				},
			},
		})
		convey.So(isNeed, convey.ShouldEqual, false)
		isNeed = IsNeedRaspSideCar(&types.FunctionSpecification{
			ExtendedMetaData: commonTypes.ExtendedMetaData{
				RaspConfig: commonTypes.RaspConfig{
					RaspImage:    "someImage",
					InitImage:    "someImage",
					RaspServerIP: "1.2.3.4",
				},
			},
		})
		convey.So(isNeed, convey.ShouldEqual, false)
	})
}
