// +build mage,go1.9

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/magefile/mage/sh"
)

const (
	// Linting tools.
	goplsImportPath     = "golang.org/x/tools/gopls"
	goimportsImportPath = "golang.org/x/tools/cmd/goimports"
	golintImportPath    = "golang.org/x/lint/golint"

	bin = ".bin"
)

var (
	goCmd     = os.Getenv("GOCMD")
	goVersion = runtime.Version()

	cwd, _ = os.Getwd()
)

func init() {
	if goCmd == "" {
		goCmd = "go"
	}
}

// Fix runs "goimports -w ." to fix all files.
func Fix() error {
	root, err := findRoot(cwd)
	if err != nil {
		return err
	}

	goimportsCmd := filepath.Join(root, bin, "goimports")
	return sh.Run(goimportsCmd, "-w", root)
}

// Install installs all development dependencies.
func Install() error {
	root, err := findRoot(cwd)
	if err != nil {
		return err
	}
	deps := []string{
		goplsImportPath, // not used in CI
		goimportsImportPath,
		golintImportPath,
	}
	gobin := filepath.Join(root, bin)
	for _, dep := range deps {
		if err := sh.RunWith(
			map[string]string{"GOBIN": gobin},
			goCmd, "install", dep,
		); err != nil {
			return err
		}
	}
	return nil
}

// Lint lints using "golint" and "goimports".
func Lint() error {
	root, err := findRoot(cwd)
	if err != nil {
		return err
	}
	goimportsCmd := filepath.Join(root, bin, "goimports")
	goimportsDiff, err := sh.Output(goimportsCmd, "-d", root)
	if err != nil {
		return err
	}
	if goimportsDiff != "" {
		return fmt.Errorf("\n%s", goimportsDiff)
	}
	golintCmd := filepath.Join(root, bin, "golint")
	return sh.Run(golintCmd, "-set_exit_status", filepath.Join(root, "..."))
}

// Test tests using "go test".
func Test() error {
	root, err := findRoot(cwd)
	if err != nil {
		return err
	}
	flags := strings.Split(os.Getenv("TEST_FLAGS"), " ")
	args := append([]string{"test"}, flags...)
	return sh.Run(goCmd, append(args, filepath.Join(root, "..."))...)
}

func findRoot(dir string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", err
	}
	if matches != nil {
		return dir, nil
	}
	return findRoot(filepath.Dir(dir))
}
