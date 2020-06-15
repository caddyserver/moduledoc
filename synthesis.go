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
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

// getPackage parses the package at packagePath. This method is
// amortized, so repeated calls will use an in-memory cache.
// TODO: expire cache entries after some amount of time
func (ds *Driver) getPackage(packagePath, version string) (*packages.Package, error) {
	if packagePath == "" {
		return nil, fmt.Errorf("package path is empty")
	}
	// TODO: help, haha
	// if version == "" {
	// 	log.Printf("[WARNING] Go package %s: version is empty; using 'latest'", packagePath)
	// 	version = "latest"
	// }

	// TODO: these should probably expire, esp. if using 'latest'
	if pkg, ok := ds.parsedPackages[packagePath+"@"+version]; ok {
		return pkg, nil
	}

	// set up go.mod in a new temporary folder so that the
	// x/tools/go/packages package can run 'go list' in a
	// way that honors the desired version, if set
	tempDir, err := ioutil.TempDir("", "caddy_docsys_")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	cmd := exec.Command("go", "mod", "init", "temp/docsys")
	cmd.Dir = tempDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("exec %v: %v", cmd.Args, err)
	}
	if version != "" {
		cmd = exec.Command("go", "get", packagePath+"@"+version)
		cmd.Dir = tempDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return nil, fmt.Errorf("exec %v: %v", cmd.Args, err)
		}
	}

	// finally, load and parse the package
	cfg := &packages.Config{
		Dir: tempDir,
		Mode: packages.NeedSyntax |
			packages.NeedImports |
			packages.NeedTypes |
			packages.NeedTypesInfo,
	}
	pkgs, err := packages.Load(cfg, packagePath)
	if err != nil {
		return nil, err
	}

	// see if there are any errors in the import graph
	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		for i, e := range pkg.Errors {
			var prefix string
			if i > 0 {
				prefix = "\n"
			}
			err = fmt.Errorf("%v%s%s", err, prefix, e.Error())
		}
	})
	if err != nil {
		return nil, err
	}
	if len(pkgs) != 1 {
		return nil, fmt.Errorf("expected 1 package, but got %d", len(pkgs))
	}
	pkg := pkgs[0]

	ds.parsedPackages[packagePath+"@"+version] = pkg

	return pkg, nil
}

// getStructFieldGodocs gets the godoc for the struct fields in typ,
// and keys them by the field name. typ must be a named struct type.
func (rb representationBuilder) getStructFieldGodocs(typ types.Type) (map[string]string, error) {
	packagePath, typeName := typePackageAndName(typ)

	// TODO: version is not known here; as we traverse a type, we
	// could pass into different packages (go modules) which have
	// different versions... is that going to be a problem?
	var version string
	if packagePath == rb.baseGoModule {
		version = rb.goModuleVersion
	}
	pkg, err := rb.driver.getPackage(packagePath, version)
	if err != nil {
		return nil, err
	}

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
		return nil, fmt.Errorf("did not find struct type %s in %s", typeName, pkg.Name)
	}
	return fieldGodocs, nil
}

// refineDoc transforms godoc slightly so it is more suitable
// for display to the client; for example, it replaces the Go
// type name (WhichHasCaps) to the JSON name (which_has_underscores)
// so it is more recognizable.
// TODO: This could probably be a lot better.
func refineDoc(doc, oldName, newName string) string {
	if oldName == "" || newName == "" {
		return doc
	}
	_, oldName = SplitLastDot(oldName)
	doc = strings.TrimSpace(doc)
	if strings.HasPrefix(doc, oldName) {
		doc = strings.Replace(doc, oldName, newName, 1)
	}
	return doc
}

// getGodocForType returns the godoc for the given type.
func (rb representationBuilder) getGodocForType(typ types.Type) (string, error) {
	packagePath, typeName := typePackageAndName(typ)

	// TODO: version is not known here; as we traverse a type, we
	// could pass into different packages (go modules) which have
	// different versions... is that going to be a problem?
	var version string
	if packagePath == rb.baseGoModule {
		version = rb.goModuleVersion
	}
	pkg, err := rb.driver.getPackage(packagePath, version)
	if err != nil {
		return "", err
	}

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
		return "", fmt.Errorf("did not find type '%s' in '%s' from package path '%s'", typeName, pkg.Name, packagePath)
	}
	return "", nil
}

type representationBuilder struct {
	driver          *Driver
	baseGoModule    string
	goModuleVersion string
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
		// if type has already been seen, return that
		fqtn := caddyModuleType.String() // all that matters is that this is unique
		if _, ok := rb.driver.discoveredTypes[fqtn]; ok {
			return &Value{SameAs: fqtn}, nil
		}

		// if type has not already been seen but already exists in db, return that
		packagePath, typeName := typePackageAndName(caddyModuleType)
		discoveredType, err := rb.driver.db.GetTypeByName(packagePath, typeName)
		if err != nil {
			return nil, err
		}
		if discoveredType != nil {
			rb.driver.discoveredTypes[fqtn] = discoveredType
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
					embedded, err := rb.driver.dereference(fieldRep)
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
						Doc:   refineDoc(structFieldDocs[field.Name()], field.Name(), jsonName),
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
		rb.driver.discoveredTypes[fqtn] = rep
		err = rb.driver.db.StoreType(packagePath, typeName, rep)
		if err != nil {
			return nil, err
		}

		return &Value{SameAs: rep.TypeName}, nil

	case *types.Struct:
		// very similar to above case, but this is an unnamed struct (can't get godoc without a name)
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

const caddyCorePackagePath = "github.com/caddyserver/caddy/v2"
