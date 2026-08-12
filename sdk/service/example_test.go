package service_test

import (
	"bytes"
	"context"
	"zengo/platform/sdk/service"
)

func ExampleNew() {
	runtime, _ := service.New(context.Background(), service.Options{
		Name: "demo-service",
	})
	_ = runtime
}

func ExampleNewGRPCServer() {
	server, _ := service.NewGRPCServer(false)
	_ = server
}

func ExamplePrintVersion() {
	var buf bytes.Buffer
	_ = service.PrintVersion(&buf, "demo-service")
}
