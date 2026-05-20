package sign

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chromedp/cdproto/debugger"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

type JSONResult map[string]any

type PauseFrameSnapshot struct {
	Index        int    `json:"index"`
	FunctionName string `json:"functionName"`
	ScriptID     string `json:"scriptId"`
	LineNumber   int64  `json:"lineNumber"`
	ColumnNumber int64  `json:"columnNumber"`
	PType        string `json:"pType"`
	PAType       string `json:"pAType"`
	GetBaseType  string `json:"getBaseType"`
}

// ReadJSSource 读取外部 JS 源码文件。
func ReadJSSource(jsPath string) (string, error) {
	if strings.TrimSpace(jsPath) == "" {
		return "", fmt.Errorf("JS path can not be empty")
	}
	content, err := os.ReadFile(filepath.Clean(jsPath))
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// ParseJSONResult 解析页面返回的 JSON 字符串。
func ParseJSONResult(value string, errorMessage string) (JSONResult, error) {
	if value == "" {
		return nil, fmt.Errorf("%s: empty result", errorMessage)
	}
	var result JSONResult
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return nil, fmt.Errorf("%s: %s", errorMessage, value)
	}
	return result, nil
}

// evaluateOnCallFrameAction 在 chromedp 执行器上下文中执行暂停栈帧表达式。
func evaluateOnCallFrameAction(callFrameID debugger.CallFrameID, expression string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		_, exceptionDetails, err := debugger.EvaluateOnCallFrame(callFrameID, expression).WithReturnByValue(true).WithSilent(false).Do(ctx)
		if err != nil {
			return err
		}
		if exceptionDetails != nil {
			return fmt.Errorf("evaluate on call frame failed: %s", exceptionDetails.Text)
		}
		return nil
	})
}

// evalRuntimeAction 在 chromedp 执行器上下文中执行 Runtime.evaluate。
func evalRuntimeAction(expression string, value *string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		remoteObject, exceptionDetails, err := runtime.Evaluate(expression).WithReturnByValue(true).WithIncludeCommandLineAPI(true).Do(ctx)
		if err != nil {
			return err
		}
		if exceptionDetails != nil {
			return fmt.Errorf("runtime evaluate failed: %s", exceptionDetails.Text)
		}
		if remoteObject.Value == nil {
			*value = ""
			return nil
		}
		var result string
		if err := json.Unmarshal(remoteObject.Value, &result); err == nil {
			*value = result
			return nil
		}
		*value = string(remoteObject.Value)
		return nil
	})
}

// EvalJS 在页面运行时执行 JS 表达式并返回字符串结果。
func EvalJS(ctx context.Context, expression string) (string, error) {
	var value string
	err := chromedp.Run(ctx, evalRuntimeAction(expression, &value))
	return value, err
}

// EvalJSOnCallFrame 在调试暂停栈帧中执行 JS 表达式。
func EvalJSOnCallFrame(ctx context.Context, callFrameID debugger.CallFrameID, expression string) error {
	return chromedp.Run(ctx, evaluateOnCallFrameAction(callFrameID, expression))
}

// InjectJSEnv 向页面注入本地 JS RPC 环境并校验结果。
func InjectJSEnv(ctx context.Context, jsEnvPath string, verifyPath string) error {
	jsSource, err := ReadJSSource(jsEnvPath)
	if err != nil {
		return err
	}
	if _, err := EvalJS(ctx, jsSource); err != nil {
		return err
	}

	verifyCode, err := ReadJSSource(verifyPath)
	if err != nil {
		return err
	}
	value, err := EvalJS(ctx, verifyCode)
	if err != nil {
		return err
	}
	fmt.Printf("[INFO] js_env.js injection result: %s\n", value)
	parsed, err := ParseJSONResult(value, "js_env.js injection failed")
	if err != nil {
		return err
	}
	if success, _ := parsed["success"].(bool); !success {
		return fmt.Errorf("js_env.js injection failed: %s", value)
	}
	fmt.Println("[SUCCESS] js_env.js injected")
	return nil
}

