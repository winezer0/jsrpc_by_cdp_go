package cdp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/chromedp/cdproto/target"
)

// devToolsURL 构造与 Python 版一致的本地 DevTools HTTP 地址。
func devToolsURL(port int, endpoint string) string {
	return fmt.Sprintf("http://127.0.0.1:%d%s", port, endpoint)
}

// readDevToolsJSON 读取并解析 DevTools HTTP 接口返回的 JSON。
func readDevToolsJSON(ctx context.Context, method string, requestURL string, destination any) error {
	request, err := http.NewRequestWithContext(ctx, method, requestURL, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("devtools http request failed: %s", response.Status)
	}
	return json.NewDecoder(response.Body).Decode(destination)
}

// ListPageTargets 获取全部 page 类型 target 信息。
func ListPageTargets(ctx context.Context, port int) ([]pageTargetInfo, error) {
	var targets []pageTargetInfo
	if err := readDevToolsJSON(ctx, http.MethodGet, devToolsURL(port, "/json/list"), &targets); err != nil {
		return nil, err
	}
	return targets, nil
}

// FindPageTargetByURL 按 URL 查找匹配的页面 target。
func FindPageTargetByURL(targets []pageTargetInfo, targetURL string) (target.ID, bool) {
	for _, item := range targets {
		if item.Type == "page" && item.URL == targetURL {
			return item.ID, true
		}
	}
	return "", false
}

// CreatePageTarget 按 Python 版逻辑创建 about:blank 页面 target。
func CreatePageTarget(ctx context.Context, port int) (target.ID, error) {
	var targetInfo pageTargetInfo
	requestURL := devToolsURL(port, "/json/new?"+url.QueryEscape("about:blank"))
	err := readDevToolsJSON(ctx, http.MethodPut, requestURL, &targetInfo)
	if err != nil {
		err = readDevToolsJSON(ctx, http.MethodGet, requestURL, &targetInfo)
	}
	if err != nil {
		return "", err
	}
	if targetInfo.ID == "" {
		return "", fmt.Errorf("created page target id is empty")
	}
	return targetInfo.ID, nil
}

// GetOrCreatePageTargetID 对齐 Python 版获取或创建页面 target 的流程。
func GetOrCreatePageTargetID(ctx context.Context, port int, targetURL string) (target.ID, error) {
	targets, err := ListPageTargets(ctx, port)
	if err != nil {
		return "", err
	}
	if targetID, found := FindPageTargetByURL(targets, targetURL); found {
		return targetID, nil
	}

	targetID, err := CreatePageTarget(ctx, port)
	if err != nil {
		return "", err
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		targets, err = ListPageTargets(ctx, port)
		if err != nil {
			return "", err
		}
		for _, item := range targets {
			if item.ID == targetID && item.Type == "page" {
				return targetID, nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return "", fmt.Errorf("can not find created page target: %s", targetID)
}
