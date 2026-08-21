package valuerecv

import "github.com/caddyserver/caddy/v2"

func init() {
	caddy.RegisterModule(Sprocket{})
}

type Sprocket struct{}

// CaddyModule implements caddy.Module
func (Sprocket) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: "app.namespace.sprocket",
		New: func() caddy.Module {
			return Sprocket{}
		},
	}
}

var _ caddy.Module = Sprocket{}
