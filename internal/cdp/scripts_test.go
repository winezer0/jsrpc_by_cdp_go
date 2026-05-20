package cdp

import (
	"context"
	"testing"

	"github.com/chromedp/cdproto/runtime"
)

// TestFindBreakpointLocationUsesSearchPattern 验证断点位置直接使用传入的搜索片段。
func TestFindBreakpointLocationUsesSearchPattern(t *testing.T) {
	lineNumber, columnNumber, err := FindBreakpointLocation("const first = 1;\nconst marker = \"ok\";\nconst tail = 3;", "marker = \"ok\"")
	if err != nil {
		t.Fatalf("FindBreakpointLocation returned error: %v", err)
	}
	if lineNumber != 1 || columnNumber != 6 {
		t.Fatalf("unexpected location: line=%d column=%d", lineNumber, columnNumber)
	}
}

// TestFindBreakpointLocationRejectsMissingPattern 验证搜索片段不存在时返回错误。
func TestFindBreakpointLocationRejectsMissingPattern(t *testing.T) {
	_, _, err := FindBreakpointLocation("const value = 1;", "missing")
	if err == nil {
		t.Fatal("expected error for missing pattern")
	}
}

// TestFindBreakpointLocationUsesRuneColumn 验证断点列号按字符而不是字节计算。
func TestFindBreakpointLocationUsesRuneColumn(t *testing.T) {
	lineNumber, columnNumber, err := FindBreakpointLocation("const 文本 = 1;marker()", "marker")
	if err != nil {
		t.Fatalf("FindBreakpointLocation returned error: %v", err)
	}
	if lineNumber != 0 || columnNumber != 13 {
		t.Fatalf("unexpected rune-based location: line=%d column=%d", lineNumber, columnNumber)
	}
}

// TestMatchScriptURLIgnoresCase 验证脚本匹配忽略大小写。
func TestMatchScriptURLIgnoresCase(t *testing.T) {
	if !MatchScriptURL("https://example.com/static/App.123.JS", "app.*.js") {
		t.Fatal("expected script URL to match")
	}
}

// TestPreparePreciseBreakpointReloadsAndRetries 验证首次未命中 scriptId 时会自动刷新后重试。
func TestPreparePreciseBreakpointReloadsAndRetries(t *testing.T) {
	ctx := context.Background()
	scripts := map[string]runtime.ScriptID{
		"https://example.com/app.123.js": "script-1",
	}
	collectCalls := 0
	reloadCalls := 0
	getSourceCalls := 0

	collectURLs := func(_ context.Context) ([]string, error) {
		collectCalls++
		return []string{"https://example.com/app.123.js"}, nil
	}
	reloadPage := func(_ context.Context) error {
		reloadCalls++
		return nil
	}
	getSource := func(_ context.Context, scriptID runtime.ScriptID) (string, error) {
		getSourceCalls++
		if getSourceCalls == 1 {
			return "", nil
		}
		if scriptID != "script-1" {
			t.Fatalf("unexpected script id: %s", scriptID)
		}
		return `const marker = 1; a=p["a"];`, nil
	}

	scriptURL, lineNumber, columnNumber, err := PreparePreciseBreakpointWithDeps(ctx, "app.*.js", `a=p["a"]`, scripts, collectURLs, reloadPage, getSource)
	if err != nil {
		t.Fatalf("PreparePreciseBreakpointWithDeps returned error: %v", err)
	}
	if scriptURL != "https://example.com/app.123.js" {
		t.Fatalf("unexpected script url: %s", scriptURL)
	}
	if lineNumber != 0 || columnNumber != 18 {
		t.Fatalf("unexpected breakpoint location: line=%d column=%d", lineNumber, columnNumber)
	}
	if collectCalls != 2 {
		t.Fatalf("expected collect URLs to be called twice, got %d", collectCalls)
	}
	if reloadCalls != 1 {
		t.Fatalf("expected reload to be called once, got %d", reloadCalls)
	}
	if getSourceCalls != 2 {
		t.Fatalf("expected getSource to be called twice, got %d", getSourceCalls)
	}
}
