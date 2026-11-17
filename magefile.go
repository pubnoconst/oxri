//go:build mage
// +build mage

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Default target to run when no argument is given.
var Default = Run

// Generate runs templ to compile all .templ files into Go code.
func Generate() error {
	fmt.Println("==> Generating Templ templates...")
	templCmd := "templ"
	if _, err := exec.LookPath("templ"); err != nil {
		gopath := os.Getenv("GOPATH")
		if gopath == "" {
			home, _ := os.UserHomeDir()
			gopath = filepath.Join(home, "go")
		}
		candidate := filepath.Join(gopath, "bin", "templ.exe")
		if _, err := os.Stat(candidate); err == nil {
			templCmd = candidate
		}
	}
	return sh.Run(templCmd, "generate")
}

// Build compiles the application binary into bin/.
func Build() error {
	mg.Deps(Generate)
	fmt.Println("==> Compiling server binary into bin/...")
	if err := os.MkdirAll("bin", 0755); err != nil {
		return err
	}
	binName := "bin/server"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	return sh.Run("go", "build", "-o", binName, "./cmd/server")
}

// Run emulates 'cargo run': compiles templates and launches the server.
func Run() error {
	mg.Deps(Generate)
	fmt.Println("==> Starting Res Imperii server...")
	return sh.RunV("go", "run", "./cmd/server")
}

// Test executes the full unit and integration test suite.
func Test() error {
	mg.Deps(Generate)
	fmt.Println("==> Running tests...")
	return sh.RunV("go", "test", "-v", "./...")
}

// Clean removes build artifacts in bin/ and generated templ Go files.
func Clean() error {
	fmt.Println("==> Removing bin/...")
	if err := os.RemoveAll("bin"); err != nil {
		return err
	}
	fmt.Println("==> Removing generated *_templ.go files...")
	return filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), "_templ.go") {
			return os.Remove(path)
		}
		return nil
	})
}
