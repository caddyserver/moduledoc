package moduledoc

import (
	"go/types"
	"reflect"
	"testing"
)

func TestSplitLastDotEdgeCases(t *testing.T) {
	for _, tc := range []struct {
		input, before, after string
	}{
		{"github.com/caddyserver/caddy/v2.Config", "github.com/caddyserver/caddy/v2", "Config"},
		{"http.handlers.file_server", "http.handlers", "file_server"},
		{"http", "", "http"},
		{"", "", ""},
		{".", "", ""},
		{"trailing.", "trailing", ""},
		{".leading", "", "leading"},
		{"a.b.c.d", "a.b.c", "d"},
		{"..", ".", ""},
	} {
		before, after := SplitLastDot(tc.input)
		if before != tc.before || after != tc.after {
			t.Errorf("SplitLastDot(%q) = (%q, %q); want (%q, %q)",
				tc.input, before, after, tc.before, tc.after)
		}
	}
}

func TestConfigPathPartsEdgeCases(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  []string
	}{
		{"apps/http/servers", []string{"apps", "http", "servers"}},
		{"/apps/http/", []string{"apps", "http"}},
		{"apps", []string{"apps"}},
		// degenerate inputs collapse to a single empty part
		{"", []string{""}},
		{"/", []string{""}},
		{"//", []string{""}},
		// interior empty segments are preserved
		{"a//b", []string{"a", "", "b"}},
	} {
		got := ConfigPathParts(tc.input)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ConfigPathParts(%q) = %#v; want %#v", tc.input, got, tc.want)
		}
	}
}

func TestJSONNameFromTagEdgeCases(t *testing.T) {
	for _, tc := range []struct {
		tag     string
		name    string
		include bool
	}{
		{`json:"listen"`, "listen", true},
		{`json:"listen,omitempty"`, "listen", true},
		{`json:"listen, omitempty"`, "listen", true},
		{`json:"-"`, "", false},
		// "-," means the field is named "-", not excluded
		{`json:"-,"`, "-", true},
		// empty name with options means no explicit JSON name
		{`json:",omitempty"`, "", true},
		{`json:""`, "", true},
		{``, "", true},
		{`yaml:"listen"`, "", true},
		{`json:"a" caddy:"namespace=http"`, "a", true},
	} {
		name, include := jsonNameFromTag(tc.tag)
		if name != tc.name || include != tc.include {
			t.Errorf("jsonNameFromTag(%q) = (%q, %v); want (%q, %v)",
				tc.tag, name, include, tc.name, tc.include)
		}
	}
}

func TestCaddyTagFieldsEdgeCases(t *testing.T) {
	fields, err := caddyTagFields(`caddy:"namespace=http.handlers inline_key=handler"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fields["namespace"] != "http.handlers" || fields["inline_key"] != "handler" {
		t.Errorf("unexpected fields: %#v", fields)
	}

	fields, err = caddyTagFields(``)
	if err != nil {
		t.Fatalf("empty tag should not error: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("empty tag should yield no fields, got %#v", fields)
	}

	fields, err = caddyTagFields(`json:"a"`)
	if err != nil {
		t.Fatalf("tag without caddy key should not error: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("expected no caddy fields, got %#v", fields)
	}
}

func TestTypeNameHelpersOnNonNamedTypes(t *testing.T) {
	basic := types.Typ[types.String]

	if got := fullyQualifiedTypeName(basic); got != "string" {
		t.Errorf("fullyQualifiedTypeName(string) = %q; want %q", got, "string")
	}
	if pkg, name := typePackageAndName(basic); pkg != "" || name != "" {
		t.Errorf("typePackageAndName(string) = (%q, %q); want empty", pkg, name)
	}
	if got := localTypeName(basic); got != "" {
		t.Errorf("localTypeName(string) = %q; want empty", got)
	}

	ptr := types.NewPointer(basic)
	if got := fullyQualifiedTypeName(ptr); got != "*string" {
		t.Errorf("fullyQualifiedTypeName(*string) = %q; want %q", got, "*string")
	}

	slice := types.NewSlice(basic)
	if got := fullyQualifiedTypeName(slice); got != "[]string" {
		t.Errorf("fullyQualifiedTypeName([]string) = %q; want %q", got, "[]string")
	}
}
