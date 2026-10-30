package app

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"yingce/backend/internal/protocol"
)

// Gemini/Vertex 用 URL action 选择流式；保留插件的鉴权、参数和缓存引用。
func geminiStreamingRequest(spec protocol.RequestSpec) (protocol.RequestSpec, error) {
	cloned, err := cloneProtocolRequestSpec(spec)
	if err != nil {
		return spec, err
	}
	path, err := url.Parse(cloned.Path)
	if err != nil {
		return spec, fmt.Errorf("Gemini 流式接口路径无效：%w", err)
	}
	switch {
	case strings.HasSuffix(path.Path, ":generateContent"):
		escaped := strings.TrimSuffix(path.EscapedPath(), ":generateContent") + ":streamGenerateContent"
		path.Path = strings.TrimSuffix(path.Path, ":generateContent") + ":streamGenerateContent"
		path.RawPath = escaped
	case strings.HasSuffix(path.Path, ":streamGenerateContent"):
	default:
		return spec, errors.New("Gemini 文本流式请求需要 generateContent 接口")
	}
	query := path.Query()
	query.Del("alt")
	path.RawQuery = query.Encode()
	cloned.Path = path.String()
	if cloned.Query == nil {
		cloned.Query = make(map[string][]string)
	}
	cloned.Query["alt"] = []string{"sse"}
	delete(protocolBodyObject(cloned.Body), "stream")
	return cloned, nil
}

func (p *streamingAgentParser) consumeGeminiEvent(payload map[string]any) {
	if usage, ok := payload["usageMetadata"].(map[string]any); ok {
		if p.geminiUsage == nil {
			p.geminiUsage = make(map[string]any)
		}
		// usageMetadata 是累计值，不把每个数据块的用量重复相加。
		for key, value := range usage {
			p.geminiUsage[key] = value
		}
	}
	if feedback, ok := payload["promptFeedback"].(map[string]any); ok {
		if reason := stringField(feedback, "blockReason"); reason != "" && reason != "BLOCK_REASON_UNSPECIFIED" {
			p.err = geminiFinishError(reason)
			return
		}
	}
	for fallbackIndex, value := range interfaceSlice(payload["candidates"]) {
		candidate, _ := value.(map[string]any)
		if intField(candidate, "index", fallbackIndex) != 0 {
			continue
		}
		reason := stringField(candidate, "finishReason")
		if reason == "FINISH_REASON_UNSPECIFIED" {
			reason = ""
		}
		if reason != "" && reason != "STOP" {
			p.err = geminiFinishError(reason)
			return
		}
		content, _ := candidate["content"].(map[string]any)
		for _, value := range interfaceSlice(content["parts"]) {
			part, _ := value.(map[string]any)
			if thought, _ := part["thought"].(bool); thought {
				p.appendReasoning(stringField(part, "text"))
			} else {
				p.appendText(stringField(part, "text"))
			}
		}
		if reason == "STOP" {
			p.geminiFinished = true
		}
	}
}

func geminiFinishError(reason string) error {
	switch reason {
	case "SAFETY", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII", "IMAGE_SAFETY", "IMAGE_PROHIBITED_CONTENT":
		return providerPayloadError{raw: "content_policy_violation: " + reason, message: "内容未通过上游安全审核，请修改提示词或参考素材后重试"}
	case "MAX_TOKENS":
		return errors.New("Gemini 输出达到模型 Token 上限，内容未完整生成")
	default:
		return fmt.Errorf("Gemini 未正常完成生成，结束原因：%s", reason)
	}
}

func (p *streamingAgentParser) completeGeminiStream(mimeType string) error {
	if !strings.Contains(strings.ToLower(mimeType), "event-stream") {
		return errors.New("Gemini 流式接口未返回 SSE，请检查中转服务是否支持 streamGenerateContent?alt=sse")
	}
	p.flush()
	_, err := p.result()
	return err
}

func (p *streamingAgentParser) geminiTextResult() (map[string]any, error) {
	if !p.geminiFinished {
		return nil, fmt.Errorf("Gemini 流式响应在生成完成前中断：%w", io.ErrUnexpectedEOF)
	}
	if p.text.Len() == 0 {
		return nil, errors.New("Gemini 流式文本接口没有返回内容")
	}
	result := map[string]any{"mode": "text", "text": p.text.String()}
	if p.reasoning.Len() > 0 {
		result["reasoning"] = p.reasoning.String()
	}
	return result, nil
}
