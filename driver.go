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
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"
)

// Driver is an instance of the Caddy documentation system.
// It should be a long-lived value that is reused over the
// lifetime of a server.
//
// An empty value is not valid; use New to obtain a valid value.
type Driver struct {
	db Storage

	// TODO: use this, there's probably a race on discoveredTypes
	mu sync.RWMutex

	// a cache of type definitions we've processed, keyed
	// by the type's fqtn@version string.
	discoveredTypes map[string]*Value
}

// New constructs a new documentation system.
func New(database Storage) *Driver {
	return &Driver{
		db:              database,
		discoveredTypes: make(map[string]*Value),
	}
}

// LoadModulesFromImportingPackage returns the Caddy modules (plugins) registered when
// package at its given version is imported.
func (d *Driver) LoadModulesFromImportingPackage(packagePattern, version string) ([]CaddyModule, error) {
	ws, err := d.openWorkspace()
	if err != nil {
		return nil, fmt.Errorf("opening workspace: %v", err)
	}
	defer ws.Close()

	pkgs, err := ws.getPackages(packagePattern, version)
	if err != nil {
		return nil, fmt.Errorf("loading package %s: %v", packagePattern, err)
	}

	rb := ws.representationBuilder()

	var allModules []CaddyModule

	var visitErr error
	packages.Visit(pkgs, func(pkg *packages.Package) bool {
		return visitErr == nil
	}, func(pkg *packages.Package) {
		pkgModules, err := rb.loadModulesFromSinglePackage(pkg)
		if err != nil {
			visitErr = err
			return
		}
		// TODO: remove duplicates?
		allModules = append(allModules, pkgModules...)
	})
	if visitErr != nil {
		return nil, visitErr
	}

	return allModules, nil
}

func (rb representationBuilder) loadModulesFromSinglePackage(pkg *packages.Package) ([]CaddyModule, error) {
	caddyModuleIdents, err := rb.ws.driver.findCaddyModuleIdents(pkg)
	if err != nil {
		return nil, err
	}

	var modules []CaddyModule
	for ident, caddyModName := range caddyModuleIdents {
		caddyModuleObj := pkg.TypesInfo.Uses[ident]

		rep, err := rb.buildRepresentation(caddyModuleObj.Type())
		if err != nil {
			return nil, err
		}

		typeName := localTypeName(caddyModuleObj.Type())

		modules = append(modules, CaddyModule{
			Name:           caddyModName,
			Representation: rep,
		})

		err = rb.ws.driver.db.SetCaddyModuleName(pkg, typeName, caddyModName)
		if err != nil {
			return nil, fmt.Errorf("saving Caddy module name to type: %v", err)
		}
	}
	return modules, nil
}

// AddType loads, parses, inspects, and stores the type representation for the given
// type in the given package. This is generally used for bootstrapping the docs with
// the initial/base Config type, within which all modules are used.
func (d *Driver) AddType(packageName, typeName, version string) (*Value, error) {
	ws, err := d.openWorkspace()
	if err != nil {
		return nil, fmt.Errorf("opening workspace: %v", err)
	}
	defer ws.Close()

	pkgs, err := ws.getPackages(packageName, version)
	if err != nil {
		return nil, fmt.Errorf("getting package %s: %v", packageName, err)
	}
	if len(pkgs) != 1 {
		return nil, fmt.Errorf("expected 1 package, but got %d from pattern '%s'", len(pkgs), packageName)
	}
	pkg := pkgs[0]

	obj := pkg.Types.Scope().Lookup(typeName)
	if obj == nil {
		return nil, fmt.Errorf("type %s not found in %s", typeName, packageName)
	}

	rep, err := ws.representationBuilder().buildRepresentation(obj.Type())
	if err != nil {
		return nil, fmt.Errorf("building representation of %s: %v", obj.Name(), err)
	}

	return rep, nil
}