// TestGlobalFunction 校验 sign 函数是否已经提升到全局。
func TestGlobalFunction(ctx context.Context, verifyPath string) error {
	verifyCode, err := ReadJSSource(verifyPath)
	if err != nil {
		return err
	}
	value, err := EvalJS(ctx, verifyCode)
	if err != nil {
		return err
	}
	fmt.Printf("[INFO] Test result: %s\n", value)
	parsed, err := ParseJSONResult(value, "global function verification failed")
	if err != nil {
		return err
	}
	if success, _ := parsed["success"].(bool); !success {
		return fmt.Errorf("global function verification failed: %v", parsed["message"])
	}
	fmt.Printf("[SUCCESS] sign.getBaseHeader(e).signatureStr = %v\n", parsed["signatureStr"])
	return nil
}

// PromoteSignToGlobal 在断点暂停上下文中将 sign 提升到全局。
func PromoteSignToGlobal(ctx context.Context, paused *debugger.EventPaused, promotePath string) error {
	if paused == nil || len(paused.CallFrames) == 0 {
		return fmt.Errorf("debugger paused without call frames")
	}
	promoteCode, err := ReadJSSource(promotePath)
	if err != nil {
		return err
	}
	if err := EvalJSOnCallFrame(ctx, paused.CallFrames[0].CallFrameID, promoteCode); err != nil {
		return err
	}
	fmt.Println("[SUCCESS] sign promotion code executed in paused context")
	return nil
}

// InspectSignRegisterContext 检查注册 sign 动作需要的页面上下文。
func InspectSignRegisterContext(ctx context.Context, contextPath string) ([]string, error) {
	validateCode, err := ReadJSSource(contextPath)
	if err != nil {
		return nil, err
	}
	value, err := EvalJS(ctx, validateCode)
	if err != nil {
		return nil, err
	}
	fmt.Printf("[INFO] sign action context check: %s\n", value)
	contextResult, err := ParseJSONResult(value, "sign action context check failed")
	if err != nil {
		return nil, err
	}
	missing, _ := contextResult["missingItems"].([]any)
	items := make([]string, 0, len(missing))
	for _, item := range missing {
		if text, ok := item.(string); ok {
			items = append(items, text)
		}
	}
	return items, nil
}

// RegisterSignAction 在页面上下文中注册 sign 动作。
func RegisterSignAction(ctx context.Context, paths ScriptPaths) error {
	missingItems, err := InspectSignRegisterContext(ctx, paths.RegActionContext)
	if err != nil {
		return err
	}
	if contains(missingItems, "demo") || contains(missingItems, "demo.regAction") {
		fmt.Println("[WARN] demo environment is missing, reinjecting js_env.js")
		if err := InjectJSEnv(ctx, paths.JSEnv, paths.JSEnvVerify); err != nil {
			return err
		}
		missingItems, err = InspectSignRegisterContext(ctx, paths.RegActionContext)
		if err != nil {
			return err
		}
	}
	if len(missingItems) > 0 {
		return fmt.Errorf("sign action prerequisites missing: %s", strings.Join(missingItems, ", "))
	}

	registerCode, err := ReadJSSource(paths.RegAction)
	if err != nil {
		return err
	}
	value, err := EvalJS(ctx, registerCode)
	if err != nil {
		return err
	}
	fmt.Printf("[INFO] sign action register result: %s\n", value)
	return VerifySignAction(ctx, paths.RegActionVerify)
}

// VerifySignAction 校验 sign 动作是否已成功注册。
func VerifySignAction(ctx context.Context, verifyPath string) error {
	verifyCode, err := ReadJSSource(verifyPath)
	if err != nil {
		return err
	}
	value, err := EvalJS(ctx, verifyCode)
	if err != nil {
		return err
	}
	fmt.Printf("[INFO] sign action verify result: %s\n", value)
	verifyData, err := ParseJSONResult(value, "sign action verification failed")
	if err != nil {
		return err
	}
	if hasSignAction, _ := verifyData["hasSignAction"].(bool); !hasSignAction {
		return fmt.Errorf("sign action registration failed: %s", value)
	}
	fmt.Println("[SUCCESS] demo.regAction(\"sign\") registered")
	return nil
}

// contains 判断字符串切片是否包含目标值。
func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
