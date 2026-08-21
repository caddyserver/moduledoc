# ModuleDoc AI Coding Instructions

## Project Overview
ModuleDoc analyzes Go source code to generate JSON documentation for Caddy modules. It uses Go's AST parsing and type system to extract struct definitions, JSON tags, and documentation, specifically targeting Caddy's plugin architecture.

## Core Architecture

### Three-Layer System
1. **Driver** (`driver.go`) - Main API entry point with caching (`discoveredTypes` map)
2. **Workspace** (`workspace.go`) - Temporary Go module management with `go get` and package loading
3. **Synthesis** (`synthesis.go`) - Type representation building from Go AST/types

### Key Data Flow
```
LoadModulesFromImportingPackage → openWorkspace → findCaddyModuleIdents → buildRepresentation → Value struct
```

## Critical Patterns

### AST Analysis for Caddy Modules
- Look for `caddy.RegisterModule()` calls AND `CaddyModule()` method implementations
- Extract module IDs from `caddy.ModuleInfo{ID: "module.name"}` return statements
- Both registration and implementation must exist (validated in `findCaddyModuleIdents`)

### Type System Mapping
The `Value` struct is central - it flattens Go types into JSON-documentable forms:
- `Type` enum: primitives (`Bool`, `String`) + structural (`Struct`, `Array`, `Map`) + Caddy-specific (`Module`, `ModuleMap`)
- `SameAs` field enables type reuse via FQTN (fully-qualified type name) references
- `ModuleNamespace`/`ModuleInlineKey` handle Caddy's module configuration patterns

### Workspace Management
- Creates temporary Go modules in `/tmp` for isolated package analysis
- Runs `go get` commands to fetch dependencies at specific versions
- Uses `golang.org/x/tools/go/packages` with specific `packages.Config` modes
- **Always** set `CGO_ENABLED=0` in test environments (Linux compatibility)

### Storage Interface
Implement `Storage` for caching - methods: `GetTypeByName`, `GetTypesByCaddyModuleID`, `StoreType`, `SetCaddyModuleName`

## Testing Patterns

### Test Structure
- Two suites: baseline tests (pass on current behavior, document limitations) and the edge-case suite (assert correct behavior; intentionally RED until the tracked bug is fixed)
- Do not "fix" a red edge-case test by weakening its assertions; fix the source bug it tracks
- Use `testdata/` fixture packages (one per directory) with minimal Caddy module examples
- Load packages with full AST/types info: `packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo`
- Set `CGO_ENABLED=0` in test environments to avoid "no metadata for C" errors
- Toolchain-dependent tests skip under `go test -short`; crash-prone tests (stack overflow, fatal map races) re-run themselves in a child process via `runIsolated`
- A reusable thread-safe in-memory mock Storage (`memStorage`) lives in `storage_edge_test.go`

### Example Module Pattern (see `testdata/gizmo.go`):
```go
type ModuleName struct{}

func init() {
    caddy.RegisterModule(new(ModuleName))
}

func (*ModuleName) CaddyModule() caddy.ModuleInfo {
    return caddy.ModuleInfo{
        ID: "namespace.module_name",
        New: func() caddy.Module { return new(ModuleName) },
    }
}
```

## Key Constants & Conventions
- `CaddyCorePackage = "github.com/caddyserver/caddy/v2"`
- `registerModule = "RegisterModule"`
- FQTN format: `"package/path.TypeName@version"`
- Module ID format: dot-separated namespaces like `"http.handlers.file_server"`

## JSON Tag Processing
- Uses `reflect.StructTag` to parse `json:` and `caddy:` tags
- JSON name extraction handles comma-separated options, excludes `"-"` fields
- Caddy tags parsed via `caddy.ParseStructTag()` for module configuration

## Error Handling Patterns
- Extensive validation that module registration matches implementation
- AST inspection errors bubble up through `visitErr` pattern
- Package loading failures wrapped with context (`fmt.Errorf("loading package %s: %v", ...)`)

## Recent Improvements (2025)

None. An earlier version of this document claimed fixes (mutex locking, LRU cache with TTL, dynamic module ID support, graceful type fallbacks, relaxed validation) that were never implemented. Do not assume any of them exist.

## Known Limitations & Open Bugs

- **Not thread-safe**: `Driver.mu` is declared but never used; `discoveredTypes` is accessed without locking (confirmed data race). `dereference` mutates Storage-owned values in place, making concurrent `LoadTypesByModuleID` racy as well.
- **Static module IDs only**: IDs computed from constants or expressions (e.g. `caddy.ModuleID("prefix" + suffix)`) are silently skipped with a warning; only `*ast.BasicLit` string literals are supported.
- **Panics on edge inputs**: `TraverseType` panics on unknown module IDs (`vals[0]` without a length check); unchecked AST type assertions panic on inputs like `caddy.RegisterModule(otherpkg.Type{})`.
- **No recursion guards**: self-referential types cause stack overflows in both `buildRepresentation` and `deepDereference`.
- **Hard failures on unknown types**: chan/func/generic fields error out the whole containing type; no fallback representation.
- **Unbounded caches**: per-workspace package caches have no TTL, size limit, or eviction (memory-growth issue only; access is properly locked).
- **Strict validation**: one incomplete module (registration without implementation, or vice versa) fails the entire package load.

## Development Notes
- Requires Go toolchain installed (uses `go` commands directly)
- **NOT thread-safe** — do not share a Driver or workspace across goroutines until the tracked races are fixed
- Caching is per-workspace and unbounded (no eviction); the Driver's `discoveredTypes` type cache is unsynchronized