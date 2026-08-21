package constid

import "github.com/caddyserver/caddy/v2"

const widgetModuleID = "app.namespace.const_widget"

func init() {
	caddy.RegisterModule(new(ConstWidget))
}

type ConstWidget struct{}

// CaddyModule implements caddy.Module
func (*ConstWidget) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: widgetModuleID,
		New: func() caddy.Module {
			return new(ConstWidget)
		},
	}
}

var _ caddy.Module = (*ConstWidget)(nil)
