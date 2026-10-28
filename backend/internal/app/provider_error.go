package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const contentModerationErrorCode = "sensitive_words_detected"

// 只提取供应商明确返回的错误码和短消息，避免把完整响应或用户输入复制到调用日志。
func providerFailureDetails(payload map[string]any) (string, string) {
	candidates := make([]map[string]any, 0, 3)
	for _, key := range []string{"error", "data"} {
		if nested, ok := payload[key].(map[string]any); ok {
			candidates = append(candidates, nested)
		}
	}
	// 内层通常是供应商业务错误，外层 code 可能只是 HTTP 包装码。
	candidates = append(candidates, payload)
	code := ""
	message := ""
	for _, candidate := range candidates {
		if code == "" {
			code = normalizedProviderErrorCode(candidate["code"])
		}
		if message == "" {
			message = strings.TrimSpace(stringField(candidate, "message"))
			if message == "" {
				message = strings.TrimSpace(stringField(candidate, "msg"))
			}
		}
	}
	return code, truncateRunes(message, 500)
}

func providerResponseBusinessFailure(responseBody []byte) (string, string, bool) {
	if len(responseBody) == 0 {
		return "", "", false
	}
	// 火山单向语音合成把多帧 JSON 直接拼在同一个 HTTP 200 里。只解析第一帧会把后面的业务失败当成成功。
	decoder := json.NewDecoder(bytes.NewReader(responseBody))
	decoder.UseNumber()
	decoded := false
	for {
		var payload map[string]any
		err := decoder.Decode(&payload)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if !decoded {
				return "", "", false
			}
			break
		}
		decoded = true
		if payload == nil {
			continue
		}
		if code, message, failed := providerPayloadBusinessFailure(payload); failed {
			return code, message, true
		}
	}
	return "", "", false
}

func providerPayloadBusinessFailure(payload map[string]any) (string, string, bool) {
	if header, ok := payload["header"].(map[string]any); ok {
		// 只认火山语音合成这种明确的资源拒绝。header.code 在别的协议里可能是 HTTP 状态，不能一律当成业务失败。
		rawMessage := strings.TrimSpace(stringField(header, "message"))
		if speechResourceDeniedUserMessage(rawMessage) != "" {
			if code, message, failed := providerBusinessFailure(header); failed {
				return code, message, true
			}
			return "resource_not_granted", rawMessage, true
		}
	}
	if code, message, failed := providerBusinessFailure(payload); failed {
		return code, message, true
	}
	// DashScope 业务失败在 output 内
	if output, ok := payload["output"].(map[string]any); ok {
		return providerBusinessFailure(output)
	}
	return "", "", false
}

func providerBusinessFailure(payload map[string]any) (string, string, bool) {
	if errorValue, ok := payload["error"].(map[string]any); ok {
		code, message := providerFailureDetails(map[string]any{"error": errorValue})
		if code != "" || message != "" {
			return code, message, true
		}
	}

	code := strings.ToLower(strings.TrimSpace(fmt.Sprint(payload["code"])))
	if code != "" && code != "0" &&
		code != "success" && code != "succeeded" &&
		code != "ok" && code != "<nil>" {
		code, message := providerFailureDetails(payload)
		return code, message, true
	}

	status := strings.ToLower(strings.TrimSpace(fmt.Sprint(payload["task_status"])))
	switch status {
	case "failed", "failure", "error", "expired":
		code, message := providerFailureDetails(payload)
		if code == "" {
			code = "task_failed"
		}
		return code, message, true
	}

	return "", "", false
}

func normalizedProviderErrorCode(value any) string {
	var code string
	switch current := value.(type) {
	case string:
		code = current
	case fmt.Stringer:
		code = current.String()
	case float64:
		if current != 0 {
			code = fmt.Sprintf("%g", current)
		}
	case int:
		if current != 0 {
			code = fmt.Sprintf("%d", current)
		}
	case int64:
		if current != 0 {
			code = fmt.Sprintf("%d", current)
		}
	}
	code = strings.TrimSpace(code)
	if code == "0" {
		return ""
	}
	return truncateRunes(code, 80)
}

func isContentModerationFailure(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{
		contentModerationErrorCode, "content_policy_violation", "content policy violation",
		"内容审核未通过", "未通过安全审核", "未通过上游安全审核", "未通过模型服务安全审核",
		"内容安全检查未通过", "防护限制", "请求触发了平台安全策略",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
