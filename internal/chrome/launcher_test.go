package chrome

import "testing"

// TestBuildChromeCommandIncludesCDPFlags 验证 Chrome 启动参数包含 CDP 必需配置。
func TestBuildChromeCommandIncludesCDPFlags(t *testing.T) {
	command := BuildChromeCommand("chrome.exe", 9222, `C:\profile`, true)
	want := []string{"chrome.exe", "--remote-debugging-port=9222", "--no-first-run", "--no-default-browser-check", `--user-data-dir=C:\profile`, "--headless=new"}
	if len(command) != len(want) {
		t.Fatalf("unexpected command length: %#v", command)
	}
	for index := range want {
		if command[index] != want[index] {
			t.Fatalf("unexpected command at %d: got %q want %q", index, command[index], want[index])
		}
	}
}
