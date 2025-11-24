package localauth

import (
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestEncrypt(t *testing.T) {
	convey.Convey("Encrypt", t, func() {
		encrypt, err := Encrypt("123")
		convey.So(encrypt, convey.ShouldEqual, "123")
		convey.So(err, convey.ShouldBeNil)
	})
}

func TestDecrypt(t *testing.T) {
	convey.Convey("Encrypt", t, func() {
		src := "123"
		decrypt, err := Decrypt(src)
		convey.So(string(decrypt), convey.ShouldEqual, "123")
		convey.So(err, convey.ShouldBeNil)
	})
}
