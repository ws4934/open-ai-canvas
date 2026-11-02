package app

import (
	"context"
	"errors"
	"strings"
	"yingce/backend/internal/protocol"
)

func executeDeclarativeStream(ctx context.Context, input canvasGenerationInput, spec protocol.RequestSpec) ([]byte, error) {
	parser := newStreamingAgentParser("", input.OnTextDelta)
	parser.emitReasoning = input.OnReasoningDelta
	parser.declarative = protocol.NewStreamDecoder(spec.Stream.Rules)
	var final []byte
	complete := func(mime string) error {
		if !strings.Contains(strings.ToLower(mime), "event-stream") {
			if !spec.Stream.Rules.AllowFullResponse {
				return errors.New("流式接口未返回 SSE 响应")
			}
			return nil
		}
		parser.flush()
		if parser.err != nil {
			return protocolResponseError(parser.err)
		}
		var err error
		final, err = parser.declarative.Finish()
		if err == nil && input.AgentRequests == nil && strings.TrimSpace(parser.text.String()) == "" {
			return errors.New("流式文本接口没有返回内容")
		}
		return protocolResponseError(err)
	}
	data, mime, err := executeProtocolBinaryRequestWithConsumer(ctx, input.Config, spec.Stream.Request, parser.consume, complete)
	if err != nil {
		return nil, err
	}
	if !strings.Contains(strings.ToLower(mime), "event-stream") {
		return data, nil
	}
	return final, nil
}

func protocolResponseError(err error) error {
	var response *protocol.ResponseError
	if errors.As(err, &response) {
		return providerPayloadError{raw: response.Error(), message: response.Message}
	}
	return err
}
