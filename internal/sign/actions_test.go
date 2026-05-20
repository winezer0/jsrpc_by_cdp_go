package sign

import "testing"

// TestParseJSONResultReturnsMap 验证 JSON 字符串可以解析为 map。
func TestParseJSONResultReturnsMap(t *testing.T) {
	result, err := ParseJSONResult(`{"success": true}`, "parse failed")
	if err != nil {
		t.Fatalf("ParseJSONResult returned error: %v", err)
	}
	if success, _ := result["success"].(bool); !success {
		t.Fatalf("unexpected parse result: %#v", result)
	}
}

// TestParseJSONResultRejectsEmptyValue 验证空返回值会报错。
func TestParseJSONResultRejectsEmptyValue(t *testing.T) {
	_, err := ParseJSONResult("", "parse failed")
	if err == nil {
		t.Fatal("expected error for empty value")
	}
}

// TestParseJSONResultRejectsInvalidJSON 验证非法 JSON 会报错。
func TestParseJSONResultRejectsInvalidJSON(t *testing.T) {
	_, err := ParseJSONResult("invalid", "parse failed")
	if err == nil {
		t.Fatal("expected error for invalid json")
	}
}

// TestReadJSSourceRejectsEmptyPath 验证空 JS 路径会报错。
func TestReadJSSourceRejectsEmptyPath(t *testing.T) {
	_, err := ReadJSSource("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}
