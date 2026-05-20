package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/chromedp/cdproto/debugger"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/jessevdk/go-flags"

	"jsrpcbycdp/internal/assets"
	projectcdp "jsrpcbycdp/internal/cdp"
	"jsrpcbycdp/internal/chrome"
	"jsrpcbycdp/internal/sign"
)

type options struct {
	Port                 int    `long:"port" default:"9222" description:"CDP debug port"`
	URL                  string `long:"url" default:"https://mgmt.huashangjk.com/#/login" description:"Target URL"`
	Script               string `long:"script" default:"app.*.js" description:"Target script match pattern"`
	Pattern              string `long:"pattern" default:"a=p[\"a\"]" description:"Target code snippet"`
	PageLoadTimeout      int    `long:"page-load-timeout" default:"60" description:"Page load timeout seconds"`
	ChromePath           string `long:"chrome-path" default:"C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe" description:"Chrome executable path"`
	UserDataDir          string `long:"user-data-dir" default:"C:\\chrome-debug-profile" description:"Chrome user data directory"`
	ChromeStartupTimeout int    `long:"chrome-startup-timeout" default:"15" description:"Chrome CDP startup timeout seconds"`
	Headless             bool   `long:"headless" description:"Start Chrome in headless mode"`
	JSDir                string `long:"js-dir" description:"Directory containing reusable JS files"`
	Gen                  bool   `long:"gen" description:"Generate default embedded JS files to js-dir or assets directory"`
}

// main 解析命令行参数并运行 Go 版本 jsrpcbycdp 流程。
func main() {
	var opts options
	parser := flags.NewParser(&opts, flags.Default)
	if _, err := parser.Parse(); err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && errors.Is(flagsErr.Type, flags.ErrHelp) {
			return
		}
		os.Exit(1)
	}
	if err := run(opts); err != nil {
		fmt.Printf("\n[ERROR] Execution failed: %v\n", err)
		os.Exit(1)
	}
}

// run 编排 Chrome 启动、页面调试、断点提升与 JS RPC 注册流程。
func run(opts options) error {
	if opts.Gen {
		return generateScripts(opts)
	}
	fmt.Println("============================================================")
	fmt.Println("Chrome CDP Browser Ops for jsrpcbycdp")
	fmt.Println("============================================================")

	baseCtx := context.Background()
	fmt.Printf("\n[Step 0] Checking Chrome CDP server (port: %d)\n", opts.Port)
	if err := chrome.EnsureCDPServer(baseCtx, opts.Port, opts.ChromePath, opts.UserDataDir, time.Duration(opts.ChromeStartupTimeout)*time.Second, opts.Headless); err != nil {
		return err
	}

	allocatorCtx, _ := chromedp.NewRemoteAllocator(baseCtx, fmt.Sprintf("http://127.0.0.1:%d", opts.Port))

	targetID, err := projectcdp.GetOrCreatePageTargetID(baseCtx, opts.Port, opts.URL)
	if err != nil {
		return err
	}

	ctx, _ := chromedp.NewContext(allocatorCtx, chromedp.WithTargetID(targetID))
	if err := chromedp.Run(ctx); err != nil {
		return err
	}
	fmt.Printf("[INFO] Attached page target: %s\n", targetID)

	if err := runBrowserFlow(ctx, opts); err != nil {
		return err
	}
	return nil
}

