// +build tools

package tools

import (
	// These imports refer to tools intendend to be
	// used while developing or running CI.
	_ "github.com/magefile/mage"
	_ "golang.org/x/lint/golint"
	_ "golang.org/x/tools/cmd/goimports"
)
