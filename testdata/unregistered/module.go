package unregistered

import "github.com/caddyserver/caddy/v2"

func init() {
	caddy.RegisterModule(new(Good))
}

type Good struct{}

// CaddyModule implements caddy.Module
func (*Good) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: "app.namespace.good",
		New: func() caddy.Module {
			return new(Good)
		},
	}
}

// Orphan implements caddy.Module but is never registered.
type Orphan struct{}

// CaddyModule implements caddy.Module
func (*Orphan) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: "app.namespace.orphan",
		New: func() caddy.Module {
			return new(Orphan)
		},
	}
}