// LoadTypeByPath loads the type representation at the given config path.
// It returns the exact value at that path and the nearest named type.
func (d *Driver) LoadTypeByPath(configPath, version string) (exact, nearest *Value, err error) {
	val, err := d.db.GetTypeByName(CaddyCorePackage, "Config", version)
	if err != nil {
		return nil, nil, fmt.Errorf("getting start type: %v", err)
	}
	if val == nil {
		return nil, nil, fmt.Errorf("start type not found")
	}
	exact, nearest, err = d.TraverseType(configPath, val)
	if err != nil {
		return nil, nil, fmt.Errorf("traversing type: %v", err)
	}
	exact, err = d.deepDereference(exact)
	if err != nil {
		return nil, nil, fmt.Errorf("dereferencing type path %s: %v", configPath, err)
	}
	return
}

// TraverseType traverses the start value according to path until the
// end of path is reached or the value is no longer traverseable, in
// which case it returns an error. On success, it returns the value
// at the given path, along with its nearest (containing) defined type.
func (d *Driver) TraverseType(path string, start *Value) (val, nearestType *Value, err error) {
	if start.Type == "" || start.TypeName == "" {
		return nil, nil, fmt.Errorf("must start at an actual type")
	}
	if path == "" {
		return start, start, nil
	}

	parts := ConfigPathParts(path)

	val = start
	nearestType = start

	for i := 0; i < len(parts); i++ {
		part := parts[i]

		// dereference this "pointer" (if it is one) to its actual type
		val, err = d.dereference(val)
		if err != nil {
			return nil, nil, fmt.Errorf("dereferencing type to %s: %v", val.SameAs, err)
		}

		// see if we can satisfy the next part with this type
	typeSwitch:
		switch val.Type {
		case Struct:
			for _, sf := range val.StructFields {
				if sf.Key == part {
					val = sf.Value
					if i == len(parts)-1 {
						// normally, the doc for the struct field would be irrelevant
						// while we traverse deeper in the structure, but if we're at
						// the target, we should include the struct field's docs, which
						// can provide crucial information that is otherwise missed
						if val.Doc != "" {
							val.Doc += "\n\n"
						}
						val.Doc += sf.Doc
					}
					break typeSwitch
				}
			}
			return nil, nil, fmt.Errorf("struct field '%s' not found at: %s",
				part, strings.Join(parts[:i], "/"))

		case Module, ModuleMap:
			caddyModuleID := part
			if val.ModuleNamespace != nil && *val.ModuleNamespace != "" {
				caddyModuleID = *val.ModuleNamespace + "." + part
			}
			var moduleInlineKey *string
			if i == len(parts)-1 {
				moduleInlineKey = val.ModuleInlineKey
			}
			vals, err := d.db.GetTypesByCaddyModuleID(caddyModuleID)
			if err != nil {
				return nil, nil, fmt.Errorf("loading type for module %s: %v", caddyModuleID, err)
			}
			val = vals[0] // TODO: support multiple values (two modules with same ID)... how? if in the middle, maybe find the one that matches; if at end...? maybe return a slice of them?
			val.ModuleInlineKey = moduleInlineKey

		case Map, Array:
			// container type; fallthrough to its element
			val = val.Elems
			i--

		default:
			return nil, nil, fmt.Errorf("%s: traversal not supported for type %#v",
				strings.Join(parts[:i], "/"), val)
		}

		// if this is an actual defined type, we need
		// to keep track of it so we can return it
		if val.TypeName != "" {
			nearestType = val
		}
	}

	return val, nearestType, nil
}

// LoadTypesByModuleID returns the type information for the Caddy module(s)
// with the given ID. It deeply dereferences the module(s) so that all type
// information and docs are included in the result.
func (d *Driver) LoadTypesByModuleID(moduleName string) ([]*Value, error) {
	vals, err := d.db.GetTypesByCaddyModuleID(moduleName)
	if err != nil {
		return nil, err
	}
	for i := range vals {
		vals[i], err = d.deepDereference(vals[i])
		if err != nil {
			return nil, fmt.Errorf("dereferencing module type %s: %v", vals[i].TypeName, err)
		}
	}
	return vals, nil
}

// CaddyModule represents a Caddy module.
type CaddyModule struct {
	Name           string `json:"module_name,omitempty"`
	Representation *Value `json:"structure,omitempty"`
}

// CaddyCorePackage is the import path of the Caddy core package.
const CaddyCorePackage = "github.com/caddyserver/caddy/v2"
