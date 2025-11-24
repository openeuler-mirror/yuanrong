package utils

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestClearStringMemory(t *testing.T) {
	Convey("Given a string", t, func() {
		testStr := "helloworld"

		b := []byte(testStr)
		s := string(b)
		Convey("When we clear the string", func() {
			ClearStringMemory(s)
			Convey("The string should be empty", func() {
				So(s, ShouldEqual, string([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}))
			})
		})
	})
}
