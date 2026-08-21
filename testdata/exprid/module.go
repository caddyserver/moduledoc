package exprid

import "github.com/caddyserver/caddy/v2"

const idPrefix = "app.namespace."

func init() {
	caddy.RegisterModule(new(ExprWidget))
}

type ExprWidget struct{}

// CaddyModule implements caddy.Module
func (*ExprWidget) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: idPrefix + "expr_widget",
		New: func() caddy.Module {
			return new(ExprWidget)
		},
	}
}

var _ caddy.Module = (*ExprWidget)(nil)
