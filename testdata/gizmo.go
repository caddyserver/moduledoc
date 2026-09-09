package testdata

import "github.com/caddyserver/caddy/v2"

func init() {
	caddy.RegisterModule(new(Gizmo))
}

type Gizmo struct{}

// CaddyModule implements caddy.Module
func (*Gizmo) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: "app.namespace.gizmo",
		New: func() caddy.Module {
			return new(Gizmo)
		},
	}
}

var _ caddy.Module = (*Gizmo)(nil)
