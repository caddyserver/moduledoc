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
