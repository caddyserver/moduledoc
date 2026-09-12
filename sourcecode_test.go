package moduledoc

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestFindModuleRegistrationIgnoresMethods(t *testing.T) {
	for _, tc := range []struct {
		name   string
		method string
		call   string
	}{
		{"two arguments", "func (*State) RegisterModule(string, map[string]func()) {}", `state.RegisterModule("_G", nil)`},
		{"one argument", "func (*State) RegisterModule(any) {}", `state.RegisterModule(App{})`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			// Type-check a minimal Caddy package so the real Go type checker
			// distinguishes an imported package name from a method receiver.
			caddyFile, err := parser.ParseFile(fset, "caddy.go", `package caddy
func RegisterModule(any) {}`, 0)
			if err != nil {
				t.Fatal(err)
			}
			caddyPkg, err := new(types.Config).Check(caddyCorePackagePath, fset, []*ast.File{caddyFile}, nil)
			if err != nil {
				t.Fatal(err)
			}
			src := `package example
import caddycore "github.com/caddyserver/caddy/v2"
type App struct{}
type State struct{}
` + tc.method + `
func init() {
 state := new(State)
 ` + tc.call + `
 caddycore.RegisterModule(App{})
}`
			file, err := parser.ParseFile(fset, "example.go", src, 0)
			if err != nil {
				t.Fatal(err)
			}
			info := &types.Info{Uses: make(map[*ast.Ident]types.Object)}
			cfg := types.Config{Importer: registrationTestImporter{caddyPkg}}
			if _, err := cfg.Check("example.com/plugin", fset, []*ast.File{file}, info); err != nil {
				t.Fatal(err)
			}
			pkg := &packages.Package{TypesInfo: info}
			var registered []string
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				ident, err := new(Driver).findModuleRegistration(pkg, call)
				if err != nil {
					t.Errorf("%s: %v", fset.Position(call.Pos()), err)
				}
				if ident != nil {
					registered = append(registered, ident.Name)
				}
				return true
			})
			if len(registered) != 1 || registered[0] != "App" {
				t.Fatalf("registered types = %v; want only the Caddy registration of App", registered)
			}
		})
	}
}

type registrationTestImporter struct{ pkg *types.Package }

func (i registrationTestImporter) Import(path string) (*types.Package, error) {
	if path != i.pkg.Path() {
		return nil, fmt.Errorf("unexpected import %q", path)
	}
	return i.pkg, nil
}
