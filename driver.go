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

	"golang.org/x/tools/go/packages"
)

// Driver is an instance of the Caddy documentation system.
// It should be a long-lived value that is reused over the
// lifetime of a server.
//
// An empty value is not valid; use New to obtain a valid value.
type Driver struct {
	db              Storage
	parsedPackages  map[string]*packages.Package
	discoveredTypes map[string]*Value
}

// New constructs a new documentation system.
func New(database Storage) *Driver {
	return &Driver{
		db:              database,
		parsedPackages:  make(map[string]*packages.Package),
		discoveredTypes: make(map[string]*Value),
	}
}

// LoadModulesFromPackage loads and stores all Caddy modules found in the package
// at the given fully-qualified package path.
func (d *Driver) LoadModulesFromPackage(packagePath, version string) ([]CaddyModule, error) {
	pkg, err := d.getPackage(packagePath, version)
	if err != nil {
		return nil, fmt.Errorf("loading package %s: %v", packagePath, err)
	}

	caddyModuleIdents, err := d.findCaddyModuleIdents(pkg)
	if err != nil {
		return nil, err
	}

	var modules []CaddyModule
	for ident, caddyModName := range caddyModuleIdents {
		caddyModuleObj := pkg.TypesInfo.Uses[ident]

		rb := representationBuilder{driver: d, baseGoModule: packagePath, goModuleVersion: version}
		rep, err := rb.buildRepresentation(caddyModuleObj.Type())
		if err != nil {
			return nil, err
		}

		typeName := localTypeName(caddyModuleObj.Type())
		rep.Doc = refineDoc(rep.Doc, typeName, caddyModName)

		modules = append(modules, CaddyModule{
			Name:           caddyModName,
			Representation: rep,
		})

		err = d.db.SetCaddyModuleName(packagePath, typeName, caddyModName)
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
	pkg, err := d.getPackage(packageName, version)
	if err != nil {
		return nil, fmt.Errorf("getting package %s: %v", packageName, err)
	}

	obj := pkg.Types.Scope().Lookup(typeName)
	if obj == nil {
		return nil, fmt.Errorf("type %s not found in %s", typeName, packageName)
	}

	rb := representationBuilder{driver: d, baseGoModule: packageName, goModuleVersion: version}
	rep, err := rb.buildRepresentation(obj.Type())
	if err != nil {
		return nil, fmt.Errorf("building representation of %s: %v", obj.Name(), err)
	}

	return rep, nil
}

// LoadTypeByPath loads the type representation at the given config path.
// It returns the exact value at that path and the nearest named type.
func (d *Driver) LoadTypeByPath(configPath string) (exact, nearest *Value, err error) {
	val, err := d.db.GetTypeByName("github.com/caddyserver/caddy/v2", "Config")
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
			val, err = d.db.GetTypeByCaddyModuleID(caddyModuleID)
			if err != nil {
				return nil, nil, fmt.Errorf("loading type for module %s: %v", caddyModuleID, err)
			}
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

// LoadTypeByModuleID returns the type information for the Caddy module
// with the given ID. It deeply dereferences the module so that all
// type information and docs are included in the result.
func (d *Driver) LoadTypeByModuleID(moduleName string) (*Value, error) {
	val, err := d.db.GetTypeByCaddyModuleID(moduleName)
	if err != nil {
		return nil, err
	}
	val, err = d.deepDereference(val)
	if err != nil {
		return nil, fmt.Errorf("dereferencing module type %s: %v", val.TypeName, err)
	}
	return val, nil
}

// GetAllModules returns a map of all modules, keyed by the
// full module ID.
func (d *Driver) GetAllModules() (map[string]Value, error) {
	return d.db.GetAllModules()
}

// CaddyModule represents a Caddy module.
type CaddyModule struct {
	Name           string `json:"module_name,omitempty"`
	Representation *Value `json:"structure,omitempty"`
}
