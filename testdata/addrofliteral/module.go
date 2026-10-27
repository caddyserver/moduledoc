// Package addrofliteral registers a pointer-receiver module using &Widget{}.
package addrofliteral

import "github.com/caddyserver/caddy/v2"

func init() {
	caddy.RegisterModule(&Widget{})
}

type Widget struct{}

func (*Widget) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "app.namespace.addr_widget",
		New: func() caddy.Module { return new(Widget) },
	}
}

var _ caddy.Module = (*Widget)(nil)
