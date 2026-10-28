package moduledoc

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"golang.org/x/tools/go/packages"
)

func TestPackageErrorKindNames(t *testing.T) {
	for _, tc := range []struct {
		kind packages.ErrorKind
		want string
	}{
		{packages.UnknownError, "unknown"},
		{packages.ListError, "list"},
		{packages.ParseError, "parse"},
		{packages.TypeError, "type"},
		{packages.ErrorKind(99), "unknown(99)"},
		{packages.ErrorKind(-1), "unknown(-1)"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			if got := packageErrorKindName(tc.kind); got != tc.want {
				t.Errorf("packageErrorKindName(%d) = %q; want %q", tc.kind, got, tc.want)
			}
		})
	}
}

func TestOperationLoggingIsolation(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	d := New(newMemStorage())
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprint(i)
			ctx := WithLogger(context.Background(), zap.New(core).With(
				zap.String("request_id", id), zap.String("account_id", "account-"+id)))
			values, err := d.LoadTypesByModuleIDContext(ctx, "module-"+id)
			if err != nil || len(values) != 0 {
				t.Errorf("loading unknown module: values=%v err=%v", values, err)
			}
		}(i)
	}
	wg.Wait()
	if logs.Len() != 32 {
		t.Fatalf("got %d entries; want 32", logs.Len())
	}
	for _, entry := range logs.All() {
		fields := entry.ContextMap()
		id, ok := fields["request_id"].(string)
		if !ok || fields["account_id"] != "account-"+id || fields["module_id"] != "module-"+id {
			t.Errorf("cross-request log fields: %v", fields)
		}
		if entry.Message == "operation completed" {
			if _, ok := fields["duration"]; !ok {
				t.Error("completion log missing duration")
			}
		}
	}
}

