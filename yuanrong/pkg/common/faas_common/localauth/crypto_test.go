package localauth

import (
	"sync"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"
	"huaweicloud.com/containers/security/cbb_adapt/src/go/gcrypto"

	"yuanrong/pkg/common/faas_common/utils"
)

type mockEngine struct {
	gcrypto.Engine
}

func (m mockEngine) Encrypt(domainId int, encData string) (string, error) {
	return encData, nil
}

func (m mockEngine) Decrypt(domainId int, encData string) (string, error) {
	return encData, nil
}

func TestEncrypt(t *testing.T) {
	convey.Convey("Encrypt", t, func() {
		defer gomonkey.ApplyFunc(gcrypto.New, func(algo string) (engine gcrypto.Engine, err error) {
			return &mockEngine{}, nil
		}).Reset()
		once = sync.Once{}
		encrypt, err := Encrypt("123")
		convey.So(encrypt, convey.ShouldEqual, "123")
		convey.So(err, convey.ShouldBeNil)
	})
}

func TestDecrypt(t *testing.T) {
	convey.Convey("Encrypt", t, func() {
		defer gomonkey.ApplyFunc(gcrypto.New, func(algo string) (engine gcrypto.Engine, err error) {
			return &mockEngine{}, nil
		}).Reset()
		defer gomonkey.ApplyFunc(utils.ClearByteMemory, func(b []byte) {
			return
		}).Reset()
		once = sync.Once{}
		src := "123"
		decrypt, err := Decrypt(src)
		convey.So(string(decrypt), convey.ShouldEqual, "123")
		convey.So(err, convey.ShouldBeNil)
	})
}
