// protoc-gen-go-zz runs protoc-gen-go and prefixes each *.pb.go output with zz_generated_.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

// main proxies protoc-gen-go and renames generated outputs to zz_generated_*.pb.go.
func main() {
	in, err := io.ReadAll(os.Stdin)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "protoc-gen-go-zz: read stdin: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.CommandContext(context.Background(), "protoc-gen-go")
	cmd.Stdin = bytes.NewReader(in)

	var out []byte
	out, err = cmd.Output()
	if err != nil {
		if len(out) > 0 {
			_, _ = os.Stdout.Write(out)
		}

		ee, ok := errors.AsType[*exec.ExitError](err)
		if ok && len(ee.Stderr) > 0 {
			_, _ = os.Stderr.Write(ee.Stderr)
		}

		_, _ = fmt.Fprintf(os.Stderr, "protoc-gen-go-zz: protoc-gen-go: %v\n", err)
		os.Exit(1)
	}

	var resp pluginpb.CodeGeneratorResponse
	err = proto.Unmarshal(out, &resp)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "protoc-gen-go-zz: unmarshal response: %v\n", err)
		os.Exit(1)
	}

	if resp.Error != nil {
		_, _ = fmt.Fprintf(os.Stderr, "protoc-gen-go: %s\n", resp.GetError())
		os.Exit(1)
	}

	for _, f := range resp.File {
		name := f.GetName()
		if !strings.HasSuffix(name, ".pb.go") {
			continue
		}
		dir, base := filepath.Split(name)
		if strings.HasPrefix(base, "zz_generated_") {
			continue
		}
		f.Name = new(dir + "zz_generated_" + base)
	}

	var encoded []byte
	encoded, err = proto.Marshal(&resp)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "protoc-gen-go-zz: marshal response: %v\n", err)
		os.Exit(1)
	}

	_, err = os.Stdout.Write(encoded)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "protoc-gen-go-zz: write stdout: %v\n", err)
		os.Exit(1)
	}
}
