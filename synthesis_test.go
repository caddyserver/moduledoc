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
	"strings"
	"testing"
)

func TestGetStructFieldGodocs_AliasFieldType(t *testing.T) {
	pkg := loadAliasesFixture(t)
	rb := newTestBuilder(t, pkg)

	configType := lookupType(t, pkg, "Config")

	docs, err := rb.getStructFieldGodocs(configType)
	if err != nil {
		t.Fatalf("getStructFieldGodocs: %v", err)
	}

	for _, field := range []string{"Nested", "Label", "Items"} {
		if !strings.Contains(docs[field], field) {
			t.Errorf("field %q doc = %q, want it to mention the field name", field, docs[field])
		}
	}
}

// Passing a *types.Alias directly used to panic with the unchecked
// t.(*types.Named) assertion on Go 1.23+ when gotypesalias=1.
func TestGetStructFieldGodocs_AliasDirect(t *testing.T) {
	pkg := loadAliasesFixture(t)
	rb := newTestBuilder(t, pkg)

	docs, err := rb.getStructFieldGodocs(lookupType(t, pkg, "SettingsAlias"))
	if err != nil {
		t.Fatalf("getStructFieldGodocs(SettingsAlias): %v", err)
	}
	if !strings.Contains(docs["Name"], "plain string field") {
		t.Errorf("field 'Name' doc = %q, want the underlying Settings.Name doc", docs["Name"])
	}
}

func TestGetGodocForType_AliasedTargetType(t *testing.T) {
	pkg := loadAliasesFixture(t)
	rb := newTestBuilder(t, pkg)

	settingsType := lookupType(t, pkg, "Settings")

	doc, err := rb.getGodocForType(settingsType)
	if err != nil {
		t.Fatalf("getGodocForType: %v", err)
	}
	if !strings.Contains(doc, "Settings holds nested configuration") {
		t.Errorf("godoc = %q, want it to contain the Settings doc line", doc)
	}
}

// Passing a *types.Alias directly used to panic with the unchecked
// t.(*types.Named) assertion on Go 1.23+ when gotypesalias=1.
func TestGetGodocForType_AliasDirect(t *testing.T) {
	pkg := loadAliasesFixture(t)
	rb := newTestBuilder(t, pkg)

	doc, err := rb.getGodocForType(lookupType(t, pkg, "SettingsAlias"))
	if err != nil {
		t.Fatalf("getGodocForType(SettingsAlias): %v", err)
	}
	if !strings.Contains(doc, "Settings holds nested configuration") {
		t.Errorf("godoc = %q, want the underlying Settings godoc after unalias", doc)
	}
}

func TestGetGodocForType_RejectsNonNamed(t *testing.T) {
	pkg := loadAliasesFixture(t)
	rb := newTestBuilder(t, pkg)

	// A *types.Basic reaches this path if a caller forgets to filter primitives.
	_, err := rb.getGodocForType(lookupType(t, pkg, "Config").Underlying())
	if err == nil {
		t.Fatal("expected error for non-named type, got nil")
	}
}

func TestBuildRepresentation_AliasStructField(t *testing.T) {
	pkg := loadAliasesFixture(t)
	rb := newTestBuilder(t, pkg)

	configType := lookupType(t, pkg, "Config")

	val, err := rb.buildRepresentation(configType)
	if err != nil {
		t.Fatalf("buildRepresentation(Config): %v", err)
	}

	// buildRepresentation returns a SameAs pointer; the actual structured
	// value lives on the driver's discoveredTypes cache.
	if val.SameAs == "" {
		t.Fatalf("expected SameAs to be set, got %+v", val)
	}
	rep := rb.ws.driver.discoveredTypes[val.SameAs]
	if rep == nil {
		t.Fatalf("discoveredTypes missing entry for %q", val.SameAs)
	}
	if rep.Type != Struct {
		t.Fatalf("Config.Type = %q, want %q", rep.Type, Struct)
	}

	fields := map[string]*StructField{}
	for _, f := range rep.StructFields {
		fields[f.Key] = f
	}

	nested, ok := fields["nested"]
	if !ok {
		t.Fatal("missing struct field 'nested' (alias-to-struct)")
	}
	if nested.Value.SameAs == "" {
		t.Errorf("nested.SameAs empty; alias likely fell through to 'unknown type' branch")
	}
	if !strings.HasSuffix(nested.Value.SameAs, ".Settings") {
		t.Errorf("nested.SameAs = %q, want it to point at the aliased target Settings", nested.Value.SameAs)
	}

	label, ok := fields["label"]
	if !ok {
		t.Fatal("missing struct field 'label' (alias-to-primitive)")
	}
	if label.Value.Type != String {
		t.Errorf("label.Type = %q, want %q", label.Value.Type, String)
	}

	items, ok := fields["items"]
	if !ok {
		t.Fatal("missing struct field 'items' (slice of alias)")
	}
	if items.Value.Type != Array {
		t.Errorf("items.Type = %q, want %q", items.Value.Type, Array)
	}
	if items.Value.Elems == nil || items.Value.Elems.SameAs == "" {
		t.Errorf("items.Elems missing SameAs; alias slice element likely fell through to 'unknown type' branch: %+v", items.Value.Elems)
	}
}
