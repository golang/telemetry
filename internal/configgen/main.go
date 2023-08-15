// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package configgen generates the upload config file stored in the config.json
// file of golang.org/x/telemetry/config.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	_ "embed"

	"golang.org/x/mod/semver"
	"golang.org/x/telemetry"
	"golang.org/x/telemetry/internal/graphconfig"
)

var write = flag.Bool("w", false, "if set, write the config file; otherwise, print to stdout")

//go:embed config.txt
var graphConfig []byte

func main() {
	flag.Parse()

	ucfg, err := generate(graphConfig)
	if err != nil {
		log.Fatal(err)
	}
	ucfgJSON, err := json.MarshalIndent(ucfg, "", "\t")
	if err != nil {
		log.Fatal(err)
	}
	if !*write {
		fmt.Println(string(ucfgJSON))
		os.Exit(0)
	}
	configFile, err := configFile()
	if err != nil {
		log.Fatalf("finding config file: %v", err)
	}
	os.WriteFile(configFile, ucfgJSON, 0666)
}

// configFile returns the path to the x/telemetry/config config.json file in
// this repo.
//
// The file must already exist: this won't be a valid location if running from
// the module cache; this functionality only works when executed from the
// telemetry repo.
func configFile() (string, error) {
	out, err := exec.Command("go", "list", "-f", "{{.Dir}}", "golang.org/x/telemetry/internal/configgen").Output()
	if err != nil {
		return "", err
	}
	dir := strings.TrimSpace(string(out))
	configFile := filepath.Join(dir, "..", "..", "config", "config.json")
	if _, err := os.Stat(configFile); err != nil {
		return "", err
	}
	return configFile, nil
}

// generate computes the upload config from graph configs and module
// information, returning the resulting formatted JSON.
func generate(graphConfig []byte, env ...string) (*telemetry.UploadConfig, error) {
	ucfg := &telemetry.UploadConfig{
		GOOS:   goos(),
		GOARCH: goarch(),
	}
	var err error
	ucfg.GoVersion, err = goVersions()
	if err != nil {
		return nil, fmt.Errorf("querying go info: %v", err)
	}

	gcfgs, err := graphconfig.Parse(graphConfig)
	if err != nil {
		return nil, fmt.Errorf("parsing graph config records: %v", err)
	}

	for i, r := range gcfgs {
		if err := graphconfig.Validate(r); err != nil {
			// TODO(rfindley): this is a poor way to identify the faulty record. We
			// should probably store position information in the GraphConfig.
			return nil, fmt.Errorf("graph config #%d (%q): %v", i, r.Title, err)
		}
	}

	var (
		programs    = make(map[string]*telemetry.ProgramConfig) // package path -> config
		minVersions = make(map[string]string)                   // package path -> min version required, or "" for all
	)
	for _, gcfg := range gcfgs {
		pcfg := programs[gcfg.Program]
		if pcfg == nil {
			pcfg = &telemetry.ProgramConfig{
				Name: gcfg.Program,
			}
			programs[gcfg.Program] = pcfg
			minVersions[gcfg.Program] = gcfg.Version
		}
		minVersions[gcfg.Program] = minVersion(minVersions[gcfg.Program], gcfg.Version)
		ccfg := telemetry.CounterConfig{
			Name:  gcfg.Counter,
			Rate:  0.1, // TODO(rfindley): how should rate be configured?
			Depth: gcfg.Depth,
		}
		if gcfg.Depth > 0 {
			pcfg.Stacks = append(pcfg.Stacks, ccfg)
		} else {
			pcfg.Counters = append(pcfg.Counters, ccfg)
		}
	}

	for _, p := range programs {
		minVersion := minVersions[p.Name]
		versions, err := listProxyVersions(p.Name)
		if err != nil {
			return nil, fmt.Errorf("listing versions for %q: %v", p.Name, err)
		}
		// Filter proxy versions in place.
		i := 0
		for _, v := range versions {
			if !semver.IsValid(v) {
				// In order to perform semver comparison below, we must have valid
				// versions. This should always be the case for the proxy.
				// Trust, but verify.
				return nil, fmt.Errorf("invalid semver %q returned from proxy for %q", v, p.Name)
			}
			if minVersion == "" || semver.Compare(minVersion, v) <= 0 {
				versions[i] = v
				i++
			}
		}
		p.Versions = versions[:i]
		ucfg.Programs = append(ucfg.Programs, p)
	}
	sort.Slice(ucfg.Programs, func(i, j int) bool {
		return ucfg.Programs[i].Name < ucfg.Programs[j].Name
	})

	return ucfg, nil
}

