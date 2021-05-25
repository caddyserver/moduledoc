// Copyright 2019 Matthew Holt
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package moduledoc

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"
)

type workspace struct {
	mu     *sync.RWMutex
	dir    string
	driver *Driver
	goGets map[string]struct{}
}

func (d *Driver) openWorkspace() (workspace, error) {
	tempDir, err := ioutil.TempDir("", "caddy_docsys_")
	if err != nil {
		return workspace{}, err
	}

	cmd := exec.Command("go", "mod", "init", "temp/docsys")
	cmd.Dir = tempDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		os.RemoveAll(tempDir)
		return workspace{}, fmt.Errorf("exec %v: %v", cmd.Args, err)
	}

	return workspace{
		mu:     new(sync.RWMutex),
		dir:    tempDir,
		driver: d,
		goGets: make(map[string]struct{}),
	}, nil
}

func (ws workspace) Close() error {
	return os.RemoveAll(ws.dir)
}

// getPackage parses the package at packagePattern. This method is
// amortized, so repeated calls will use an in-memory cache.
// TODO: evict cache entries at some point...
func (ws *workspace) getPackages(packagePattern, version string) ([]*packages.Package, error) {
	if packagePattern == "" {
		return nil, fmt.Errorf("package path is empty")
	}

	pkgKey := packagePattern
	if version != "" {
		pkgKey += "@" + version
	}

	// if we've already processed this pattern, reuse it
	if cached := ws.driver.cachedPackages(pkgKey); len(cached) > 0 {
		return cached, nil
	}

	// as of Go 1.16, running "go get" is always required for module tooling to work
	// properly (https://golang.org/issue/40728) - only need to do it once per workspace
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if !ws.alreadyGotModule(packagePattern) {
		cmd := exec.Command("go", "get", pkgKey)
		cmd.Dir = ws.dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			return nil, fmt.Errorf("exec %v: %v", cmd.Args, err)
		}

		// remember that we 'go got' this package's module, so we don't have to do it again
		pkgInfo, err := runGoList(ws.dir, packagePattern)
		if err != nil {
			return nil, fmt.Errorf("listing package to get module: %v", err)
		}
		ws.goGets[pkgInfo.Module.Path] = struct{}{}
	}

	// finally, load and parse the package
	cfg := &packages.Config{
		Dir: ws.dir,
		Mode: packages.NeedSyntax |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedTypes |
			packages.NeedModule |
			packages.NeedTypesInfo,

		// on Linux, leaving CGO_ENABLED to the default value of 1 would
		// cause an error: "could not import C (no metadata for C)", but
		// only on Linux... on my Mac it worked fine either way (ca. 2020)
		Env: append(os.Environ(), "CGO_ENABLED=0"),
	}
	pkgs, err := packages.Load(cfg, packagePattern)
	if err != nil {
		return nil, fmt.Errorf("packages.Load: %v", err)
	}

	// generate and cache the list of top-level packages from the single input pattern;
	// this allows us to recall the parsed packages later without recomputing it all
	var pkgNames []string
	for _, pkg := range pkgs {
		pkgNames = append(pkgNames, packageKey(pkg))
	}
	// TODO: these should probably expire, esp. if using 'latest' or a branch name
	ws.driver.packagePatterns[pkgKey] = pkgNames

	// visit all packages (including imported ones) to cache them for future use,
	// (shaves a *ton* of time off future processing; core Caddy package goes from
	// taking 5 minutes to 5 seconds); and also to see if there are any errors in
	// the import graph
	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		// cache parsed package for future use; key by both the versioned and
		// non-versioned form of the package key, since future gets might not
		// have or know a version (not perfect, but no harm yet?)
		ws.driver.parsedPackages[pkg.ID] = pkg
		ws.driver.parsedPackages[packageKey(pkg)] = pkg

		// check for errors
		for i, e := range pkg.Errors {
			var prefix string
			if i > 0 {
				prefix = "\n"
			}
			log.Printf("[WARNING] Load '%s': found error while visiting package on import graph %s: %v - skipping",
				packagePattern, prefix, e)
		}
	})
	if err != nil {
		return nil, err
	}

	return pkgs, nil
}

func packageKey(pkg *packages.Package) string {
	pkgKey := pkg.ID
	if pkg.Module != nil && pkg.Module.Version != "" {
		pkgKey += "@" + pkg.Module.Version
	}
	return pkgKey
}

func (ws workspace) alreadyGotModule(packagePath string) bool {
	parts := strings.Split(packagePath, "/")
	for i := len(parts); i > 0; i-- {
		parent := strings.Join(parts[:i], "/")
		if _, ok := ws.goGets[parent]; ok {
			return true
		}
	}
	return false
}

func (ws workspace) representationBuilder() representationBuilder {
	return representationBuilder{
		ws:           ws,
		versionCache: make(map[string]string),
	}
}
