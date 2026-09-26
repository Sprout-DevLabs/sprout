package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// mcpSession sends requests to the server and returns responses by id.
func mcpSession(t *testing.T, root string, reqs ...string) map[float64]map[string]any {
	t.Helper()
	var out, errOut bytes.Buffer
	if code := serveMCP([]string{root}, strings.NewReader(strings.Join(reqs, "\n")), &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	resps := map[float64]map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var r map[string]any
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("bad response %q: %v", line, err)
		}
		id, _ := r["id"].(float64)
		resps[id] = r
	}
	return resps
}

func toolText(t *testing.T, r map[string]any) (string, bool) {
	t.Helper()
	res, ok := r["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result in %v", r)
	}
	content := res["content"].([]any)[0].(map[string]any)
	return content["text"].(string), res["isError"].(bool)
}

func TestMCPHandshakeAndTools(t *testing.T) {
	dir := setupTestDir(t)
	resps := mcpSession(t, dir,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"project_map","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"tree","arguments":{"path":"src"}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"nope"}`,
	)
	if len(resps) != 5 {
		t.Errorf("notifications must not be answered; got %d responses", len(resps))
	}
	if v := resps[1]["result"].(map[string]any)["protocolVersion"]; v != "2025-06-18" {
		t.Errorf("protocolVersion = %v", v)
	}
	if n := len(resps[2]["result"].(map[string]any)["tools"].([]any)); n != 4 {
		t.Errorf("tools/list returned %d tools", n)
	}
	if text, isErr := toolText(t, resps[3]); isErr || !strings.Contains(text, "## structure") {
		t.Errorf("project_map failed: %s", text)
	}
	text, isErr := toolText(t, resps[4])
	if isErr || !strings.HasPrefix(text, "src\n") || !strings.Contains(text, "main.go") {
		t.Errorf("tree on subdir: %q", text)
	}
	if strings.Contains(text, dir) {
		t.Errorf("absolute root path leaked into tool output: %q", text)
	}
	if resps[5]["error"].(map[string]any)["code"].(float64) != -32601 {
		t.Errorf("unknown method: %v", resps[5])
	}
}

func TestMCPConfinesPaths(t *testing.T) {
	dir := setupTestDir(t)
	outside := t.TempDir()
	if runtime.GOOS != "windows" {
		if err := os.Symlink(outside, filepath.Join(dir, "escape")); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{"..", "../..", "src/../..", outside, "escape", "--all"} {
		if p == "escape" && runtime.GOOS == "windows" {
			continue
		}
		arg, _ := json.Marshal(map[string]string{"path": p})
		resps := mcpSession(t, dir, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"tree","arguments":`+string(arg)+`}}`)
		if text, isErr := toolText(t, resps[1]); !isErr {
			t.Errorf("path %q escaped the root:\n%s", p, text)
		}
	}
}

func TestMCPDiffRejectsInjection(t *testing.T) {
	dir := setupGitRepo(t)
	resps := mcpSession(t, dir, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"diff_tree","arguments":{"rev":"--output=pwned"}}}`)
	if _, isErr := toolText(t, resps[1]); !isErr {
		t.Error("option-like rev must be rejected")
	}
	if _, err := os.Stat(filepath.Join(dir, "pwned")); err == nil {
		t.Error("git wrote a file from an injected option")
	}
}
