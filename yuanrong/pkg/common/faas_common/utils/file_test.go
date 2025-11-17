package utils

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFileExists(t *testing.T) {
	Convey("Given a temp file", t, func() {
		file, err := ioutil.TempFile("", "test-file")
		So(err, ShouldBeNil)
		filename := file.Name()

		Convey("When it is created", func() {
			Convey("Then it should return true", func() {
				So(FileExists(filename), ShouldBeTrue)
			})
		})

		Convey("When we delete the file", func() {
			err := file.Close()
			So(err, ShouldBeNil)
			err = os.Remove(filename)
			So(err, ShouldBeNil)

			Convey("Then it should return false", func() {
				So(FileExists(filename), ShouldBeFalse)
			})
		})
	})
}

func TestValidateFilePath(t *testing.T) {
	Convey("Given a abs file path and a rel file path", t, func() {
		relPath := "a/b"
		absPath, err := filepath.Abs(relPath)
		So(err, ShouldBeNil)

		Convey("The abs path should not return an error", func() {
			err = ValidateFilePath(absPath)
			So(err, ShouldBeNil)
		})

		Convey("The rel path should return an error", func() {
			err := ValidateFilePath(relPath)
			So(err, ShouldNotBeNil)
		})
	})
}
