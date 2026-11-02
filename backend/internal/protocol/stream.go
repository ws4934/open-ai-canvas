package protocol

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type ResponseError struct {
	Code    string
	Message string
}

func (e *ResponseError) Error() string {
	if e.Code != "" {
		return e.Code + ": " + e.Message
	}
	return e.Message
}

func evaluateResponseError(template any, env map[string]any) error {
	if template == nil {
		return nil
	}
	value, err := evaluateManifestValue(template, env)
	if err != nil {
		return err
	}
	if manifestEmpty(value) {
		return nil
	}
	if object, ok := value.(map[string]any); ok {
		return &ResponseError{Code: manifestString(object["code"]), Message: manifestString(object["message"])}
	}
	return &ResponseError{Message: manifestString(value)}
}

func manifestJoinedText(value any) string {
	var out strings.Builder
	for _, part := range manifestArray(value) {
		if part != nil {
			out.WriteString(manifestString(part))
		}
	}
	return out.String()
}

// The host still owns HTTP framing, credentials, cancellation and usage records.
type StreamDecoder struct {
	rules           ManifestStream
	text, reasoning strings.Builder
	appended        map[string]any
	merged          map[string]any
	finished        bool
	err             error
}

func NewStreamDecoder(rules ManifestStream) *StreamDecoder {
	return &StreamDecoder{rules: rules, appended: map[string]any{}, merged: map[string]any{}}
}

func (d *StreamDecoder) Consume(event string, payload map[string]any) (string, string, error) {
	if d.err != nil {
		return "", "", d.err
	}
	env := map[string]any{"event": event, "response": payload}
	if d.err = evaluateResponseError(d.rules.Error, env); d.err != nil {
		return "", "", d.err
	}
	read := func(template any) string {
		if template == nil {
			return ""
		}
		value, err := evaluateManifestValue(template, env)
		if err != nil {
			d.err = err
			return ""
		}
		return manifestJoinedText(value)
	}
	text, reasoning := read(d.rules.Text), read(d.rules.Reasoning)
	d.text.WriteString(text)
	d.reasoning.WriteString(reasoning)
	for key, template := range d.rules.Append {
		value, err := evaluateManifestValue(template, env)
		if err != nil {
			d.err = err
			break
		}
		d.appended[key] = append(manifestArray(d.appended[key]), manifestArray(value)...)
	}
	for key, template := range d.rules.Merge {
		value, err := evaluateManifestValue(template, env)
		if err != nil {
			d.err = err
			break
		}
		result, _ := d.merged[key].(map[string]any)
		if result == nil {
			result = map[string]any{}
		}
		for name, field := range manifestObject(value) {
			result[name] = field
		}
		d.merged[key] = result
	}
	complete, err := evaluateManifestValue(d.rules.Complete, env)
	if err != nil {
		d.err = err
	}
	d.finished = d.finished || manifestTruthy(complete)
	return text, reasoning, d.err
}

func (d *StreamDecoder) Finish() ([]byte, error) {
	if d.err != nil {
		return nil, d.err
	}
	if !d.finished {
		return nil, fmt.Errorf("流式响应在生成完成前中断：%w", io.ErrUnexpectedEOF)
	}
	value, err := evaluateManifestValue(d.rules.Response, map[string]any{"stream": map[string]any{
		"text": d.text.String(), "reasoning": d.reasoning.String(), "append": d.appended, "merge": d.merged,
	}})
	if err != nil {
		return nil, err
	}
	return json.Marshal(value)
}
