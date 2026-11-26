package app_test

import (
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/nednella/orchestrator/desktop/internal/app"
	"github.com/nednella/orchestrator/desktop/internal/apptest"
)

// boundMethods reads the table of window.go.<namespace>.Service methods that api.ts calls.
func boundMethods(t *testing.T) map[string][]string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "..", "frontend", "src", "api.ts"))
	if err != nil {
		t.Fatal(err)
	}
	_, table, ok := strings.Cut(string(src), "const namespaces = {")
	if !ok {
		t.Fatal("api.ts has no namespaces table")
	}
	table, _, _ = strings.Cut(table, "\n}")
	line := regexp.MustCompile(`(?m)^\s*(\w+): \[(.*)\],?$`)
	name := regexp.MustCompile(`'(\w+)'`)
	out := map[string][]string{}
	for _, m := range line.FindAllStringSubmatch(table, -1) {
		for _, n := range name.FindAllStringSubmatch(m[2], -1) {
			out[m[1]] = append(out[m[1]], n[1])
		}
	}
	if len(out) == 0 {
		t.Fatal("api.ts namespaces table is empty")
	}
	return out
}

func TestEveryMethodTheFrontEndCallsIsBound(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ORCHESTRATOR_CONFIG", filepath.Join(root, "config.yaml"))
	t.Setenv("ORCHESTRATOR_STATE_DIR", filepath.Join(root, "state"))
	t.Setenv("ORCHESTRATOR_DEV_DATA_DIR", filepath.Join(root, "data"))
	t.Setenv("ORCHESTRATOR_DIR", root)
	cfg, err := app.Load()
	if err != nil {
		t.Fatal(err)
	}
	a := app.New(cfg, app.Host{Emit: func(string, any) {}, Clipboard: func(string) {}}, apptest.NoGH, apptest.NoStream, apptest.NoClaude)
	bound := map[string]reflect.Type{}
	for _, svc := range a.Services() {
		typ := reflect.TypeOf(svc)
		bound[path.Base(typ.Elem().PkgPath())] = typ
	}
	for namespace, methods := range boundMethods(t) {
		typ, ok := bound[namespace]
		if !ok {
			t.Errorf("api.ts calls the %s namespace and the app binds no such service", namespace)
			continue
		}
		for _, method := range methods {
			if _, ok := typ.MethodByName(method); !ok {
				t.Errorf("api.ts calls %s.Service.%s, which %s does not have", namespace, method, typ)
			}
		}
	}
}
