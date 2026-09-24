// Package noimpl registers a type that satisfies caddy.Module only
// through an embedded field, so the package has a registration but no
// locally declared CaddyModule method.
//
// Expected: findCaddyModuleIdents returns an error.
package noimpl

import (
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/moduledoc/testdata"
)

func init() {
	caddy.RegisterModule(new(Inherited))
}

// Inherited satisfies caddy.Module through embedding, so this
// package has a registration but no local CaddyModule method.
type Inherited struct {
	testdata.Gizmo
}
