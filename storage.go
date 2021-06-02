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

// Storage describes the methods necessary for a documentation driver to
// be able to store and lookup type and value information.
type Storage interface {
	// GetTypeByName returns a type by its type name, comprising a
	// package path and the identifier/name within that package.
	GetTypeByName(packagePath, name, version string) (*Value, error)

	// GetTypesByCaddyModuleID returns matching types by the Caddy module ID.
	// (Caddy module IDs are not necessarily globally unique.)
	GetTypesByCaddyModuleID(caddyModuleID string) ([]*Value, error)

	// StoreType stores a type with the given package path and type name.
	StoreType(packagePath, typeName, version string, rep *Value) error

	// SetCaddyModuleName sets the module name for the type with the
	// given package and type name.
	SetCaddyModuleName(pkg *packages.Package, typeName, modName string) error
}

// dereference follows val.SameAs and returns the
// value that is pointed to by val.SameAs. The
// ModuleNamespace and ModuleInlineKey information is
// preserved in the returned value. If val.SameAs is
// empty string, val is returned and this is a no-op.
func (ds *Driver) dereference(val *Value) (*Value, error) {
	// if there is no equivalent type, nothing to dereference
	if val.SameAs == "" {
		return val, nil
	}

	// load the referenced type
	parts := strings.SplitN(val.SameAs, "@", 2)
	fqtn, version := parts[0], ""
	if len(parts) == 2 {
		version = parts[1]
	}
	typ, err := ds.getTypeByFullName(fqtn, version)
	if err != nil {
		return nil, err
	}
	if typ == nil {
		return nil, fmt.Errorf("dereference failed, type not found: %s@%s", fqtn, version)
	}

	// transfer over the module namespace and inline key, since that
	// information is specific to the context in which the type appears,
	// thus the normalized stored type will not have that information;
	// but first we have to dive down through maps and arrays until we
	// are not at a map or array anymore, so that the information is
	// in the relevant spot in the structure
	moduleElem := typ
	for moduleElem.Elems != nil {
		moduleElem = moduleElem.Elems
	}
	moduleElem.ModuleNamespace = val.ModuleNamespace
	moduleElem.ModuleInlineKey = val.ModuleInlineKey

	// it is also useful to combine the type's godoc with the parent's.
	if val.Doc != "" {
		typ.Doc = val.Doc + "\n\n" + typ.Doc
	}

	return typ, nil
}

// deepDereference calls ds.dereference, but recursively,
// for val and all struct fields or map/array elems of val.
// As a result, the returned value information is completely
// dereferenced and filled out.
func (ds *Driver) deepDereference(val *Value) (*Value, error) {
	var err error
	val, err = ds.dereference(val)
	if err != nil {
		return nil, err
	}

	// dereference all struct fields
	for _, sf := range val.StructFields {
		sf.Value, err = ds.deepDereference(sf.Value)
		if err != nil {
			return nil, err
		}

		// Append the struct field's type godoc to the struct field's godoc.
		// The struct field's godoc can give context and describe how this
		// particular instance of the type is used, but the type's godoc is
		// still important because it describes the type's universal usage.
		// If it's a map or array, get the underlying type that has docs.
		underlyingTypeVal := sf.Value
		if sf.Value.Elems != nil {
			underlyingTypeVal = sf.Value.Elems
		}

		// prepend the struct field doc (it is usually more specific, and
		// a better introduction to what the value is, than the type's doc)
		// to the type doc
		if sf.Doc != "" {
			underlyingTypeVal.Doc = strings.TrimSpace(sf.Doc + "\n\n" + underlyingTypeVal.Doc)
		}

		if underlyingTypeVal.Doc != "" {
			sf.Doc = underlyingTypeVal.Doc
		}
	}

	// dereference all map keys
	if val.MapKeys != nil {
		val.MapKeys, err = ds.deepDereference(val.MapKeys)
		if err != nil {
			return nil, err
		}
	}

	// dereference all map or array elements
	if val.Elems != nil {
		val.Elems, err = ds.deepDereference(val.Elems)
		if err != nil {
			return nil, err
		}
	}

	return val, nil
}

// getTypeByFullName gets the type representation for the given type
// by its fully-qualified type name and version.
func (ds *Driver) getTypeByFullName(fqtn, version string) (*Value, error) {
	pkgName, typeName := SplitLastDot(fqtn)
	return ds.db.GetTypeByName(pkgName, typeName, version)
}
