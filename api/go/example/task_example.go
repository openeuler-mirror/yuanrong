package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"yuanrong.org/kernel/runtime/yr"
)

func PlusOne(x int) int {
	fmt.Println("Hello PlusOne")
	return x + 1
}

func PlusOne2(x int) int {
	fmt.Println("Hello PlusOne2")
	function := yr.Function(PlusOne).Options(yr.NewInvokeOptions())
	ref := function.Invoke(300)
	fmt.Println(yr.Get[int](ref[0], 3000))
	return x + 1
}

func TaskExample() {
	flag.Parse()
	config := &yr.Config{
		Mode:           yr.ClusterMode,
		FunctionUrn:    "sn:cn:yrk:12345678901234561234567890123456:function:0-opc-opc:$latest",
		ServerAddr:     flag.Args()[0],
		DataSystemAddr: flag.Args()[1],
		InCluster:      true,
		LogLevel:       "DEBUG",
	}
	info, err := yr.Init(config)
	if err != nil {
		fmt.Println("Init failed, error: ", err)
	}
	fmt.Println(info)

	function := yr.Function(PlusOne).Options(yr.NewInvokeOptions())
	ref := function.Invoke(298)
	fmt.Println(yr.Get[int](ref[0], 3000))

	// task call task
	function1 := yr.Function(PlusOne2).Options(yr.NewInvokeOptions())
	ref1 := function1.Invoke(298)
	fmt.Println(yr.Get[int](ref1[0], 3000))
}

// compile command:
// CGO_LDFLAGS="-L$(path which contains libcpplibruntime.so)" go build -buildmode=plugin -o yrlib.so task_example.go
// CGO_LDFLAGS="-L$(path which contains libcpplibruntime.so)" go build -o main task_example.go
func main() {
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGINT)
	TaskExample()
	<-c
}
