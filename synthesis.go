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
	"encoding/json"
	"fmt"
	"go/ast"
	"go/types"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/tools/go/ast/astutil"
)

type goListOutput struct {
	Dir        string `json:"Dir"`
	ImportPath string `json:"ImportPath"`
	Name       string `json:"Name"`
	Root       string `json:"Root"`
	Module     struct {
		Path    string `json:"Path"`
		Version string `json:"Version"`
		Replace struct {
			Path      string `json:"Path"`
			Dir       string `json:"Dir"`
			GoMod     string `json:"GoMod"`
			GoVersion string `json:"GoVersion"`
		} `json:"Replace"`
		Time      time.Time `json:"Time"`
		Dir       string    `json:"Dir"`
		GoMod     string    `json:"GoMod"`
		GoVersion string    `json:"GoVersion"`
	} `json:"Module"`
	Match          []string `json:"Match"`
	Stale          bool     `json:"Stale"`
	StaleReason    string   `json:"StaleReason"`
	GoFiles        []string `json:"GoFiles"`
	GoRoot         bool     `json:"Goroot"`
	Standard       bool     `json:"Standard"`
	IgnoredGoFiles []string `json:"IgnoredGoFiles"`
	Imports        []string `json:"Imports"`
	Deps           []string `json:"Deps"`
	TestGoFiles    []string `json:"TestGoFiles"`
	TestImports    []string `json:"TestImports"`
}

// getStructFieldGodocs gets the godoc for the struct fields in typ,
// and keys them by the field name. typ must be a named struct type.
func (rb representationBuilder) getStructFieldGodocs(typ types.Type) (map[string]string, error) {
	packagePath, typeName := typePackageAndName(typ)

	typeVersion, err := rb.getDepVersion(typ.(*types.Named))
	if err != nil {
		return nil, err
	}

	pkgs, err := rb.ws.getPackages(packagePath, typeVersion)
	if err != nil {
		return nil, err
	}
	if len(pkgs) != 1 {
		return nil, fmt.Errorf("expected 1 package, but got %d from pattern '%s'", len(pkgs), packagePath)
	}
	pkg := pkgs[0]

	fieldGodocs := make(map[string]string)
	var found bool

	for _, f := range pkg.Syntax {
		obj := f.Scope.Lookup(typeName)
		if obj == nil || obj.Decl == nil {
			continue
		}
		typeSpec, ok := obj.Decl.(*ast.TypeSpec)
		if !ok {
			continue
		}
		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			continue
		}
		found = true
		for _, field := range structType.Fields.List {
			if field.Doc == nil {
				continue
			}
			for _, fieldIdent := range field.Names {
				fieldGodocs[fieldIdent.Name] = field.Doc.Text()
			}
		}
	}
	if !found {
		return nil, fmt.Errorf("did not find struct type %s in %s", typeName, pkg.ID)
	}
	return fieldGodocs, nil
}

// getGodocForType returns the godoc for the given type.
func (rb representationBuilder) getGodocForType(typ types.Type) (string, error) {
	packagePath, typeName := typePackageAndName(typ)

	typeVersion, err := rb.getDepVersion(typ.(*types.Named))
	if err != nil {
		return "", err
	}

	pkgs, err := rb.ws.getPackages(packagePath, typeVersion)
	if err != nil {
		return "", err
	}
	if len(pkgs) != 1 {
		return "", fmt.Errorf("expected 1 package, but got %d from pattern '%s'", len(pkgs), packagePath)
	}
	pkg := pkgs[0]

	var foundObj bool
	for _, f := range pkg.Syntax {
		obj := f.Scope.Lookup(typeName)
		if obj == nil {
			continue
		}
		foundObj = true
		objPath, _ := astutil.PathEnclosingInterval(f, obj.Pos(), obj.Pos())
		for _, op := range objPath {
			// Types defined within a parenthesized `type (...)` block
			// and which have their own individual godoc will have
			// their godoc in a TypeSpec, otherwise the godoc will be
			// in a GenDecl. TypeSpec will usually be more specific,
			// since type blocks often have more than one type in them.
			if typespec, ok := op.(*ast.TypeSpec); ok &&
				typespec != nil &&
				typespec.Doc != nil {
				return typespec.Doc.Text(), nil
			}
			if gendecl, ok := op.(*ast.GenDecl); ok &&
				gendecl != nil &&
				gendecl.Doc != nil {
				return gendecl.Doc.Text(), nil
			}
		}
		break
	}

	if !foundObj {
		return "", fmt.Errorf("did not find type '%s' in '%s' from package path '%s'", typeName, pkg.ID, packagePath)
	}
	return "", nil
}

