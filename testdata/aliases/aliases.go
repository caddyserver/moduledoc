// Package aliases is a test fixture exercising type-alias handling in
// moduledoc under Go 1.23+ (where go/types materialises aliases as
// *types.Alias).
//
// Expected (see synthesis_test.go, utils_test.go): buildRepresentation
// unwraps aliases so Config.Nested resolves to the Settings struct,
// Config.Label resolves to a plain string, and Config.Items resolves
// to a slice of the Item struct — the alias names themselves do not
// appear as distinct types.
package aliases

// Settings holds nested configuration used to verify alias unwrapping.
type Settings struct {
	// Name is a plain string field.
	Name string `json:"name"`
}

// SettingsAlias aliases Settings so field lookup must unwrap it.
type SettingsAlias = Settings

// NameAlias aliases the string primitive.
type NameAlias = string

// Item is an element type referenced through an alias slice.
type Item struct {
	// Value is the item's value.
	Value string `json:"value"`
}

// ItemAlias aliases Item so slices of aliases resolve to the target struct.
type ItemAlias = Item

// Config is the top-level struct wired to alias-typed fields.
type Config struct {
	// Nested is an alias-typed struct field.
	Nested SettingsAlias `json:"nested"`

	// Label is an alias-typed primitive field.
	Label NameAlias `json:"label"`

	// Items is a slice whose element is an alias to Item.
	Items []ItemAlias `json:"items"`
}
