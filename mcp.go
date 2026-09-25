package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// MCP (Model Context Protocol) server over stdio: newline-delimited
// JSON-RPC 2.0. Each tool is a thin wrapper that builds CLI arguments and
// calls run(), so the CLI and agents always get identical behaviour.

var mcpVersions = []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}

type rpcRequest struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	args        func(a toolArgs) ([]string, error)
}

type toolArgs struct {
	Path   string `json:"path"`
	Budget int    `json:"budget"`
	Depth  *int   `json:"depth"`
	All    bool   `json:"all"`
	Git    bool   `json:"git"`
	Churn  bool   `json:"churn"`
	Since  string `json:"since"`
	Rev    string `json:"rev"`
}

func schema(props map[string]any, required ...string) map[string]any {
	props["path"] = map[string]any{"type": "string", "description": "Directory relative to the project root. Defaults to the root."}
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

var mcpTools = []mcpTool{
	{
		Name: "project_map",
		Description: "Compact overview of a codebase for getting oriented: purpose, stack, languages, " +
			"entry points, config/CI files, uncommitted work, recent hotspots, and directory structure, " +
			"fitted to a token budget. Call this first in an unfamiliar repository.",
		InputSchema: schema(map[string]any{
			"budget": map[string]any{"type": "integer", "description": "Approximate token budget (default 2000)."},
		}),
		args: func(a toolArgs) ([]string, error) {
			args := []string{"--ai"}
			if a.Budget > 0 {
				args = append(args, "--budget", strconv.Itoa(a.Budget))
			}
			return args, nil
		},
	},
	{
		Name: "tree",
		Description: "Directory tree respecting .gitignore. Optionally mark git status (git) or " +
			"per-path commit counts to find hotspots (churn).",
		InputSchema: schema(map[string]any{
			"depth": map[string]any{"type": "integer", "description": "Maximum depth (default 3; -1 for unlimited)."},
			"all":   map[string]any{"type": "boolean", "description": "Include hidden and ignored entries."},
			"git":   map[string]any{"type": "boolean", "description": "Mark changed files: M, A, D, R, ?, U."},
			"churn": map[string]any{"type": "boolean", "description": "Show commits touching each path."},
			"since": map[string]any{"type": "string", "description": "With churn: only count commits since, e.g. '90 days ago'."},
		}),
		args: func(a toolArgs) ([]string, error) {
			depth := 3 // an unbounded tree of a big repo would flood the agent's context
			if a.Depth != nil {
				depth = *a.Depth
			}
			args := []string{"--depth", strconv.Itoa(depth)}
			if a.All {
				args = append(args, "--all")
			}
			if a.Git {
				args = append(args, "--git")
			}
			if a.Churn {
				args = append(args, "--churn")
				if a.Since != "" {
					args = append(args, "--since", a.Since)
				}
			}
			return args, nil
		},
	},
	{
		Name: "diff_tree",
		Description: "Paths changed in a git revision range as a tree with per-directory +/- line totals. " +
			"Use rev 'main...HEAD' to see what the current branch changed.",
		InputSchema: schema(map[string]any{
			"rev":   map[string]any{"type": "string", "description": "Git revision or range, e.g. 'main...HEAD', 'HEAD~3'."},
			"depth": map[string]any{"type": "integer", "description": "Collapse directories below this depth into totals."},
		}, "rev"),
		args: func(a toolArgs) ([]string, error) {
			args := []string{"--diff", a.Rev}
			if a.Depth != nil {
				args = append(args, "--depth", strconv.Itoa(*a.Depth))
			}
			return args, nil
		},
	},
}

func serveMCP(args []string, in io.Reader, out, errOut io.Writer) int {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}
	root, err := resolveRoot(root)
	if err != nil {
		fmt.Fprintln(errOut, "sprout mcp:", err)
		return 1
	}
	fmt.Fprintf(errOut, "sprout %s: MCP server on stdio, root %s\n", resolveVersion(), root)

	enc := json.NewEncoder(out)
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 10<<20)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			enc.Encode(map[string]any{"jsonrpc": "2.0", "id": nil, "error": rpcError{-32700, "parse error"}})
			continue
		}
		result, rerr := handleMCP(root, req)
		if req.ID == nil {
			continue // notification: never answered
		}
		resp := map[string]any{"jsonrpc": "2.0", "id": req.ID}
		if rerr != nil {
			resp["error"] = rerr
		} else {
			resp["result"] = result
		}
		if err := enc.Encode(resp); err != nil {
			return 1
		}
	}
	return 0
}

func handleMCP(root string, req rpcRequest) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		json.Unmarshal(req.Params, &p)
		version := mcpVersions[0]
		if slices.Contains(mcpVersions, p.ProtocolVersion) {
			version = p.ProtocolVersion
		}
		return map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "sprout", "version": resolveVersion()},
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": mcpTools}, nil
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, &rpcError{-32602, "invalid params"}
		}
		i := slices.IndexFunc(mcpTools, func(t mcpTool) bool { return t.Name == p.Name })
		if i < 0 {
			return nil, &rpcError{-32602, "unknown tool: " + p.Name}
		}
		text, err := callTool(root, mcpTools[i], p.Arguments)
		if err != nil {
			return toolResult(err.Error(), true), nil
		}
		return toolResult(text, false), nil
	}
	if strings.HasPrefix(req.Method, "notifications/") {
		return nil, nil
	}
	return nil, &rpcError{-32601, "method not found: " + req.Method}
}

func toolResult(text string, isError bool) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": isError,
	}
}

func callTool(root string, tool mcpTool, raw json.RawMessage) (string, error) {
	var a toolArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", fmt.Errorf("invalid arguments: %v", err)
		}
	}
	dir, err := confine(root, a.Path)
	if err != nil {
		return "", err
	}
	args, err := tool.args(a)
	if err != nil {
		return "", err
	}
	var out, errOut bytes.Buffer
	if code := run(append(args, "--", dir), &out, &errOut); code != 0 {
		return "", fmt.Errorf("%s", strings.TrimSpace(errOut.String()))
	}
	// Show the path the agent asked for, not the user's absolute home path.
	shown := filepath.ToSlash(filepath.Join(".", a.Path))
	return strings.Replace(out.String(), dir, shown, 1), nil
}

func resolveRoot(root string) (string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// confine resolves p against root and refuses anything that lands outside
// it, including via "..", absolute paths, or symlinks. An agent's arguments
// are untrusted input.
func confine(root, p string) (string, error) {
	if filepath.IsAbs(p) {
		return "", fmt.Errorf("path must be relative to the project root")
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(root, p))
	if err != nil {
		return "", fmt.Errorf("path %q: %v", p, errReason(err))
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q is outside the project root", p)
	}
	return resolved, nil
}
