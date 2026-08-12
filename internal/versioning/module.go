package versioning

import (
	"bufio"
	"os"
	"strings"
)

// ReadModule reads the module path from go.mod in the current directory.
func ReadModule() string {
	return ReadModuleAt("go.mod")
}

// ReadModuleAt reads the module path from the specified go.mod file.
func ReadModuleAt(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() {
		_ = f.Close()
	}()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		after, ok := strings.CutPrefix(line, "module ")
		if ok {
			return strings.TrimSpace(after)
		}
	}
	return ""
}
