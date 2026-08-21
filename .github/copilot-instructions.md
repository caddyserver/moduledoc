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
- Use `testdata/` directory with minimal Caddy module examples
- Load packages with full AST/types info: `packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo`
- Set `CGO_ENABLED=0` in test environments to avoid "no metadata for C" errors

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

### Fixed Critical Issues
- **Race Conditions**: Added proper mutex locking around `discoveredTypes` map access
- **Dynamic Module IDs**: Enhanced AST analysis to handle computed module IDs like `caddy.ModuleID("prefix" + suffix)`
- **Constant Resolution**: Module IDs can now reference package-level constants
- **Graceful Degradation**: Unknown Go types fallback to string representation instead of hard failures
- **Memory Management**: Implemented LRU cache with TTL (30min) and size limits (100 entries) for package cache
- **Relaxed Validation**: Modules with only `CaddyModule()` implementation (no registration) now generate warnings instead of errors

### Enhanced Error Handling
- Interface types, channels, and function signatures have proper fallback representations
- Complex AST expressions (binary operations, function calls, selectors) are now supported for module IDs
- Cache eviction prevents memory leaks in long-running processes

## Development Notes
- Requires Go toolchain installed (uses `go` commands directly)
- Thread-safe operations with proper mutex locking on all shared state
- Heavy caching for performance - workspace package cache with automatic eviction, type representation cache