type representationBuilder struct {
	ws           workspace
	versionCache map[string]string
}

// buildRepresentation returns a structured representation of
// the given type, which we can use to put into our database
// and thus use to render documentation for the type.
func (rb representationBuilder) buildRepresentation(caddyModuleType types.Type) (*Value, error) {
	var rep *Value

	switch typ := caddyModuleType.(type) {
	case *types.Interface:
		return new(Value), nil
	case *types.Pointer:
		return rb.buildRepresentation(typ.Elem())

	case *types.Basic:
		switch typ.Kind() {
		case types.Bool:
			return &Value{Type: Bool}, nil
		case types.Int, types.Int8, types.Int16, types.Int32, types.Int64:
			return &Value{Type: Int}, nil
		case types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64, types.Uintptr:
			return &Value{Type: Uint}, nil
		case types.Float32, types.Float64:
			return &Value{Type: Float}, nil
		case types.Complex64, types.Complex128:
			return &Value{Type: Complex}, nil
		case types.String:
			return &Value{Type: String}, nil
		default:
			return nil, fmt.Errorf("unrecognized basic kind: %#v", caddyModuleType)
		}
	}

	switch typ := caddyModuleType.(type) {
	case *types.Named:
		typeVersion, err := rb.getDepVersion(typ)
		if err != nil {
			return nil, err
		}

		// if type has already been seen, return that
		fqtn := caddyModuleType.String() // all that matters is that this is unique
		sameAs := fqtn
		if typeVersion != "" {
			sameAs += "@" + typeVersion
		}
		if _, ok := rb.ws.driver.discoveredTypes[sameAs]; ok {
			return &Value{SameAs: sameAs}, nil
		}

		// if type has not already been seen but already exists in db, return that
		packagePath, typeName := typePackageAndName(caddyModuleType)
		discoveredType, err := rb.ws.driver.db.GetTypeByName(packagePath, typeName, typeVersion)
		if err != nil {
			return nil, err
		}
		if discoveredType != nil {
			rb.ws.driver.discoveredTypes[sameAs] = discoveredType
			return &Value{SameAs: discoveredType.TypeName}, nil
		}

		// a json.RawMessage type represents a module!
		if packagePath == "encoding/json" && typeName == "RawMessage" {
			return &Value{Type: Module}, nil
		}

		// otherwise, if this type is new, store it in the DB
		switch utyp := typ.Underlying().(type) {
		case *types.Struct:
			rep = &Value{Type: Struct}

			// load the godoc for the struct fields
			structFieldDocs, err := rb.getStructFieldGodocs(caddyModuleType)
			if err != nil {
				return nil, err
			}

			for i := 0; i < utyp.NumFields(); i++ {
				field := utyp.Field(i)

				if !field.Exported() {
					continue
				}

				// JSON field name from tag is required, but if the field
				// is embedded, it's OK if there isn't a JSON struct tag,
				// because when embedding a field it is often desirable
				// that such a field is a JSON-fallthrough
				jsonName, ok := jsonNameFromTag(utyp.Tag(i))
				if !ok || (jsonName == "" && !field.Embedded()) {
					continue
				}

				fieldRep, err := rb.buildRepresentation(field.Type())
				if err != nil {
					return nil, err
				}

				// get module information from the caddy struct tags
				ctf, err := caddyTagFields(utyp.Tag(i))
				if err != nil {
					return nil, err
				}
				modVal := fieldRep
				if fieldRep.Elems != nil {
					modVal = fieldRep.Elems
				}
				if moduleNamespace, ok := ctf["namespace"]; ok {
					modVal.ModuleNamespace = &moduleNamespace
				}
				if ModuleInlineKey, ok := ctf["inline_key"]; ok {
					modVal.ModuleInlineKey = &ModuleInlineKey
				}

				// embedded values act as if their fields were part of this type
				if field.Embedded() {
					embedded, err := rb.ws.driver.dereference(fieldRep)
					if err != nil {
						return nil, err
					}
					if embedded.Type == Struct {
						rep.StructFields = append(rep.StructFields, embedded.StructFields...)
					}
				} else {
					rep.StructFields = append(rep.StructFields, &StructField{
						Key:   jsonName,
						Value: fieldRep,
						Doc:   structFieldDocs[field.Name()],
					})
				}
			}

		default:
			rep, err = rb.buildRepresentation(typ.Underlying())
			if err != nil {
				return nil, err
			}
		}

		fullTypeName := fullyQualifiedTypeName(caddyModuleType)
		typeGodoc, err := rb.getGodocForType(caddyModuleType)
		if err != nil {
			return nil, err
		}
		rep.Doc = typeGodoc
		rep.TypeName = fullTypeName

		// remember this type so we don't have to re-assemble it all later
		rb.ws.driver.discoveredTypes[sameAs] = rep
		err = rb.ws.driver.db.StoreType(packagePath, typeName, typeVersion, rep)
		if err != nil {
			return nil, err
		}

		return &Value{SameAs: sameAs}, nil

	case *types.Struct:
		// very similar to above case, but this is an inlined, unnamed struct (can't get godoc without a name)
		rep := &Value{Type: Struct}

		for i := 0; i < typ.NumFields(); i++ {
			jsonName, ok := jsonNameFromTag(typ.Tag(i))
			if !ok {
				continue
			}
			fieldRep, err := rb.buildRepresentation(typ.Field(i).Type())
			if err != nil {
				return nil, err
			}

			// get module information from the caddy struct tags
			ctf, err := caddyTagFields(typ.Tag(i))
			if err != nil {
				return nil, err
			}
			modVal := fieldRep
			if fieldRep.Elems != nil {
				modVal = fieldRep.Elems
			}
			if moduleNamespace, ok := ctf["namespace"]; ok {
				modVal.ModuleNamespace = &moduleNamespace
			}
			if ModuleInlineKey, ok := ctf["inline_key"]; ok {
				modVal.ModuleInlineKey = &ModuleInlineKey
			}

			// correctly represent embedded fields inline with this parent type
			if typ.Field(i).Embedded() {
				if fieldRep.Type == Struct {
					rep.StructFields = append(rep.StructFields, fieldRep.StructFields...)
				}
			} else {
				rep.StructFields = append(rep.StructFields, &StructField{
					Key:   jsonName,
					Value: fieldRep,
				})
			}
		}
		return rep, nil

	case *types.Slice:
		elemRep, err := rb.buildRepresentation(typ.Elem())
		if err != nil {
			return nil, err
		}
		return &Value{Type: Array, Elems: elemRep}, nil

	case *types.Map:
		keyRep, err := rb.buildRepresentation(typ.Key())
		if err != nil {
			return nil, err
		}
		elemRep, err := rb.buildRepresentation(typ.Elem())
		if err != nil {
			return nil, err
		}
		if keyRep.Type == String && elemRep.Type == Module {
			return &Value{Type: ModuleMap}, nil
		}
		return &Value{Type: Map, MapKeys: keyRep, Elems: elemRep}, nil

	case *types.Interface:
		return nil, nil

	default:
		return nil, fmt.Errorf("unknown type %s: %#v", caddyModuleType.String(), caddyModuleType)
	}
}

