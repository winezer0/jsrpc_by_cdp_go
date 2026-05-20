package chrome

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// IsCDPServerReady 检测本地 Chrome CDP HTTP 服务是否可访问。
func IsCDPServerReady(ctx context.Context, port int) bool {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/json/version", port), nil)
	if err != nil {
		return false
	}

	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()

	return response.StatusCode == http.StatusOK
}

// BuildChromeCommand 构建启动 Chrome CDP 服务需要的命令行参数。
func BuildChromeCommand(chromePath string, port int, userDataDir string, headless bool) []string {
	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--no-first-run",
		"--no-default-browser-check",
		fmt.Sprintf("--user-data-dir=%s", userDataDir),
	}
	if headless {
		args = append(args, "--headless=new")
	}
	return append([]string{chromePath}, args...)
}

// StartChromeWithCDP 启动带远程调试端口的 Chrome 进程。
func StartChromeWithCDP(chromePath string, port int, userDataDir string, headless bool) (*exec.Cmd, error) {
	if _, err := os.Stat(chromePath); err != nil {
		return nil, fmt.Errorf("Chrome executable not found: %s", chromePath)
	}
	if err := os.MkdirAll(filepath.Clean(userDataDir), 0755); err != nil {
		return nil, err
	}

	command := BuildChromeCommand(chromePath, port, userDataDir, headless)
	cmd := exec.Command(command[0], command[1:]...)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

// EnsureCDPServer 确保 Chrome CDP 服务已启动并可访问。
func EnsureCDPServer(ctx context.Context, port int, chromePath string, userDataDir string, startupTimeout time.Duration, headless bool) error {
	if IsCDPServerReady(ctx, port) {
		fmt.Printf("[SUCCESS] CDP server is ready: http://127.0.0.1:%d\n", port)
		return nil
	}

	fmt.Printf("[WARN] CDP server is not running: http://127.0.0.1:%d\n", port)
	fmt.Printf("[INFO] Starting Chrome: %s\n", chromePath)
	if _, err := StartChromeWithCDP(chromePath, port, userDataDir, headless); err != nil {
		return err
	}

	deadline := time.Now().Add(startupTimeout)
	for time.Now().Before(deadline) {
		if IsCDPServerReady(ctx, port) {
			fmt.Printf("[SUCCESS] CDP server started: http://127.0.0.1:%d\n", port)
			return nil
		}
		time.Sleep(time.Second)
	}

	return fmt.Errorf("Chrome CDP server startup timeout: http://127.0.0.1:%d", port)
}
