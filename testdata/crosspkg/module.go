package crosspkg

import (
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/moduledoc/testdata/valuerecv"
)

func init() {
	caddy.RegisterModule(valuerecv.Sprocket{})
}
