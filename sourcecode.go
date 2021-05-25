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
	"go/ast"
	"go/types"
	"log"
	"strings"

	"golang.org/x/tools/go/packages"
)

// findCaddyModuleIdents finds all caddy modules within the package  by traversing its
// AST. It looks for module registrations (which are calls to caddy.RegisterModule) and
// caddy.Module implementations (which are CaddyModule methods). Strictly speaking,
// module registrations are useless to us because they do not contain the module name:
// for that, we need to inspect the AST of the type's CaddyModule method; but we check
// for module registrations anyway because a caddy.Module that is not registered cannot
// be used (or at the very least, is inconsistent) so we return an error in that case.
//
// This function returns a map of type identifiers from the AST to their associated
// Caddy module IDs.
func (ds *Driver) findCaddyModuleIdents(pkg *packages.Package) (map[*ast.Ident]string, error) {
	caddyModRegs := make(map[string]*ast.Ident)
	caddyModImpls := make(map[string]*ast.Ident)
	caddyModIDs := make(map[string]string)

	for _, file := range pkg.Syntax {
		var inspectErr error
		var currentCaddyModuleFunc *ast.Ident
		ast.Inspect(file, func(node ast.Node) bool {
			switch val := node.(type) {
			case *ast.CallExpr:
				// function call; look for module registration which is
				// a call to caddy.RegisterModule()
				moduleReg, err := ds.findModuleRegistration(pkg, val)
				if err != nil {
					inspectErr = err
					return false
				}
				if moduleReg == nil {
					return true
				}
				caddyModRegs[moduleReg.Name] = moduleReg

			case *ast.FuncDecl:
				// function (or method) declaration; look for CaddyModule()
				// method, which implements the caddy.Module interface
				moduleImpl, err := ds.findModuleImpl(val)
				if err != nil {
					inspectErr = err
					return false
				}
				if moduleImpl == nil {
					return true
				}
				caddyModImpls[moduleImpl.Name] = moduleImpl
				currentCaddyModuleFunc = moduleImpl

			case *ast.ReturnStmt:
				// return statement; look for caddy.ModuleInfo struct so we
				// can extract the Caddy module name

				// only check if we are inside a CaddyModule() method right now
				if currentCaddyModuleFunc == nil {
					break
				}

				// expect exactly 1 return value
				if len(val.Results) != 1 {
					inspectErr = fmt.Errorf("expected exactly 1 return value from %#v, got %d", val, len(val.Results))
					return false
				}

				// it should be a composite literal (struct)
				compLit, ok := val.Results[0].(*ast.CompositeLit)
				if !ok {
					inspectErr = fmt.Errorf("expected composite literal return value from %#v; got %#v", val, val.Results[0])
					return false
				}

				// TODO: Maybe double-check that the type is specifically a caddy.ModuleInfo struct

				// peer inside its elements to get the name
				var caddyModName string
				for _, element := range compLit.Elts {
					kv, ok := element.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					if kv.Key.(*ast.Ident).Name == "ID" {
						// TODO: configadapters.go in the main caddy module has an unexported helper type called
						// adapterModule which implements CaddyModule interface, and its ID is computed, not static:
						// `caddy.ModuleID("caddy.adapters." + am.name)` - this is obviously problematic here...
						// but that's also a special case that real modules should not be having
						kvValueBasicLiteral, ok := kv.Value.(*ast.BasicLit)
						if !ok {
							log.Printf("[WARNING] CaddyModule() method in %s returns ModuleInfo with unsupported ID value (must be a static literal value); skipping: %#v", file.Name, kv.Value)
							delete(caddyModRegs, currentCaddyModuleFunc.Name)
							delete(caddyModImpls, currentCaddyModuleFunc.Name)
							currentCaddyModuleFunc = nil
							return true
						}

						// TODO: What if the module name is pulled out to a constant? do we need to evaluate it?
						rawString := kvValueBasicLiteral.Value
						caddyModName = strings.Trim(rawString, `"`)
						break
					}
				}

				if caddyModName == "" {
					inspectErr = fmt.Errorf("found module info, but missing module name: %#v", compLit)
					return false
				}

				// associate the caddy module name with the type name
				caddyModIDs[currentCaddyModuleFunc.Name] = caddyModName
				currentCaddyModuleFunc = nil
			}

			return true
		})
		if inspectErr != nil {
			return nil, inspectErr
		}
	}

	// see if any caddy module types are implemented but
	// not registered, and vice-versa
	for key, val := range caddyModRegs {
		if _, ok := caddyModImpls[key]; !ok {
			return nil, fmt.Errorf("caddy module gets registered but does not implement caddy.Module interface: %#v", val)
		}
		if _, ok := caddyModIDs[key]; !ok {
			return nil, fmt.Errorf("caddy module gets registered, but we could not find its module name: %#v", val)
		}
	}
	for key, val := range caddyModImpls {
		if _, ok := caddyModRegs[key]; !ok {
			return nil, fmt.Errorf("type has CaddyModule method, but does not get registered via caddy.%s(): %#v", registerModule, val)
		}
		if _, ok := caddyModIDs[key]; !ok {
			return nil, fmt.Errorf("type has CaddyModule method, but we could not find its module name: %#v", val)
		}
	}

	// the contents of all maps should now be consistent, so finally
	// pair each type identifier with its caddy module name
	mods := make(map[*ast.Ident]string)
	for typeName, ident := range caddyModRegs {
		mods[ident] = caddyModIDs[typeName]
	}

	return mods, nil
}