// minVersion returns the lesser semantic version of v1 and v2.
//
// As a special case, the empty string is treated as an absolute minimum
// (empty => all versions are greater).
func minVersion(v1, v2 string) string {
	if v1 == "" || v2 == "" {
		return ""
	}
	if semver.Compare(v1, v2) > 0 {
		return v2
	}
	return v1
}

// goos returns a sorted slice of known GOOS values.
func goos() []string {
	var gooses []string
	for goos := range knownOS {
		gooses = append(gooses, goos)
	}
	sort.Strings(gooses)
	return gooses
}

// goarch returns a sorted slice of known GOARCH values.
func goarch() []string {
	var arches []string
	for arch := range knownArch {
		arches = append(arches, arch)
	}
	sort.Strings(arches)
	return arches
}

// goInfo queries the proxy for information about go distributions, including
// versions, GOOS, and GOARCH values.
func goVersions() ([]string, error) {
	// Trick: read Go distribution information from the module versions of
	// golang.org/toolchain. These define the set of valid toolchains, and
	// therefore are a reasonable source for version information.
	//
	// A more authoritative source for this information may be
	// https://go.dev/dl?mode=json&include=all.
	proxyVersions, err := listProxyVersions("golang.org/toolchain")
	if err != nil {
		return nil, fmt.Errorf("listing toolchain versions: %v", err)
	}
	var goVersionRx = regexp.MustCompile(`^-(go.+)\.[^.]+-[^.]+$`)
	verSet := make(map[string]struct{})
	for _, v := range proxyVersions {
		pre := semver.Prerelease(v)
		match := goVersionRx.FindStringSubmatch(pre)
		if match == nil {
			return nil, fmt.Errorf("proxy version %q does not match prerelease regexp %q", v, goVersionRx)
		}
		verSet[match[1]] = struct{}{}
	}
	var vers []string
	for v := range verSet {
		vers = append(vers, v)
	}
	sort.Sort(byGoVersion(vers))
	return vers, nil
}

type byGoVersion []string

func (vs byGoVersion) Len() int      { return len(vs) }
func (vs byGoVersion) Swap(i, j int) { vs[i], vs[j] = vs[j], vs[i] }
func (vs byGoVersion) Less(i, j int) bool {
	cmp := Compare(vs[i], vs[j])
	if cmp != 0 {
		return cmp < 0
	}
	// To ensure that we have a stable sort, order equivalent Go versions lexically.
	return vs[i] < vs[j]
}

// versionsForTesting contains versions to use for testing, rather than
// querying the proxy.
var versionsForTesting map[string][]string

// listProxyVersions queries the Go module mirror for published versions of the
// given modulePath.
//
// modulePath must be lower-case (or already escaped): this function doesn't do
// any escaping of upper-cased letters, as is required by the proxy prototol
// (https://go.dev/ref/mod#goproxy-protocol).
func listProxyVersions(modulePath string) ([]string, error) {
	if vers, ok := versionsForTesting[modulePath]; ok {
		return vers, nil
	}
	cmd := exec.Command("go", "list", "-m", "--versions", modulePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing versions: %v (stderr: %v)", err, stderr.String())
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) == 0 {
		return nil, fmt.Errorf("invalid version list output: %q", string(out))
	}
	return fields[1:], nil
}
