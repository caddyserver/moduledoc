// Package exprid is a module whose ID is built from concatenating a
// package-level constant with a string literal, exercising constant
// binary-expression evaluation in module ID discovery.
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