func (rb *representationBuilder) getDepVersion(typ *types.Named) (string, error) {
	fieldTypePackageName, _ := typePackageAndName(typ.Obj().Type())
	if fieldTypePackageName == "" {
		// TODO: we could probably ignore this error, but let's see...
		return "", fmt.Errorf("unable to determine type's package")
	}

	// see if we already have the version cached (should be same as any parent packages)
	parts := strings.Split(fieldTypePackageName, "/")
	for i := len(parts); i > 0; i-- {
		parent := strings.Join(parts[:i], "/")
		if parentVersion, ok := rb.versionCache[parent]; ok {
			return parentVersion, nil
		}
	}

	// get the version of the module in use for this package in our workspace
	pkgInfo, err := runGoList(rb.ws.dir, fieldTypePackageName)
	if err != nil {
		return "", err
	}

	// cache for future use (shaves off a *LOT* of time)
	pathKey := pkgInfo.Module.Path
	if pkgInfo.Standard {
		// module version will be empty because it's a Go standard library type; oh well
		pathKey = pkgInfo.ImportPath
	}
	rb.versionCache[pathKey] = pkgInfo.Module.Version

	return pkgInfo.Module.Version, nil
}

func runGoList(workspaceDir, pkg string) (goListOutput, error) {
	pkg = strings.TrimSuffix(pkg, "/...")
	cmd := exec.Command("go", "list", "-json", pkg)
	cmd.Dir = workspaceDir
	results, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return goListOutput{}, fmt.Errorf("exec %v: %v; >>>>>>\n%s\n<<<<<<", cmd.Args, err, ee.Stderr)
		}
		return goListOutput{}, fmt.Errorf("exec %v: %v", cmd.Args, err)
	}
	var pkgInfo goListOutput
	err = json.Unmarshal(results, &pkgInfo)
	return pkgInfo, err
}

const caddyCorePackagePath = "github.com/caddyserver/caddy/v2"