// findModuleRegistration returns an AST identifier for a type
// that is registered using fnCall. If fnCall is not a call to
// caddy.RegisterModule, nil is returned.
func (ds *Driver) findModuleRegistration(pkg *packages.Package, fnCall *ast.CallExpr) (*ast.Ident, error) {
	// this could be any function call; make sure it's
	// actually a call to register a module
	switch fn := fnCall.Fun.(type) {
	case *ast.Ident:
		// in the core caddy package, i.e. `RegisterModule(...)`
		if fn.Name != registerModule {
			return nil, nil
		}
	case *ast.SelectorExpr:
		// outside of core caddy package, i.e. `caddy.RegisterModule(...)`
		if fn.Sel.Name != registerModule {
			return nil, nil
		}

		// make sure the selector's field expression
		// resolves to the actual caddy package
		x, ok := fn.X.(*ast.Ident)
		if !ok {
			return nil, nil
		}
		if pkgName, ok := pkg.TypesInfo.Uses[x].(*types.PkgName); ok {
			importedPkg := pkgName.Imported()
			if importedPkg.Path() != caddyCorePackagePath {
				return nil, fmt.Errorf("%s call does not resolve to %s; resolves to: %s",
					registerModule, caddyCorePackagePath, importedPkg.Path())
			}
		}
	default:
		return nil, nil
	}

	if len(fnCall.Args) != 1 {
		return nil, fmt.Errorf("wrong number of arguments to %s: %d (expected 1)",
			registerModule, len(fnCall.Args))
	}

	var caddyModuleIdent *ast.Ident

	switch val := fnCall.Args[0].(type) {
	case *ast.CompositeLit:
		// happens with `caddy.RegisterModule(Gizmo{})`
		caddyModuleIdent = val.Type.(*ast.Ident)

	case *ast.CallExpr:
		// happens with `caddy.RegisterModule(new(Gizmo))`
		if funIdent, ok := val.Fun.(*ast.Ident); ok && funIdent.Name == "new" {
			caddyModuleIdent = val.Args[0].(*ast.Ident)
		} else {
			return nil, fmt.Errorf("unknown function call in %s(): %#v - only support new()",
				registerModule, val.Fun)
		}
	default:
		return nil, fmt.Errorf("unexpected argument to %s(): %#v - expect either composite literal or new()",
			registerModule, val)
	}

	return caddyModuleIdent, nil
}

// findModuleImpl returns a type identifier if fnDecl implements
// the caddy.Module interface; otherwise, nil is returned.
func (ds *Driver) findModuleImpl(fnDecl *ast.FuncDecl) (*ast.Ident, error) {
	// must be named "CaddyModule"
	if fnDecl.Name.Name != "CaddyModule" {
		return nil, nil
	}

	// must be a method, i.e. it must have a receiver
	if fnDecl.Recv == nil || len(fnDecl.Recv.List) != 1 {
		return nil, nil
	}

	// TODO: check return type, make sure it returns a caddy.ModuleInfo

	var receiver *ast.Ident
	switch val := fnDecl.Recv.List[0].Type.(type) {
	case *ast.Ident:
		receiver = val
	case *ast.StarExpr:
		receiver = val.X.(*ast.Ident)
	default:
		return nil, fmt.Errorf("expected identifier or pointer for receiver type, but got %#v", fnDecl.Recv.List[0].Type)
	}

	return receiver, nil
}

// Value describes a config value. *Technically* it actually describes
// a type, but since our purpose is documentation, all uses of the
// type end up becoming values, so from the user's perspective, they
// are values, even though though they are only plausible/template
// values derived from types in source code.
type Value struct {
	// Indicates the fundamental type of the value.
	Type Type `json:"type,omitempty"`

	// The local name of the type from the source code.
	TypeName string `json:"type_name,omitempty"`

	// For struct types, these are the struct fields.
	StructFields []*StructField `json:"struct_fields,omitempty"`

	// For map types, this describes the map keys.
	MapKeys *Value `json:"map_keys,omitempty"`

	// For map types, this describes the map values.
	// For array types, this describes the array elements.
	Elems *Value `json:"elems,omitempty"`

	// The documentation as found from the source code's godoc.
	Doc string `json:"doc,omitempty"`

	// If this value's type is the reuse of an existing
	// named type for which we already have the structure
	// documented, SameAs contains the fully-qualified
	// type name and possibly version (fqtn@version).
	SameAs string `json:"same_as,omitempty"`

	// If this value is fulfilled by a Caddy module,
	// this should be the module's namespace.
	ModuleNamespace *string `json:"module_namespace,omitempty"`

	// If this value is fulfilled by a Caddy module and
	// is configured by declaring the module's name inline
	// with its struct, this is the name of the key with
	// which the module name is specified.
	ModuleInlineKey *string `json:"module_inline_key,omitempty"`
}

// StructField contains information about a struct field.
type StructField struct {
	Key   string `json:"key"`
	Value *Value `json:"value"`
	Doc   string `json:"doc,omitempty"`
}

// Type represents a funamdental type. Recognized
// values are defined as constants in this package.
// Most of the types are Go primitives, but a few
// are more complex types that we want/need to handle
// properly for documentation purposes. For example,
// it's obvious that we need a string type, but we
// also need a struct type so that we can render
// a struct's fields. Ultimately, all named types
// are expected to boil down to the types we
// recognize so we can express them properly in
// the generated documentation.
type Type string

// Possible types that are recognized by
// the documentation system.
const (
	// Go primitives
	Bool    Type = "bool"
	Int     Type = "int"
	Uint    Type = "uint"
	Float   Type = "float"
	Complex Type = "complex"
	String  Type = "string"

	// Complex or structural types
	Struct Type = "struct"
	Array  Type = "array"
	Map    Type = "map"

	// Caddy-specific types
	Module    Type = "module"
	ModuleMap Type = "module_map"
)

// registerModule is the name of the function that registers modules.
const registerModule = "RegisterModule"
