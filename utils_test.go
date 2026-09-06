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
	"testing"
)

func TestTypePackageAndName_UnwrapsAlias(t *testing.T) {
	pkg := loadAliasesFixture(t)

	aliasType := lookupType(t, pkg, "SettingsAlias")
	if _, ok := aliasType.(*types.Alias); !ok {
		t.Fatalf("fixture regression: SettingsAlias is %T, expected *types.Alias (Go 1.23+ default)", aliasType)
	}

	gotPkg, gotName := typePackageAndName(aliasType)
	if gotPkg != pkg.PkgPath {
		t.Errorf("pkg path = %q, want %q", gotPkg, pkg.PkgPath)
	}
	if gotName != "Settings" {
		t.Errorf("type name = %q, want %q", gotName, "Settings")
	}
}

func TestTypePackageAndName_NonNamedStaysEmpty(t *testing.T) {
	gotPkg, gotName := typePackageAndName(types.Typ[types.String])
	if gotPkg != "" || gotName != "" {
		t.Errorf("expected empty pkg/name for basic type, got (%q,%q)", gotPkg, gotName)
	}
}

func TestLocalTypeName_UnwrapsAlias(t *testing.T) {
	pkg := loadAliasesFixture(t)

	aliasType := lookupType(t, pkg, "SettingsAlias")
	if got := localTypeName(aliasType); got != "Settings" {
		t.Errorf("local type name = %q, want %q", got, "Settings")
	}
}
