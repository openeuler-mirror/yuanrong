package monitor

import (
	"testing"

	"github.com/smartystreets/goconvey/convey"

	"yuanrong.org/kernel/runtime/faassdk/types"
)

func TestCreateFunctionMonitor(t *testing.T) {
	convey.Convey("CreateFunctionMonitor", t, func() {
		spec := &types.FuncSpec{ResourceMetaData: types.ResourceMetaData{Memory: 500}}
		convey.Convey("create monitor custom image disk", func() {
			monitor, err := CreateFunctionMonitor(spec, make(chan struct{}))
			convey.So(monitor, convey.ShouldNotBeNil)
			convey.So(err, convey.ShouldBeNil)
		})
		convey.Convey("success", func() {
			monitor, err := CreateFunctionMonitor(spec, make(chan struct{}))
			convey.So(monitor, convey.ShouldNotBeNil)
			convey.So(err, convey.ShouldBeNil)
			close(monitor.MemoryManager.OOMChan)
			err = <-monitor.ErrChan
			convey.So(err, convey.ShouldEqual, ErrExecuteOOM)
		})
	})
}
