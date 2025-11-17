package serviceaccount

import (
	"fmt"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/json-iterator/go"
	"github.com/smartystreets/goconvey/convey"

	"huawei.com/wisesecurity/sts-sdk/pkg/stsgoapi"

	"yuanrong/pkg/common/faas_common/wisecloudtool/types"
)

func TestCipherSuitesFromName(t *testing.T) {
	convey.Convey("Test cipherSuitesFromName", t, func() {
		convey.Convey("success", func() {
			cipherSuitesArr := []string{"TLS_AES_128_GCM_SHA256", "TLS_AES_256_GCM_SHA384"}
			tlsSuite := cipherSuitesID(cipherSuitesFromName(cipherSuitesArr))
			convey.So(len(tlsSuite), convey.ShouldEqual, 2)
		})
	})
}

func TestParseServiceAccount(t *testing.T) {
	convey.Convey("Test ParseServiceAccount", t, func() {
		// Setup test cases
		validServiceAccount := &types.ServiceAccount{
			PrivateKey: "test-PrivateKey",
			ClientId:   111,
		}
		validJSON, _ := jsoniter.Marshal(validServiceAccount)

		convey.Convey("when input is empty", func() {
			_, err := ParseServiceAccount("")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "serviceAccountKeyStr is empty")
		})

		convey.Convey("when decryption fails", func() {
			patches := gomonkey.ApplyFunc(stsgoapi.DecryptSensitiveConfig, func(string) ([]byte, error) {
				return nil, fmt.Errorf("decryption error")
			})
			defer patches.Reset()

			_, err := ParseServiceAccount("encrypted-string")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "decrypt service account key failed")
		})

		convey.Convey("when decryption succeeds but unmarshal fails", func() {
			patches := gomonkey.ApplyFunc(stsgoapi.DecryptSensitiveConfig, func(string) ([]byte, error) {
				return []byte("invalid-json"), nil
			})
			defer patches.Reset()

			_, err := ParseServiceAccount("encrypted-string")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "unmarshal service account key failed")
		})

		convey.Convey("when everything works correctly", func() {
			patches := gomonkey.ApplyFunc(stsgoapi.DecryptSensitiveConfig, func(string) ([]byte, error) {
				return validJSON, nil
			})
			defer patches.Reset()

			result, err := ParseServiceAccount("encrypted-string")
			convey.So(err, convey.ShouldBeNil)
			convey.So(result.PrivateKey, convey.ShouldEqual, validServiceAccount.PrivateKey)
			convey.So(result.ClientId, convey.ShouldEqual, validServiceAccount.ClientId)
		})
	})
}
