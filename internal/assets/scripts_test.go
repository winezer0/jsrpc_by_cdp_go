package assets

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDefaultScriptNames 验证默认脚本列表完整。
func TestDefaultScriptNames(t *testing.T) {
	names := DefaultScriptNames()
	if len(names) != 7 {
		t.Fatalf("unexpected default script count: %d", len(names))
	}
}

// TestExportDefaultScripts 验证内嵌脚本可以导出到目标目录。
func TestExportDefaultScripts(t *testing.T) {
	dstDir := t.TempDir()
	if err := ExportDefaultScripts(dstDir); err != nil {
		t.Fatalf("ExportDefaultScripts returned error: %v", err)
	}
	for _, name := range DefaultScriptNames() {
		fullPath := filepath.Join(dstDir, name)
		info, err := os.Stat(fullPath)
		if err != nil {
			t.Fatalf("expected exported file %s: %v", name, err)
		}
		if info.Size() == 0 {
			t.Fatalf("expected non-empty file: %s", name)
		}
	}
}

// TestEnsureDefaultScripts 验证目录不存在时会自动导出默认脚本。
func TestEnsureDefaultScripts(t *testing.T) {
	baseDir := t.TempDir()
	jsDir := filepath.Join(baseDir, "missing-js")
	paths, err := EnsureDefaultScripts(jsDir)
	if err != nil {
		t.Fatalf("EnsureDefaultScripts returned error: %v", err)
	}
	if _, err := os.Stat(paths.JSEnv); err != nil {
		t.Fatalf("expected exported js_env.js: %v", err)
	}
	if paths.RegAction != filepath.Join(jsDir, "reg_action.js") {
		t.Fatalf("unexpected reg action path: %s", paths.RegAction)
	}
}
