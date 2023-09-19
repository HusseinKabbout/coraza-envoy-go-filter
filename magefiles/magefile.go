package main

import (
	"errors"
	"fmt"
	"github.com/magefile/mage/sh"
	"io"
	"os"
	"runtime"
	"strings"
)

var (
	available_os      = "linux"
	addLicenseVersion = "04bfe4ee9ca5764577b029acc6a1957fd1997153" // https://github.com/google/addlicense
	gosImportsVer     = "v0.3.1"                                   // https://github.com/rinchsan/gosimports/releases/tag/v0.3.1
)

// Build the go filter waf plugin.It only works on linux
func Build() error {
	os := runtime.GOOS
	if !strings.Contains(available_os, os) {
		return errors.New(fmt.Sprintf("%s is not available , place compile in %s", os, available_os))
	}
	if err := sh.RunV("touch", "access.log"); err != nil {
		return err
	}
	if err := sh.RunV("chmod", "666", "access.log"); err != nil {
		return err
	}
	return sh.RunV("go", "build", "-o", "plugin.so", "-buildmode=c-shared", "./plugin")
}

// RunExample spins up the test environment, access at http://localhost:8080. Requires docker-compose.
func RunExample() error {
	return sh.RunV("docker-compose", "--file", "example/docker-compose.yml", "up", "-d")
}

// TeardownExample tears down the test environment. Requires docker-compose.
func TeardownExample() error {
	return sh.RunV("docker-compose", "--file", "example/docker-compose.yml", "down")
}

// Coverage runs tests with coverage and race detector enabled.
func Coverage() error {
	if err := os.MkdirAll("build", 0755); err != nil {
		return err
	}
	if err := sh.RunV("go", "test", "-race", "-coverprofile=build/coverage.txt", "-covermode=atomic", "-coverpkg=./...", "./..."); err != nil {
		return err
	}

	return sh.RunV("go", "tool", "cover", "-html=build/coverage.txt", "-o", "build/coverage.html")
}

// Format formats code in this repository.
func Format() error {
	if err := sh.RunV("go", "mod", "tidy"); err != nil {
		return err
	}
	// addlicense strangely logs skipped files to stderr despite not being erroneous, so use the long sh.Exec form to
	// discard stderr too.
	if _, err := sh.Exec(map[string]string{}, io.Discard, io.Discard, "go", "run", fmt.Sprintf("github.com/google/addlicense@%s", addLicenseVersion),
		"-c", "The OWASP Coraza contributors",
		"-s=only",
		"-y=",
		"-ignore", "**/*.yml",
		"-ignore", "**/*.yaml",
		"-ignore", "examples/**", "."); err != nil {
		return err
	}
	return sh.RunV("go", "run", fmt.Sprintf("github.com/rinchsan/gosimports/cmd/gosimports@%s", gosImportsVer),
		"-w",
		"-local",
		"github.com/corazawaf/coraza-proxy-wasm",
		".")
}

// E2e runs e2e tests with a built plugin against the example deployment. Requires docker-compose.
func E2e() error {
	if err := sh.RunV("docker-compose", "--file", "e2e/docker-compose.yml", "build", "--pull"); err != nil {
		return err
	}
	return sh.RunV("docker-compose", "--file", "e2e/docker-compose.yml", "up", "--abort-on-container-exit", "tests")
}

// Doc runs godoc, access at http://localhost:6060
func Doc() error {
	return sh.RunV("go", "run", "golang.org/x/tools/cmd/godoc@latest", "-http=:6060")
}

// Ftw runs ftw tests with a built plugin and Envoy. Requires docker-compose.
func Ftw() error {
	if err := sh.RunV("docker-compose", "--file", "ftw/docker-compose.yml", "build", "--pull"); err != nil {
		return err
	}
	defer func() {
		_ = sh.RunV("docker-compose", "--file", "ftw/docker-compose.yml", "down", "-v")
	}()
	env := map[string]string{
		"FTW_CLOUDMODE": os.Getenv("FTW_CLOUDMODE"),
		"FTW_INCLUDE":   os.Getenv("FTW_INCLUDE"),
		"ENVOY_IMAGE":   os.Getenv("ENVOY_IMAGE"),
	}
	if os.Getenv("ENVOY_NOWASM") == "true" {
		env["ENVOY_CONFIG"] = "/conf/envoy-config-nowasm.yaml"
	}
	task := "ftw"
	if os.Getenv("MEMSTATS") == "true" {
		task = "ftw-memstats"
	}
	return sh.RunWithV(env, "docker-compose", "--file", "ftw/docker-compose.yml", "run", "--rm", task)
}
