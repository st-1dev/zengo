package buildinfo_test

import (
	"bytes"
	"zengo/platform/sdk/buildinfo"
)

func ExampleCurrent() {
	info := buildinfo.Current("demo-service")
	_ = info
}

func ExamplePrint() {
	var buf bytes.Buffer
	_ = buildinfo.Print(&buf, "demo-service")
}
