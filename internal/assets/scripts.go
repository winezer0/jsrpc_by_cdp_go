package assets

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"jsrpcbycdp/internal/sign"
)

const defaultExportDir = "assets"

var defaultScriptNames = []string{
	"js_env.js",
	"js_env_verify.js",
	"promote_func.js",
	"promote_func_verify.js",
	"reg_action.js",
	"reg_action_context.js",
	"reg_action_verify.js",
}

//go:embed *.js
var embeddedScripts embed.FS

// DefaultScriptNames 返回默认内嵌脚本文件名列表。
func DefaultScriptNames() []string {
	return append([]string(nil), defaultScriptNames...)
}

// DefaultExportDir 返回默认导出目录。
func DefaultExportDir() string {
	return defaultExportDir
}

// ExportDefaultScripts 将内嵌脚本导出到目标目录。
func ExportDefaultScripts(dstDir string) error {
	if dstDir == "" {
		return fmt.Errorf("destination directory can not be empty")
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	for _, name := range defaultScriptNames {
		content, err := embeddedScripts.ReadFile(name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dstDir, name), content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// EnsureDefaultScripts 在目录不存在时自动写出默认脚本，并返回脚本路径集合。
func EnsureDefaultScripts(jsDir string) (sign.ScriptPaths, error) {
	if jsDir == "" {
		jsDir = defaultExportDir
	}
	if _, err := os.Stat(jsDir); err != nil {
		if !os.IsNotExist(err) {
			return sign.ScriptPaths{}, err
		}
		if err := ExportDefaultScripts(jsDir); err != nil {
			return sign.ScriptPaths{}, err
		}
	}
	return ScriptPaths(jsDir), nil
}

// ScriptPaths 根据目录生成脚本路径集合。
func ScriptPaths(jsDir string) sign.ScriptPaths {
	return sign.ScriptPaths{
		JSEnv:            filepath.Join(jsDir, "js_env.js"),
		JSEnvVerify:      filepath.Join(jsDir, "js_env_verify.js"),
		PromoteFunc:      filepath.Join(jsDir, "promote_func.js"),
		PromoteVerify:    filepath.Join(jsDir, "promote_func_verify.js"),
		RegActionContext: filepath.Join(jsDir, "reg_action_context.js"),
		RegActionVerify:  filepath.Join(jsDir, "reg_action_verify.js"),
		RegAction:        filepath.Join(jsDir, "reg_action.js"),
	}
}
