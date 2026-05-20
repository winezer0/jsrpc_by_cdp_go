package cdp

import "testing"

// TestDevToolsURL 验证 DevTools HTTP 地址构造结果。
func TestDevToolsURL(t *testing.T) {
	got := devToolsURL(9222, "/json/list")
	want := "http://127.0.0.1:9222/json/list"
	if got != want {
		t.Fatalf("unexpected devtools url: got %q want %q", got, want)
	}
}

// TestFindPageTargetByURL 验证只返回完全匹配 URL 的 page target。
func TestFindPageTargetByURL(t *testing.T) {
	targetID, found := FindPageTargetByURL([]pageTargetInfo{
		{ID: "target-1", Type: "service_worker", URL: "https://mgmt.huashangjk.com/#/login"},
		{ID: "target-2", Type: "page", URL: "https://mgmt.huashangjk.com/#/login"},
	}, "https://mgmt.huashangjk.com/#/login")
	if !found {
		t.Fatal("expected target to be found")
	}
	if targetID != "target-2" {
		t.Fatalf("unexpected target id: %s", targetID)
	}
}

// TestFindPageTargetByURLRejectsNonPage 验证非 page 类型不会被误选。
func TestFindPageTargetByURLRejectsNonPage(t *testing.T) {
	_, found := FindPageTargetByURL([]pageTargetInfo{
		{ID: "target-1", Type: "browser", URL: "https://mgmt.huashangjk.com/#/login"},
	}, "https://mgmt.huashangjk.com/#/login")
	if found {
		t.Fatal("expected target lookup to ignore non-page target")
	}
}