// runBrowserFlow 执行页面级 CDP 操作。
func runBrowserFlow(ctx context.Context, opts options) error {
	parsedScripts := make(map[string]runtime.ScriptID)
	parsedScriptURLs := make([]string, 0, 16)
	var scriptsMu sync.Mutex
	pausedChan := make(chan *debugger.EventPaused, 1)

	chromedp.ListenTarget(ctx, func(event any) {
		switch ev := event.(type) {
		case *debugger.EventScriptParsed:
			if ev.URL == "" {
				return
			}
			scriptsMu.Lock()
			if _, exists := parsedScripts[ev.URL]; !exists {
				parsedScriptURLs = append(parsedScriptURLs, ev.URL)
			}
			parsedScripts[ev.URL] = ev.ScriptID
			scriptsMu.Unlock()
		case *debugger.EventPaused:
			select {
			case pausedChan <- ev:
			default:
			}
		}
	})

	fmt.Println("\n[Step 1] Enabling CDP domains")
	if err := chromedp.Run(ctx, page.Enable(), runtime.Enable(), enableDebuggerAction(), network.Enable()); err != nil {
		return err
	}
	fmt.Println("[INFO] Page, Runtime, Debugger and Network domains enabled")
	paths, err := resolveScriptPaths(opts.JSDir)
	if err != nil {
		return err
	}

	fmt.Println("\n[Step 2] Opening target page")
	scriptsMu.Lock()
	clear(parsedScripts)
	parsedScriptURLs = parsedScriptURLs[:0]
	scriptsMu.Unlock()
	loadCtx, cancelLoad := context.WithTimeout(ctx, time.Duration(opts.PageLoadTimeout)*time.Second)
	defer cancelLoad()
	if err := chromedp.Run(loadCtx, chromedp.Navigate(opts.URL)); err != nil {
		return err
	}
	time.Sleep(3 * time.Second)
	fmt.Println("[SUCCESS] Page loaded")

	fmt.Println("\n[Step 3] Finding script and setting precise breakpoint")
	scriptsMu.Lock()
	scriptSnapshot := make(map[string]runtime.ScriptID, len(parsedScripts))
	for scriptURL, scriptID := range parsedScripts {
		scriptSnapshot[scriptURL] = scriptID
	}
	parsedScriptSnapshot := append([]string(nil), parsedScriptURLs...)
	scriptsMu.Unlock()
	fmt.Printf("[INFO] Debugger.scriptParsed captured JS count: %d\n", len(parsedScriptSnapshot))
	for _, scriptURL := range parsedScriptSnapshot {
		fmt.Printf("[INFO]   %s\n", scriptURL)
	}
	scriptURL, lineNumber, columnNumber, err := projectcdp.PreparePreciseBreakpoint(ctx, opts.Script, opts.Pattern, scriptSnapshot)
	if err != nil {
		return err
	}
	var breakpointID debugger.BreakpointID
	if err := chromedp.Run(ctx, setPreciseBreakpointAction(scriptURL, lineNumber, columnNumber, &breakpointID)); err != nil {
		return err
	}
	fmt.Printf("[SUCCESS] Breakpoint set: %s\n", breakpointID)

	fmt.Println("\n[Step 4] Reloading page to trigger breakpoint")
	if err := chromedp.Run(ctx, reloadPageAction()); err != nil {
		return err
	}
	var paused *debugger.EventPaused
	deadline := time.After(30 * time.Second)
	for paused == nil {
		select {
		case candidate := <-pausedChan:
			if !containsBreakpoint(candidate.HitBreakpoints, breakpointID) {
				fmt.Printf("[DEBUG] Ignored paused event (reason=%s, hitBreakpoints=%v)\n", candidate.Reason, candidate.HitBreakpoints)
				if err := chromedp.Run(ctx, resumeDebuggerAction()); err != nil {
					return err
				}
				continue
			}
			paused = candidate
			fmt.Printf("[SUCCESS] Breakpoint triggered (reason=%s, frames=%d, hitBreakpoints=%v)\n", paused.Reason, len(paused.CallFrames), paused.HitBreakpoints)
		case <-deadline:
			return fmt.Errorf("waiting for debugger pause timeout")
		}
	}

	fmt.Println("\n[Step 5] Executing JS code at breakpoint")
	if err := sign.PromoteSignToGlobal(ctx, paused, paths.PromoteFunc); err != nil {
		return err
	}
	if err := sign.TestGlobalFunction(ctx, paths.PromoteVerify); err != nil {
		return err
	}
	if err := sign.InjectJSEnv(ctx, paths.JSEnv, paths.JSEnvVerify); err != nil {
		return err
	}
	if err := sign.RegisterSignAction(ctx, paths); err != nil {
		return err
	}

	fmt.Println("\n[Step 6] Resuming browser execution")
	if err := chromedp.Run(ctx, resumeDebuggerAction()); err != nil {
		return err
	}
	fmt.Println("[SUCCESS] Debugger.resume sent")

	fmt.Println("\n============================================================")
	fmt.Println("[SUCCESS] All operations completed")
	fmt.Println("[INFO] Browser and program will keep running")
	fmt.Println("============================================================")
	select {}
}

// containsBreakpoint 判断 paused 事件中是否命中了目标断点。
func containsBreakpoint(hitBreakpoints []string, breakpointID debugger.BreakpointID) bool {
	for _, item := range hitBreakpoints {
		if item == breakpointID.String() {
			return true
		}
	}
	return false
}

// enableDebuggerAction 启用 Debugger 域并适配新版 cdproto 的返回值签名。
func enableDebuggerAction() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		_, err := debugger.Enable().Do(ctx)
		return err
	})
}

// reloadPageAction 刷新页面并保持 chromedp Action 执行器上下文。
func reloadPageAction() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		return page.Reload().Do(ctx)
	})
}

// setPreciseBreakpointAction 按脚本 URL 与行列号设置精确断点。
func setPreciseBreakpointAction(scriptURL string, lineNumber int, columnNumber int, breakpointID *debugger.BreakpointID) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		result, locations, err := debugger.SetBreakpointByURL(int64(lineNumber)).WithURL(scriptURL).WithColumnNumber(int64(columnNumber)).Do(ctx)
		if err != nil {
			return err
		}
		*breakpointID = result
		fmt.Printf("[INFO] Requested breakpoint position: line=%d, column=%d\n", lineNumber, columnNumber)
		for index, location := range locations {
			fmt.Printf("[INFO] Resolved breakpoint[%d]: line=%d, column=%d, scriptId=%s\n", index, location.LineNumber, location.ColumnNumber, location.ScriptID)
		}
		return nil
	})
}

// resumeDebuggerAction 恢复浏览器调试器执行。
func resumeDebuggerAction() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		return debugger.Resume().Do(ctx)
	})
}

// generateScripts 根据参数导出默认内嵌脚本。
func generateScripts(opts options) error {
	dstDir := opts.JSDir
	if dstDir == "" {
		dstDir = assets.DefaultExportDir()
	}
	if err := assets.ExportDefaultScripts(dstDir); err != nil {
		return err
	}
	fmt.Printf("[SUCCESS] Default scripts generated to: %s\n", dstDir)
	return nil
}

// resolveScriptPaths 解析需要复用的 JS 文件路径。
func resolveScriptPaths(jsDir string) (sign.ScriptPaths, error) {
	return assets.EnsureDefaultScripts(jsDir)
}
