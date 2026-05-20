package cdp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/chromedp/cdproto/debugger"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

type scriptSourceGetter func(context.Context, runtime.ScriptID) (string, error)
type scriptURLCollector func(context.Context) ([]string, error)
type scriptReloader func(context.Context) error

// FindBreakpointLocation 从脚本源码中定位断点行列号。
func FindBreakpointLocation(scriptSource string, searchPattern string) (int, int, error) {
	patternIndex := strings.Index(scriptSource, searchPattern)
	if patternIndex < 0 {
		return 0, 0, fmt.Errorf("target code not found: %s", searchPattern)
	}
	start := patternIndex - 80
	if start < 0 {
		start = 0
	}
	end := patternIndex + len(searchPattern) + 120
	if end > len(scriptSource) {
		end = len(scriptSource)
	}
	fmt.Printf("[DEBUG] Breakpoint snippet: %s\n", scriptSource[start:end])

	lineNumber := strings.Count(scriptSource[:patternIndex], "\n")
	lastNewline := strings.LastIndex(scriptSource[:patternIndex], "\n")
	columnNumber := utf8.RuneCountInString(scriptSource[:patternIndex])
	if lastNewline >= 0 {
		columnNumber = utf8.RuneCountInString(scriptSource[lastNewline+1 : patternIndex])
	}
	return lineNumber, columnNumber, nil
}

// MatchScriptURL 判断脚本地址是否匹配目标模式。
func MatchScriptURL(scriptURL string, scriptPattern string) bool {
	normalizedURL := strings.ToLower(scriptURL)
	normalizedPattern := strings.ToLower(scriptPattern)
	return wildcardMatch(normalizedURL, "*"+normalizedPattern+"*")
}

// wildcardMatch 使用 * 通配符对字符串进行宽松匹配。
func wildcardMatch(value string, pattern string) bool {
	parts := strings.Split(pattern, "*")
	position := 0
	for index, part := range parts {
		if part == "" {
			continue
		}
		found := strings.Index(value[position:], part)
		if found < 0 {
			return false
		}
		if index == 0 && !strings.HasPrefix(pattern, "*") && found != 0 {
			return false
		}
		position += found + len(part)
	}
	if !strings.HasSuffix(pattern, "*") && len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		return strings.HasSuffix(value, lastPart)
	}
	return true
}

// CollectScriptURLs 提取页面中的脚本地址列表。
func CollectScriptURLs(ctx context.Context) ([]string, error) {
	var urls []string
	err := chromedp.Run(ctx, chromedp.Evaluate(`Array.from(document.getElementsByTagName('script')).map(item => item.src).filter(Boolean)`, &urls))
	return urls, err
}

// FetchScriptSourceViaHTTP 通过 HTTP 兜底获取脚本源码。
func FetchScriptSourceViaHTTP(ctx context.Context, scriptURL string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, scriptURL, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("fetch script failed: %s", response.Status)
	}
	content, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// GetScriptSourceFromDebugger 通过 Debugger.getScriptSource 获取脚本源码。
func GetScriptSourceFromDebugger(ctx context.Context, scriptID runtime.ScriptID) (string, error) {
	var scriptSource string
	err := chromedp.Run(ctx, getScriptSourceAction(scriptID, &scriptSource))
	return scriptSource, err
}

// getScriptSourceFromDebuggerDirect 在 chromedp 执行器上下文中获取脚本源码。
func getScriptSourceFromDebuggerDirect(ctx context.Context, scriptID runtime.ScriptID) (string, error) {
	scriptSource, _, err := debugger.GetScriptSource(scriptID).Do(ctx)
	return scriptSource, err
}

// getScriptSourceAction 将 Debugger.getScriptSource 包装为 chromedp Action。
func getScriptSourceAction(scriptID runtime.ScriptID, scriptSource *string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		value, err := getScriptSourceFromDebuggerDirect(ctx, scriptID)
		if err != nil {
			return err
		}
		*scriptSource = value
		return nil
	})
}

// PreparePreciseBreakpoint 查找目标脚本并计算精确断点位置。
func PreparePreciseBreakpoint(ctx context.Context, scriptPattern string, searchPattern string, scripts map[string]runtime.ScriptID) (string, int, int, error) {
	return PreparePreciseBreakpointWithDeps(ctx, scriptPattern, searchPattern, scripts, CollectScriptURLs, reloadScriptDiscoveryPage, GetScriptSourceFromDebugger)
}

// PreparePreciseBreakpointWithDeps 查找目标脚本并支持注入脚本收集、刷新与源码获取函数。
func PreparePreciseBreakpointWithDeps(ctx context.Context, scriptPattern string, searchPattern string, scripts map[string]runtime.ScriptID, collectURLs scriptURLCollector, reloadPage scriptReloader, getSource scriptSourceGetter) (string, int, int, error) {
	for attempt := 0; attempt < 2; attempt++ {
		if attempt == 1 {
			fmt.Println("[WARN] First attempt did not get scriptId, reloading page and retrying")
			if err := reloadPage(ctx); err != nil {
				return "", 0, 0, err
			}
			time.Sleep(2 * time.Second)
		}

		scriptURLs, err := collectURLs(ctx)
		if err != nil {
			return "", 0, 0, err
		}

		fmt.Printf("[INFO] Page script tag count: %d\n", len(scriptURLs))
		for _, scriptURL := range scriptURLs {
			fmt.Printf("[INFO]   %s\n", scriptURL)
		}

		for _, scriptURL := range scriptURLs {
			if !MatchScriptURL(scriptURL, scriptPattern) {
				continue
			}
			fmt.Printf("[SUCCESS] Target script found: %s\n", scriptURL)

			var scriptSource string
			scriptID, found := scripts[scriptURL]
			if found {
				scriptSource, err = getSource(ctx, scriptID)
				if err != nil {
					return "", 0, 0, err
				}
				fmt.Println("[INFO] Script source loaded from CDP")
			} else {
				fmt.Printf("[WARN] scriptId not found for target script: %s\n", scriptURL)
			}
			if scriptSource == "" {
				if attempt == 0 {
					break
				}
				fmt.Println("[WARN] scriptId not found, fallback to HTTP source")
				scriptSource, err = FetchScriptSourceViaHTTP(ctx, scriptURL)
				if err != nil {
					return "", 0, 0, err
				}
				fmt.Println("[INFO] Script source loaded from HTTP")
			}

			lineNumber, columnNumber, err := FindBreakpointLocation(scriptSource, searchPattern)
			if err != nil {
				return "", 0, 0, err
			}
			fmt.Printf("[INFO] Breakpoint location: line=%d, column=%d\n", lineNumber, columnNumber)
			return scriptURL, lineNumber, columnNumber, nil
		}
	}

	return "", 0, 0, fmt.Errorf("target script not found: %s", scriptPattern)
}

// reloadScriptDiscoveryPage 刷新页面，供脚本定位重试阶段复用。
func reloadScriptDiscoveryPage(ctx context.Context) error {
	return chromedp.Run(ctx, chromedp.ActionFunc(func(actionCtx context.Context) error {
		return page.Reload().Do(actionCtx)
	}))
}
