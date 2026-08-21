package kinds

import "encoding/json"

// Widget exercises the field kinds the doc system should handle.
type Widget struct {
	// Name is the widget name.
	Name string `json:"name"`

	// Hidden is excluded from JSON.
	Hidden string `json:"-"`

	unexported string

	DoublePtr **int          `json:"double_ptr,omitempty"`
	Numbers   []int          `json:"numbers,omitempty"`
	Lookup    map[int]string `json:"lookup,omitempty"`
	Anything  interface{}    `json:"anything,omitempty"`

	Raw    json.RawMessage            `json:"raw,omitempty" caddy:"namespace=widget.raw inline_key=kind"`
	RawMap map[string]json.RawMessage `json:"raw_map,omitempty" caddy:"namespace=widget.rawmap"`

	Embedded

	Nested Nested `json:"nested,omitempty"`

	Inline struct {
		A string `json:"a"`
	} `json:"inline,omitempty"`
}

// Embedded is a mixin whose fields are promoted.
type Embedded struct {
	// Extra adds extra behavior.
	Extra bool `json:"extra,omitempty"`
}

// Nested is a named nested type.
type Nested struct {
	B int `json:"b"`
}

// Node is a self-referential type.
type Node struct {
	Next *Node `json:"next,omitempty"`
}

// Tricky has field kinds with no JSON representation.
type Tricky struct {
	Notify func()   `json:"notify,omitempty"`
	Events chan int `json:"events,omitempty"`
}
