package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"yuanrong.org/kernel/runtime/yr"
)

func InitYR() {
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
}

func KVSetAndGetAndDelExample() {
	err := yr.SetKV("key", "value")
	if err != nil {
		fmt.Println("set kv failed, error: ", err)
	}

	fmt.Println(yr.GetKV("key", 30000))

	fmt.Println(yr.DelKV("key"))
}

func BatchKVSetAndGetAndDelExample() {
	err := yr.SetKV("key", "value")
	if err != nil {
		fmt.Println("set kv failed, error: ", err)
	}

	err = yr.SetKV("key1", "value1")
	if err != nil {
		fmt.Println("set kv failed, error: ", err)
	}

	fmt.Println(yr.GetKVs([]string{"key", "key1"}, 30000, false))

	fmt.Println(yr.DelKVs([]string{"key", "key1"}))

	fmt.Println(yr.GetKVs([]string{"key", "key1"}, 30000, false))
}

// compile command:
// CGO_LDFLAGS="-L$(path which contains libcpplibruntime.so)" go build -o main kv_example.go
func main() {
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGINT)
	InitYR()
	KVSetAndGetAndDelExample()
	BatchKVSetAndGetAndDelExample()
	<-c
}
