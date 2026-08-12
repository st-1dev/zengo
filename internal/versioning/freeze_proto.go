package versioning

import "regexp"

var (
	packageRe   = regexp.MustCompile(`(?m)^package\s+([\w\.]+)\s*;`)
	goPackageRe = regexp.MustCompile(`(?m)^option\s+go_package\s*=\s*"([^";]+)`)
)
