ModuleDoc
=========

[![godoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://pkg.go.dev/github.com/caddyserver/moduledoc)

This package implements the source analysis core of [Caddy](https://caddyserver.com)'s [module documentation system](https://caddyserver.com/docs/json/). It reads Go code to generate JSON docs -- kind of like godoc, but following JSON tags, and with special integration with Caddy's module architecture.

This package requires Go to be installed on the machine because it uses [the `packages` package](https://pkg.go.dev/golang.org/x/tools/go/packages). Inputs are fully-qualified package and type names, and outputs are JSON-structured type definitions and documentation. A backing data store is required for amortization.

A front-end can then be built to render the results, for example: both https://caddyserver.com/docs/json/ and https://caddyserver.com/docs/modules are powered by this package.

**Author's disclaimer:** This package comes with no guarantees of stability or correctness. The code is very hand-wavy, and I really had no idea what I was doing at the time, but at least it works! (Mostly.) Always refer to actual source code as authoritative documentation as the output of this package is not always perfect.

_(c) 2019 Matthew Holt_