func TestOperationFailureLogging(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	ctx := WithLogger(context.Background(), zap.New(core).With(zap.String("request_id", "failed-request")))
	_, _, err := New(newMemStorage()).LoadTypeByPathContext(ctx, "apps/http", "v2.11.4")
	if err == nil {
		t.Fatal("expected missing root type error")
	}
	entries := logs.FilterMessage("operation failed").All()
	if len(entries) != 1 {
		t.Fatalf("failure logs = %d; want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["request_id"] != "failed-request" || fields["config_path"] != "apps/http" ||
		fields["version"] != "v2.11.4" || fields["error"] != err.Error() {
		t.Fatalf("missing failure details: %v", fields)
	}
}

type loggingFailureStorage struct{ *memStorage }

func (loggingFailureStorage) GetTypesByCaddyModuleID(string) ([]*Value, error) {
	return nil, errors.New("storage unavailable")
}

func (loggingFailureStorage) StoreType(string, string, string, *Value) error {
	return errors.New("storage write failed")
}

func TestStorageFailureLogging(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	ctx := WithLogger(context.Background(), zap.New(core).With(zap.String("request_id", "storage-request")))
	_, err := New(loggingFailureStorage{newMemStorage()}).LoadTypesByModuleIDContext(ctx, "http.handlers.test")
	if err == nil {
		t.Fatal("storage failure was swallowed")
	}
	entries := logs.FilterMessage("loading module types from storage failed").All()
	if len(entries) != 1 || entries[0].ContextMap()["module_id"] != "http.handlers.test" ||
		entries[0].ContextMap()["request_id"] != "storage-request" {
		t.Fatalf("missing storage stage details: %v", entries)
	}
}

func TestModuleDereferenceFailureDoesNotPanic(t *testing.T) {
	db := newMemStorage()
	db.modules["broken"] = []*Value{{TypeName: "Broken", SameAs: "example.com/pkg.Missing"}}
	core, logs := observer.New(zap.DebugLevel)
	ctx := WithLogger(context.Background(), zap.New(core))
	if _, err := New(db).LoadTypesByModuleIDContext(ctx, "broken"); err == nil {
		t.Fatal("expected unresolved type error")
	}
	entries := logs.FilterMessage("dereferencing module type failed").All()
	if len(entries) != 1 || entries[0].ContextMap()["type"] != "Broken" {
		t.Fatalf("missing failing type: %v", entries)
	}
}

func TestSkippedModuleLogging(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "runtime.go", `package example
type Runtime struct{}
func (*Runtime) CaddyModule() ModuleInfo {
	return ModuleInfo{ID: runtimeID()}
}`, 0)
	if err != nil {
		t.Fatal(err)
	}
	core, logs := observer.New(zap.DebugLevel)
	ctx := WithLogger(context.Background(), zap.New(core).With(zap.String("request_id", "skip-request")))
	pkg := &packages.Package{PkgPath: "example.com/plugin", Syntax: []*ast.File{file}, TypesInfo: &types.Info{}}
	modules, err := New(nil).findCaddyModuleIdentsContext(ctx, pkg)
	if err != nil || len(modules) != 0 {
		t.Fatalf("runtime ID behavior changed: %v, %v", modules, err)
	}
	entries := logs.FilterMessage("skipping module with runtime-computed ID").All()
	if len(entries) != 1 || entries[0].ContextMap()["package"] != "example.com/plugin" ||
		entries[0].ContextMap()["type"] != "Runtime" || entries[0].ContextMap()["request_id"] != "skip-request" {
		t.Fatalf("missing skip details: %v", entries)
	}
}

func TestGoCommandDiagnosticsLogging(t *testing.T) {
	if os.Getenv("MODULEDOC_LOG_OUTPUT_CHILD") == "1" {
		fmt.Fprint(os.Stderr, strings.Repeat("x", 20*1024)+"diagnostic tail")
		os.Exit(1)
	}
	core, logs := observer.New(zap.DebugLevel)
	ctx := WithLogger(context.Background(), zap.New(core).With(zap.String("request_id", "command-request")))
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestGoCommandDiagnosticsLogging$")
	cmd.Env = append(os.Environ(), "MODULEDOC_LOG_OUTPUT_CHILD=1")
	if err := runGoCommand(ctx, cmd); err == nil {
		t.Fatal("command failure was swallowed")
	}
	entries := logs.FilterMessage("Go command failed").All()
	if len(entries) != 1 {
		t.Fatalf("command failure logs = %d; want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	output, ok := fields["diagnostics"].(string)
	if !ok || len(output) != 16*1024 || !strings.HasSuffix(output, "diagnostic tail") ||
		fields["diagnostics_truncated"] != true || fields["request_id"] != "command-request" {
		t.Fatalf("invalid command diagnostics: length=%d, truncated=%v", len(output), fields["diagnostics_truncated"])
	}
	var fullOutput strings.Builder
	for _, entry := range logs.FilterMessage("Go command output").All() {
		fields := entry.ContextMap()
		if fields["stream"] != "stderr" || fields["request_id"] != "command-request" {
			t.Fatalf("missing output correlation: %v", fields)
		}
		fullOutput.WriteString(fields["output"].(string))
	}
	if fullOutput.String() != strings.Repeat("x", 20*1024)+"diagnostic tail" {
		t.Fatal("full failed-command output was lost")
	}
}

func TestCommandOutputStreaming(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	writer := commandOutput{logger: zap.New(core)}
	if _, err := writer.Write([]byte("first\npartial")); err != nil {
		t.Fatal(err)
	}
	if logs.Len() != 1 || logs.All()[0].ContextMap()["output"] != "first\n" {
		t.Fatal("completed line was not logged immediately")
	}
	if _, err := writer.Write([]byte(" line\n\n" + strings.Repeat("x", 40*1024))); err != nil {
		t.Fatal(err)
	}
	writer.flush()
	writer.flush()
	var output strings.Builder
	for _, entry := range logs.All() {
		chunk := entry.ContextMap()["output"].(string)
		if len(chunk) == 0 || len(chunk) > 16*1024 {
			t.Fatalf("invalid output chunk size: %d", len(chunk))
		}
		output.WriteString(chunk)
	}
	if output.String() != "first\npartial line\n\n"+strings.Repeat("x", 40*1024) {
		t.Fatal("streaming output was lost or duplicated")
	}
}

func TestSuccessfulCommandOutputLogging(t *testing.T) {
	if os.Getenv("MODULEDOC_SUCCESS_OUTPUT_CHILD") == "1" {
		var wg sync.WaitGroup
		for _, stream := range []*os.File{os.Stdout, os.Stderr} {
			wg.Add(1)
			go func(stream *os.File) {
				defer wg.Done()
				for i := 0; i < 128; i++ {
					fmt.Fprintln(stream, strings.Repeat("x", 512))
				}
				fmt.Fprint(stream, "unterminated tail")
			}(stream)
		}
		wg.Wait()
		os.Exit(0)
	}
	core, logs := observer.New(zap.InfoLevel)
	ctx := WithLogger(context.Background(), zap.New(core).With(
		zap.String("request_id", "successful-request"), zap.String("account_id", "account-123")))
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSuccessfulCommandOutputLogging$")
	cmd.Env = append(os.Environ(), "MODULEDOC_SUCCESS_OUTPUT_CHILD=1")
	if err := runGoCommand(ctx, cmd); err != nil {
		t.Fatal(err)
	}
	output := map[string]*strings.Builder{"stdout": {}, "stderr": {}}
	for _, entry := range logs.FilterMessage("Go command output").All() {
		fields := entry.ContextMap()
		if fields["request_id"] != "successful-request" || fields["account_id"] != "account-123" {
			t.Fatal("command output lost correlation fields")
		}
		if _, ok := fields["command"]; !ok {
			t.Fatal("command output lost command field")
		}
		stream, ok := fields["stream"].(string)
		if !ok || output[stream] == nil {
			t.Fatalf("unexpected stream: %v", fields["stream"])
		}
		output[stream].WriteString(fields["output"].(string))
	}
	want := strings.Repeat(strings.Repeat("x", 512)+"\n", 128) + "unterminated tail"
	for stream, got := range output {
		if got.String() != want {
			t.Errorf("%s output lost or duplicated: got %d bytes; want %d", stream, got.Len(), len(want))
		}
	}
}

func TestCommandOutputUTF8Boundaries(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	writer := commandOutput{logger: zap.New(core)}
	want := strings.Repeat("x", 16*1024-1) + "\u20ac" + strings.Repeat("y", 16*1024)
	for _, chunk := range []string{want[:16*1024], want[16*1024:]} {
		if _, err := writer.Write([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	writer.flush()
	var got strings.Builder
	for _, entry := range logs.All() {
		chunk := entry.ContextMap()["output"].(string)
		if !utf8.ValidString(chunk) {
			t.Fatal("split UTF-8 character would be corrupted by JSON encoding")
		}
		got.WriteString(chunk)
	}
	if got.String() != want {
		t.Fatal("UTF-8 command output changed")
	}
}

func TestCommandDiagnosticsLimit(t *testing.T) {
	var diagnostics commandDiagnostics
	want := ""
	for _, chunk := range []string{
		strings.Repeat("a", 16*1024),
		"tail",
		strings.Repeat("b", 20*1024),
		"final",
	} {
		n, err := diagnostics.Write([]byte(chunk))
		if err != nil || n != len(chunk) {
			t.Fatalf("diagnostic write = %d, %v", n, err)
		}
		want += chunk
		truncated := len(want) > 16*1024
		tail := want
		if truncated {
			tail = want[len(want)-16*1024:]
		}
		if string(diagnostics.tail) != tail || diagnostics.truncated != truncated {
			t.Fatal("diagnostics did not retain the exact bounded tail")
		}
	}
}

func TestCancelledOperationLogging(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	ctx, cancel := context.WithCancel(WithLogger(context.Background(), zap.New(core).With(zap.String("request_id", "cancel-request"))))
	cancel()
	if _, err := New(nil).LoadModulesFromImportingPackageContext(ctx, "example.com/plugin", "v1.0.0"); err == nil {
		t.Fatal("cancelled package loading should fail")
	}
	if logs.FilterMessage("Go command failed").Len() != 1 {
		t.Fatal("cancelled Go command was not logged")
	}
	for _, entry := range logs.All() {
		if entry.ContextMap()["request_id"] != "cancel-request" {
			t.Errorf("lost cancellation correlation: %v", entry)
		}
	}
}

func TestPackageLoadingLogging(t *testing.T) {
	if testing.Short() {
		t.Skip("requires the Go toolchain")
	}
	core, logs := observer.New(zap.DebugLevel)
	ctx := WithLogger(context.Background(), zap.New(core).With(zap.String("request_id", "package-request")))
	d := New(newMemStorage())
	ws := localWorkspace(t, d)
	ws.ctx = ctx
	pkgs, err := ws.getPackages(testdataPackagePath, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ws.getPackages(testdataPackagePath, ""); err != nil {
		t.Fatal(err)
	}
	rb := ws.representationBuilder()
	modules, err := rb.loadModulesFromSinglePackage(pkgs[0])
	if err != nil || len(modules) != 1 {
		t.Fatalf("module processing: modules=%v err=%v", modules, err)
	}
	for _, message := range []string{"packages loaded", "package cache hit", "module processed", "type representation stored"} {
		if logs.FilterMessage(message).Len() == 0 {
			t.Errorf("missing internal log: %s", message)
		}
	}
	rb.ws.driver = New(loggingFailureStorage{newMemStorage()})
	if _, err := rb.loadModulesFromSinglePackage(pkgs[0]); err == nil {
		t.Fatal("type storage failure was swallowed")
	}
	entries := logs.FilterMessage("storing type representation failed").All()
	if len(entries) != 1 || entries[0].ContextMap()["type"] != testdataPackagePath+".Gizmo" {
		t.Fatalf("missing type storage failure details: %v", entries)
	}
	for _, entry := range logs.All() {
		if entry.ContextMap()["request_id"] != "package-request" {
			t.Errorf("lost internal correlation: %v", entry)
		}
	}
}
