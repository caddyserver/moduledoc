// Package crosspkg registers a type declared in another package
// (a qualified composite literal, valuerecv.Sprocket{}), exercising
// the AST handling of selector-expression registration arguments.
package crosspkg

import (
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/moduledoc/testdata/valuerecv"
)

func init() {
	caddy.RegisterModule(valuerecv.Sprocket{})
}
