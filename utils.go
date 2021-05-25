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
	"go/types"
	"reflect"
	"strings"

	"github.com/caddyserver/caddy/v2"
)

// SplitLastDot splits input into two strings at the last dot.
// If there is no dot, then before will be empty string and after
// will be the input. Examples:
//
//     "github.com/caddyserver/caddy/v2.Config" => ("github.com/caddyserver/caddy/v2", "Config")
//     "http.handlers.file_server"              => ("http.handlers", "file_server")
//     "http"                                   => ("", "http")
//
func SplitLastDot(input string) (before, after string) {
	lastDot := strings.LastIndex(input, ".")
	if lastDot < 0 {
		return "", input
	}
	before = input[:lastDot]
	after = input[lastDot+1:]
	return
}

// ConfigPathParts splits configPath by its separator, the forward
// slash (/). It also trims leading and trailing slashes.
func ConfigPathParts(configPath string) []string {
	return strings.Split(strings.Trim(configPath, "/"), "/")
}

// jsonNameFromTag takes as input the value of an entire struct
// tag and returns the JSON name of the field, and true if the
// name is not "-". If the name is "-" (field ignored/excluded
// by the encoding/json package), then false is returned.
func jsonNameFromTag(tagStr string) (string, bool) {
	jsonName := reflect.StructTag(tagStr).Get("json")
	if commaIdx := strings.Index(jsonName, ","); commaIdx > 0 {
		jsonName = strings.TrimSpace(jsonName[:commaIdx])
	}
	if jsonName == "-" {
		return "", false
	}
	return jsonName, true
}

// caddyTagFields parses tagStr which is expected to be the
// value of an entire struct tag, and returns the individual
// key-value pairs of the "caddy:" tag.
func caddyTagFields(tagStr string) (map[string]string, error) {
	caddyTag := reflect.StructTag(tagStr).Get("caddy")
	return caddy.ParseStructTag(caddyTag)
}

// fullyQualifiedTypeName returns the fully-qualified
// type name of typ. It must be a named type.
func fullyQualifiedTypeName(typ types.Type) string {
	pkgPath, typeName := typePackageAndName(typ)
	if pkgPath != "" && typeName != "" {
		return pkgPath + "." + typeName
	}
	return typ.String()
}

// typeAndPackageName returns the fully-qualified package
// name and the local type name of typ. It must be a named
// type.
func typePackageAndName(typ types.Type) (pkgPath, typeName string) {
	if nt, ok := typ.(*types.Named); ok {
		// TODO: should be Pkg().Name() instead?
		return nt.Obj().Pkg().Path(), nt.Obj().Name()
	}
	return "", ""
}

// localTypeName returns the local type name of typ,
// which must be a named type.
func localTypeName(typ types.Type) string {
	if nt, ok := typ.(*types.Named); ok {
		return nt.Obj().Name()
	}
	return ""
}
