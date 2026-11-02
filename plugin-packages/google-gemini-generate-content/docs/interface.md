# Google Gemini generateContent 接口字段

## 协议身份

- 插件 ID：`google-gemini-generate-content`。
- Provider ID：`gemini-generate-content`。
- 能力：`text`。
- 默认 Base URL：`https://generativelanguage.googleapis.com`。
- 鉴权驱动：`google-api-key`。
- 创建：`POST /v1beta/models/{{model}}:generateContent`。
- Agent：`POST /v1beta/models/{{model}}:generateContent`。
- 响应：完整响应 / 流式响应。

## 配置字段

| 字段 | 类型 | 必填 | 含义 |
| --- | --- | --- | --- |
| `apiKey` | secret | 是 | API Key |

## 统一字段映射

| 统一字段 | 类型 | 必填 | 上游映射 | 说明 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | `model` | 上游模型 ID。 |
| `messages` | message[] | 是 | `provider message container` | 包含历史消息和当前用户输入。 |
| `instructions` | string | 否 | `system/instructions` | 系统指令。 |
| `temperature` | number | 否 | `temperature` | 采样温度。 |
| `top_p` | number | 否 | `top_p` | 核采样参数。 |
| `max_tokens` | integer | 否 | `max_tokens/max_output_tokens` | 最大输出 token。 |
| `tools` | array | 否 | `tools/toolConfig` | 工具定义。 |
| `tool_choice` | object|string | 否 | `tool_choice` | 工具选择策略。 |
| `response_format` | object | 否 | `response_format/text` | 结构化输出配置。 |
| `stream` | boolean | 否 | `stream` | 流式开关；后台任务当前以最终响应归一。 |

## 上游请求模板逐字段清单

下表由插件请求模板生成，覆盖 body、query、headers 和 multipart 文件声明中的每个字段。

| 上游位置 | 值或转换表达式 |
| --- | --- |
| `create.method` | `"POST"` |
| `create.path` | `"/v1beta/models/{{model}}:generateContent"` |
| `create.contentType` | `"application/json"` |
| `create.body.contents` | `{"$map":{"from":{"$filter":{"from":{"$ref":"request.messages"},"as":"message","where":{"$ne":[{"$ref":"message.role"},"system"]}}},"as":"message","in":{"role":{"$if":{"condition":{"$eq":[{"$ref":"message.role"},"assistant"]},"then":"model","else":"user"}},"parts":[{"text":{"$ref":"message.content"}}]}}}` |
| `create.body.systemInstruction` | `{"$if":{"condition":{"$ref":"request.instructions"},"then":{"parts":[{"text":{"$ref":"request.instructions"}}]},"else":null}}` |
| `create.body.generationConfig` | `{"$omitEmpty":{"$ref":"request.providerOptions.gemini-generate-content.generationConfig"}}` |
| `create.body.safetySettings` | `{"$omitEmpty":{"$ref":"request.providerOptions.gemini-generate-content.safetySettings"}}` |
| `create.body.tools` | `{"$omitEmpty":{"$ref":"request.providerOptions.gemini-generate-content.tools"}}` |
| `create.body.toolConfig` | `{"$omitEmpty":{"$ref":"request.providerOptions.gemini-generate-content.toolConfig"}}` |
| `create.body.cachedContent` | `{"$omitEmpty":{"$ref":"request.providerOptions.gemini-generate-content.cachedContent"}}` |
| `create.stream.request.path` | `"/v1beta/models/{{model}}:streamGenerateContent"` |
| `create.stream.request.query.alt` | `"sse"` |
| `create.stream.text` | `{"$map":{"from":{"$filter":{"from":{"$ref":"response.candidates.0.content.parts"},"as":"part","where":{"$ne":[{"$ref":"part.thought"},true]}}},"as":"part","in":{"$ref":"part.text"}}}` |
| `create.stream.reasoning` | `{"$map":{"from":{"$filter":{"from":{"$ref":"response.candidates.0.content.parts"},"as":"part","where":{"$eq":[{"$ref":"part.thought"},true]}}},"as":"part","in":{"$ref":"part.text"}}}` |
| `create.stream.append.parts` | `{"$ref":"response.candidates.0.content.parts"}` |
| `create.stream.merge.usage` | `{"$ref":"response.usageMetadata"}` |
| `create.stream.complete` | `{"$eq":[{"$ref":"response.candidates.0.finishReason"},"STOP"]}` |
| `create.stream.error` | `{"$switch":{"cases":[{"when":{"$ref":"response.error"},"then":{"code":{"$coalesce":[{"$ref":"response.error.status"},{"$ref":"response.error.code"}]},"message":{"$ref":"response.error.message"}}},{"when":{"$in":[{"$coalesce":[{"$ref":"response.promptFeedback.blockReason"},{"$ref":"response.candidates.0.finishReason"}]},["SAFETY","BLOCKLIST","PROHIBITED_CONTENT","SPII","IMAGE_SAFETY","IMAGE_PROHIBITED_CONTENT"]]},"then":{"code":"content_policy_violation","message":"内容未通过上游安全审核，请修改提示词或参考素材后重试"}},{"when":{"$eq":[{"$coalesce":[{"$ref":"response.promptFeedback.blockReason"},{"$ref":"response.candidates.0.finishReason"}]},"MAX_TOKENS"]},"then":{"code":"output_token_limit","message":"Gemini 输出达到模型 Token 上限，内容未完整生成"}},{"when":{"$and":[{"$coalesce":[{"$ref":"response.promptFeedback.blockReason"},{"$ref":"response.candidates.0.finishReason"}]},{"$not":{"$in":[{"$coalesce":[{"$ref":"response.promptFeedback.blockReason"},{"$ref":"response.candidates.0.finishReason"}]},["STOP","FINISH_REASON_UNSPECIFIED","BLOCK_REASON_UNSPECIFIED"]]}}]},"then":{"code":{"$coalesce":[{"$ref":"response.promptFeedback.blockReason"},{"$ref":"response.candidates.0.finishReason"}]},"message":{"$concat":["Gemini 未正常完成生成，结束原因：",{"$coalesce":[{"$ref":"response.promptFeedback.blockReason"},{"$ref":"response.candidates.0.finishReason"}]}]}}}],"default":null}}` |
| `create.stream.response.candidates[0].content.role` | `"model"` |
| `create.stream.response.candidates[0].content.parts` | `{"$ref":"stream.append.parts"}` |
| `create.stream.response.candidates[0].finishReason` | `"STOP"` |
| `create.stream.response.usageMetadata` | `{"$ref":"stream.merge.usage"}` |
| `agent.method` | `"POST"` |
| `agent.path` | `"/v1beta/models/{{model}}:generateContent"` |
| `agent.contentType` | `"application/json"` |
| `agent.body.contents` | `{"$map":{"from":{"$ref":"request.extra.agent.conversation.messages"},"as":"message","in":{"role":{"$if":{"condition":{"$eq":[{"$ref":"message.role"},"assistant"]},"then":"model","else":"user"}},"parts":{"$map":{"from":{"$ref":"message.parts"},"as":"part","in":{"$switch":{"cases":[{"when":{"$eq":[{"$ref":"part.kind"},"text"]},"then":{"text":{"$ref":"part.text"}}},{"when":{"$eq":[{"$ref":"part.kind"},"media"]},"then":{"$if":{"condition":{"$ref":"part.inline"},"then":{"inlineData":{"mimeType":{"$ref":"part.mimeType"},"data":{"$ref":"part.data"}}},"else":{"fileData":{"mimeType":{"$ref":"part.mimeType"},"fileUri":{"$ref":"part.url"}}}}}},{"when":{"$eq":[{"$ref":"part.kind"},"tool_call"]},"then":{"functionCall":{"id":{"$ref":"part.id"},"name":{"$ref":"part.name"},"args":{"$ref":"part.arguments"}},"thoughtSignature":{"$omitEmpty":{"$ref":"part.signature"}}}},{"when":{"$eq":[{"$ref":"part.kind"},"tool_result"]},"then":{"functionResponse":{"id":{"$ref":"part.id"},"name":{"$ref":"part.name"},"response":{"result":{"$ref":"part.result"}}}}}],"default":null}}}}}}}` |
| `agent.body.systemInstruction` | `{"$if":{"condition":{"$ref":"request.extra.agent.conversation.instructions"},"then":{"parts":[{"text":{"$ref":"request.extra.agent.conversation.instructions"}}]},"else":null}}` |
| `agent.body.tools` | `{"$if":{"condition":{"$ref":"request.extra.agent.conversation.tools"},"then":[{"functionDeclarations":{"$map":{"from":{"$ref":"request.extra.agent.conversation.tools"},"as":"tool","in":{"name":{"$ref":"tool.name"},"description":{"$omitEmpty":{"$ref":"tool.description"}},"parameters":{"$schemaSubset":{"from":{"$ref":"tool.parameters"},"constToEnum":true,"stringEnumOnly":true,"inferStringEnumType":true,"enumHintPrefix":" 可选值：","omitKeywords":["exclusiveMinimum","exclusiveMaximum","uniqueItems"]}}}}}}],"else":null}}` |
| `agent.body.toolConfig` | `{"$if":{"condition":{"$ref":"request.extra.agent.conversation.tools"},"then":{"functionCallingConfig":{"$switch":{"cases":[{"when":{"$ref":"request.extra.agent.conversation.toolChoice.name"},"then":{"mode":"ANY","allowedFunctionNames":[{"$ref":"request.extra.agent.conversation.toolChoice.name"}]}},{"when":{"$eq":[{"$ref":"request.extra.agent.conversation.toolChoice"},"required"]},"then":{"mode":"ANY"}}],"default":{"mode":"AUTO"}}}},"else":null}}` |
| `agent.body.generationConfig.maxOutputTokens` | `{"$omitEmpty":{"$ref":"request.extra.agent.conversation.options.maxOutputTokens"}}` |
| `agent.body.generationConfig.thinkingConfig` | `{"$if":{"condition":{"$ref":"request.extra.agent.conversation.options.thinking"},"then":{"includeThoughts":true},"else":null}}` |
| `agent.stream.request.path` | `"/v1beta/models/{{model}}:streamGenerateContent"` |
| `agent.stream.request.query.alt` | `"sse"` |
| `agent.stream.text` | `{"$map":{"from":{"$filter":{"from":{"$ref":"response.candidates.0.content.parts"},"as":"part","where":{"$ne":[{"$ref":"part.thought"},true]}}},"as":"part","in":{"$ref":"part.text"}}}` |
| `agent.stream.reasoning` | `{"$map":{"from":{"$filter":{"from":{"$ref":"response.candidates.0.content.parts"},"as":"part","where":{"$eq":[{"$ref":"part.thought"},true]}}},"as":"part","in":{"$ref":"part.text"}}}` |
| `agent.stream.append.parts` | `{"$ref":"response.candidates.0.content.parts"}` |
| `agent.stream.merge.usage` | `{"$ref":"response.usageMetadata"}` |
| `agent.stream.complete` | `{"$eq":[{"$ref":"response.candidates.0.finishReason"},"STOP"]}` |
| `agent.stream.error` | `{"$switch":{"cases":[{"when":{"$ref":"response.error"},"then":{"code":{"$coalesce":[{"$ref":"response.error.status"},{"$ref":"response.error.code"}]},"message":{"$ref":"response.error.message"}}},{"when":{"$in":[{"$coalesce":[{"$ref":"response.promptFeedback.blockReason"},{"$ref":"response.candidates.0.finishReason"}]},["SAFETY","BLOCKLIST","PROHIBITED_CONTENT","SPII","IMAGE_SAFETY","IMAGE_PROHIBITED_CONTENT"]]},"then":{"code":"content_policy_violation","message":"内容未通过上游安全审核，请修改提示词或参考素材后重试"}},{"when":{"$eq":[{"$coalesce":[{"$ref":"response.promptFeedback.blockReason"},{"$ref":"response.candidates.0.finishReason"}]},"MAX_TOKENS"]},"then":{"code":"output_token_limit","message":"Gemini 输出达到模型 Token 上限，内容未完整生成"}},{"when":{"$and":[{"$coalesce":[{"$ref":"response.promptFeedback.blockReason"},{"$ref":"response.candidates.0.finishReason"}]},{"$not":{"$in":[{"$coalesce":[{"$ref":"response.promptFeedback.blockReason"},{"$ref":"response.candidates.0.finishReason"}]},["STOP","FINISH_REASON_UNSPECIFIED","BLOCK_REASON_UNSPECIFIED"]]}}]},"then":{"code":{"$coalesce":[{"$ref":"response.promptFeedback.blockReason"},{"$ref":"response.candidates.0.finishReason"}]},"message":{"$concat":["Gemini 未正常完成生成，结束原因：",{"$coalesce":[{"$ref":"response.promptFeedback.blockReason"},{"$ref":"response.candidates.0.finishReason"}]}]}}}],"default":null}}` |
| `agent.stream.response.candidates[0].content.role` | `"model"` |
| `agent.stream.response.candidates[0].content.parts` | `{"$ref":"stream.append.parts"}` |
| `agent.stream.response.candidates[0].finishReason` | `"STOP"` |
| `agent.stream.response.usageMetadata` | `{"$ref":"stream.merge.usage"}` |
| `agent.stream.allowFullResponse` | `true` |
| `agent.promptCache.fields[0]` | `"systemInstruction"` |
| `agent.promptCache.fields[1]` | `"tools"` |
| `agent.promptCache.fields[2]` | `"toolConfig"` |
| `agent.promptCache.model` | `{"$trimPrefix":[{"$ref":"request.model"},"models/"]}` |
| `agent.promptCache.minimumTokens` | `{"$if":{"condition":{"$contains":[{"$ref":"request.model"},"gemini-2.5"]},"then":2048,"else":4096}}` |
| `agent.promptCache.create.method` | `"POST"` |
| `agent.promptCache.create.path` | `"/v1beta/cachedContents"` |
| `agent.promptCache.create.contentType` | `"application/json"` |
| `agent.promptCache.create.body` | `{"$merge":[{"model":{"$concat":["models/",{"$ref":"cache.model"}]},"ttl":"3600s"},{"$ref":"cache.prefix"}]}` |
| `agent.promptCache.delete.method` | `"DELETE"` |
| `agent.promptCache.delete.pathTemplate` | `{"$concat":["/v1beta/",{"$ref":"cache.resourceName"}]}` |
| `agent.promptCache.delete.contentType` | `"application/json"` |
| `agent.promptCache.resource` | `{"$ref":"response.name"}` |
| `agent.promptCache.expireTime` | `{"$ref":"response.expireTime"}` |
| `agent.promptCache.resourcePattern` | `"^cachedContents/[A-Za-z0-9_-][A-Za-z0-9_.-]*$"` |
| `agent.promptCache.applyField` | `"cachedContent"` |
| `agent.promptCache.missing` | `{"$and":[{"$eq":[{"$ref":"error.status"},404]},{"$contains":[{"$ref":"error.body"},{"$ref":"error.resourceName"}]},{"$or":[{"$not":{"$ref":"response.error.code"}},{"$eq":[{"$ref":"response.error.code"},404]}]},{"$or":[{"$not":{"$ref":"response.error.status"}},{"$eq":[{"$ref":"response.error.status"},"NOT_FOUND"]}]}]}` |

## Provider 扩展键

- `providerOptions.gemini-generate-content.cachedContent`
- `providerOptions.gemini-generate-content.generationConfig`
- `providerOptions.gemini-generate-content.safetySettings`
- `providerOptions.gemini-generate-content.toolConfig`
- `providerOptions.gemini-generate-content.tools`

动态模型或工作流允许使用文档声明的完整 `parameters/input/extra_body` 对象；该对象是协议本身的开放 schema，不会被宿主裁剪。

## 响应映射逐字段清单

| 映射位置 | 上游路径或转换表达式 |
| --- | --- |
| `response.status` | `"succeeded"` |
| `response.text` | `{"$map":{"from":{"$filter":{"from":{"$ref":"response.candidates.0.content.parts"},"as":"part","where":{"$ne":[{"$ref":"part.thought"},true]}}},"as":"part","in":{"$ref":"part.text"}}}` |
| `response.reasoning` | `{"$map":{"from":{"$filter":{"from":{"$ref":"response.candidates.0.content.parts"},"as":"part","where":{"$eq":[{"$ref":"part.thought"},true]}}},"as":"part","in":{"$ref":"part.text"}}}` |
| `response.error` | `{"$switch":{"cases":[{"when":{"$ref":"response.error"},"then":{"code":{"$coalesce":[{"$ref":"response.error.status"},{"$ref":"response.error.code"}]},"message":{"$ref":"response.error.message"}}},{"when":{"$in":[{"$coalesce":[{"$ref":"response.promptFeedback.blockReason"},{"$ref":"response.candidates.0.finishReason"}]},["SAFETY","BLOCKLIST","PROHIBITED_CONTENT","SPII","IMAGE_SAFETY","IMAGE_PROHIBITED_CONTENT"]]},"then":{"code":"content_policy_violation","message":"内容未通过上游安全审核，请修改提示词或参考素材后重试"}},{"when":{"$eq":[{"$coalesce":[{"$ref":"response.promptFeedback.blockReason"},{"$ref":"response.candidates.0.finishReason"}]},"MAX_TOKENS"]},"then":{"code":"output_token_limit","message":"Gemini 输出达到模型 Token 上限，内容未完整生成"}},{"when":{"$and":[{"$coalesce":[{"$ref":"response.promptFeedback.blockReason"},{"$ref":"response.candidates.0.finishReason"}]},{"$not":{"$in":[{"$coalesce":[{"$ref":"response.promptFeedback.blockReason"},{"$ref":"response.candidates.0.finishReason"}]},["STOP","FINISH_REASON_UNSPECIFIED","BLOCK_REASON_UNSPECIFIED"]]}}]},"then":{"code":{"$coalesce":[{"$ref":"response.promptFeedback.blockReason"},{"$ref":"response.candidates.0.finishReason"}]},"message":{"$concat":["Gemini 未正常完成生成，结束原因：",{"$coalesce":[{"$ref":"response.promptFeedback.blockReason"},{"$ref":"response.candidates.0.finishReason"}]}]}}}],"default":null}}` |
| `response.usage` | `{"$ref":"response.usageMetadata"}` |
| `response.errorPaths[0]` | `"error.code"` |
| `response.messagePaths[0]` | `"error.message"` |
| `agentResponse.text` | `{"$map":{"from":{"$filter":{"from":{"$ref":"response.candidates.0.content.parts"},"as":"part","where":{"$ne":[{"$ref":"part.thought"},true]}}},"as":"part","in":{"$ref":"part.text"}}}` |
| `agentResponse.reasoning` | `{"$map":{"from":{"$filter":{"from":{"$ref":"response.candidates.0.content.parts"},"as":"part","where":{"$eq":[{"$ref":"part.thought"},true]}}},"as":"part","in":{"$ref":"part.text"}}}` |
| `agentResponse.error` | `{"$switch":{"cases":[{"when":{"$ref":"response.error"},"then":{"code":{"$coalesce":[{"$ref":"response.error.status"},{"$ref":"response.error.code"}]},"message":{"$ref":"response.error.message"}}},{"when":{"$in":[{"$coalesce":[{"$ref":"response.promptFeedback.blockReason"},{"$ref":"response.candidates.0.finishReason"}]},["SAFETY","BLOCKLIST","PROHIBITED_CONTENT","SPII","IMAGE_SAFETY","IMAGE_PROHIBITED_CONTENT"]]},"then":{"code":"content_policy_violation","message":"内容未通过上游安全审核，请修改提示词或参考素材后重试"}},{"when":{"$eq":[{"$coalesce":[{"$ref":"response.promptFeedback.blockReason"},{"$ref":"response.candidates.0.finishReason"}]},"MAX_TOKENS"]},"then":{"code":"output_token_limit","message":"Gemini 输出达到模型 Token 上限，内容未完整生成"}},{"when":{"$and":[{"$coalesce":[{"$ref":"response.promptFeedback.blockReason"},{"$ref":"response.candidates.0.finishReason"}]},{"$not":{"$in":[{"$coalesce":[{"$ref":"response.promptFeedback.blockReason"},{"$ref":"response.candidates.0.finishReason"}]},["STOP","FINISH_REASON_UNSPECIFIED","BLOCK_REASON_UNSPECIFIED"]]}}]},"then":{"code":{"$coalesce":[{"$ref":"response.promptFeedback.blockReason"},{"$ref":"response.candidates.0.finishReason"}]},"message":{"$concat":["Gemini 未正常完成生成，结束原因：",{"$coalesce":[{"$ref":"response.promptFeedback.blockReason"},{"$ref":"response.candidates.0.finishReason"}]}]}}}],"default":null}}` |
| `agentResponse.toolCallsPath` | `"candidates.0.content.parts"` |
| `agentResponse.toolCallIdPaths[0]` | `"functionCall.id"` |
| `agentResponse.toolCallNamePaths[0]` | `"functionCall.name"` |
| `agentResponse.toolCallArgumentsPaths[0]` | `"functionCall.args"` |
| `agentResponse.toolCallThoughtSignaturePaths[0]` | `"thoughtSignature"` |
| `agentResponse.toolCallThoughtSignaturePaths[1]` | `"thought_signature"` |

## 响应与错误

插件把上游 task/status/text/media/usage 映射为统一结果。临时媒体 URL 标记为 ephemeral，由宿主立即下载持久化。HTTP 错误、业务 code 和 error object 保持失败语义，不包装成成功。

## 兼容边界

v3 合同：插件负责 conversation 到 Gemini 回合、工具 JSON Schema 子集、签名、正文/思考、SSE 终态与 Prompt Cache 接口。宿主只执行通用映射、HTTP、会话、审批、幂等和缓存隔离。Agent SSE 可接受完整 JSON；普通文本 SSE 不接受非流式响应。缓存创建失败按无缓存请求继续，不修改 Vertex IAM、已有模型能力或价格。

<!-- YINGCE_MANIFEST_CONTRACT_START -->
## Manifest 完整接口定义

以下 JSON 与插件包内实际 `manifest.json` 逐字段一致，覆盖插件身份、权限、配置、鉴权、参数、校验、创建、Agent、查询、取消、结果下载、响应和 Agent 响应映射。`documentation` 字段的值就是当前完整文档；为避免文档在自身内部无限递归，JSON 中仅用等义占位文本表示正文。

```json
{
  "apiVersion": "yingce.plugin/v3",
  "id": "google-gemini-generate-content",
  "name": "Google Gemini generateContent",
  "version": "2.1.0",
  "author": "Google / 影策",
  "description": "Google Gemini generateContent 独立请求协议插件。",
  "documentation": "<当前插件的完整 documentation，由 README.md 与 docs/interface.md 拼接而成；为避免 JSON 递归，此处不重复展开正文。>",
  "permissions": [
    "generation.run",
    "media.read"
  ],
  "configuration": {
    "fields": [
      {
        "name": "apiKey",
        "type": "secret",
        "label": "API Key",
        "required": true
      }
    ]
  },
  "contributes": {
    "providers": [
      {
        "id": "gemini-generate-content",
        "label": "Google Gemini generateContent",
        "capabilities": [
          "text"
        ],
        "scopes": [
          "admin.system-channel",
          "user.custom-channel",
          "canvas",
          "creation",
          "agent"
        ],
        "baseUrl": "https://generativelanguage.googleapis.com",
        "requiresPublicMediaUrls": false,
        "auth": {
          "type": "google-api-key",
          "field": "apiKey"
        },
        "parameters": [
          {
            "name": "model",
            "type": "string",
            "required": true,
            "mapping": "model",
            "description": "上游模型 ID。"
          },
          {
            "name": "messages",
            "type": "message[]",
            "required": true,
            "mapping": "provider message container",
            "description": "包含历史消息和当前用户输入。"
          },
          {
            "name": "instructions",
            "type": "string",
            "required": false,
            "mapping": "system/instructions",
            "description": "系统指令。"
          },
          {
            "name": "temperature",
            "type": "number",
            "required": false,
            "mapping": "temperature",
            "description": "采样温度。"
          },
          {
            "name": "top_p",
            "type": "number",
            "required": false,
            "mapping": "top_p",
            "description": "核采样参数。"
          },
          {
            "name": "max_tokens",
            "type": "integer",
            "required": false,
            "mapping": "max_tokens/max_output_tokens",
            "description": "最大输出 token。"
          },
          {
            "name": "tools",
            "type": "array",
            "required": false,
            "mapping": "tools/toolConfig",
            "description": "工具定义。"
          },
          {
            "name": "tool_choice",
            "type": "object|string",
            "required": false,
            "mapping": "tool_choice",
            "description": "工具选择策略。"
          },
          {
            "name": "response_format",
            "type": "object",
            "required": false,
            "mapping": "response_format/text",
            "description": "结构化输出配置。"
          },
          {
            "name": "stream",
            "type": "boolean",
            "required": false,
            "mapping": "stream",
            "description": "流式开关；后台任务当前以最终响应归一。"
          }
        ],
        "create": {
          "method": "POST",
          "path": "/v1beta/models/{{model}}:generateContent",
          "contentType": "application/json",
          "stream": {
            "request": {
              "path": "/v1beta/models/{{model}}:streamGenerateContent",
              "query": {
                "alt": "sse"
              }
            },
            "text": {
              "$map": {
                "from": {
                  "$filter": {
                    "from": {
                      "$ref": "response.candidates.0.content.parts"
                    },
                    "as": "part",
                    "where": {
                      "$ne": [
                        {
                          "$ref": "part.thought"
                        },
                        true
                      ]
                    }
                  }
                },
                "as": "part",
                "in": {
                  "$ref": "part.text"
                }
              }
            },
            "reasoning": {
              "$map": {
                "from": {
                  "$filter": {
                    "from": {
                      "$ref": "response.candidates.0.content.parts"
                    },
                    "as": "part",
                    "where": {
                      "$eq": [
                        {
                          "$ref": "part.thought"
                        },
                        true
                      ]
                    }
                  }
                },
                "as": "part",
                "in": {
                  "$ref": "part.text"
                }
              }
            },
            "append": {
              "parts": {
                "$ref": "response.candidates.0.content.parts"
              }
            },
            "merge": {
              "usage": {
                "$ref": "response.usageMetadata"
              }
            },
            "complete": {
              "$eq": [
                {
                  "$ref": "response.candidates.0.finishReason"
                },
                "STOP"
              ]
            },
            "error": {
              "$switch": {
                "cases": [
                  {
                    "when": {
                      "$ref": "response.error"
                    },
                    "then": {
                      "code": {
                        "$coalesce": [
                          {
                            "$ref": "response.error.status"
                          },
                          {
                            "$ref": "response.error.code"
                          }
                        ]
                      },
                      "message": {
                        "$ref": "response.error.message"
                      }
                    }
                  },
                  {
                    "when": {
                      "$in": [
                        {
                          "$coalesce": [
                            {
                              "$ref": "response.promptFeedback.blockReason"
                            },
                            {
                              "$ref": "response.candidates.0.finishReason"
                            }
                          ]
                        },
                        [
                          "SAFETY",
                          "BLOCKLIST",
                          "PROHIBITED_CONTENT",
                          "SPII",
                          "IMAGE_SAFETY",
                          "IMAGE_PROHIBITED_CONTENT"
                        ]
                      ]
                    },
                    "then": {
                      "code": "content_policy_violation",
                      "message": "内容未通过上游安全审核，请修改提示词或参考素材后重试"
                    }
                  },
                  {
                    "when": {
                      "$eq": [
                        {
                          "$coalesce": [
                            {
                              "$ref": "response.promptFeedback.blockReason"
                            },
                            {
                              "$ref": "response.candidates.0.finishReason"
                            }
                          ]
                        },
                        "MAX_TOKENS"
                      ]
                    },
                    "then": {
                      "code": "output_token_limit",
                      "message": "Gemini 输出达到模型 Token 上限，内容未完整生成"
                    }
                  },
                  {
                    "when": {
                      "$and": [
                        {
                          "$coalesce": [
                            {
                              "$ref": "response.promptFeedback.blockReason"
                            },
                            {
                              "$ref": "response.candidates.0.finishReason"
                            }
                          ]
                        },
                        {
                          "$not": {
                            "$in": [
                              {
                                "$coalesce": [
                                  {
                                    "$ref": "response.promptFeedback.blockReason"
                                  },
                                  {
                                    "$ref": "response.candidates.0.finishReason"
                                  }
                                ]
                              },
                              [
                                "STOP",
                                "FINISH_REASON_UNSPECIFIED",
                                "BLOCK_REASON_UNSPECIFIED"
                              ]
                            ]
                          }
                        }
                      ]
                    },
                    "then": {
                      "code": {
                        "$coalesce": [
                          {
                            "$ref": "response.promptFeedback.blockReason"
                          },
                          {
                            "$ref": "response.candidates.0.finishReason"
                          }
                        ]
                      },
                      "message": {
                        "$concat": [
                          "Gemini 未正常完成生成，结束原因：",
                          {
                            "$coalesce": [
                              {
                                "$ref": "response.promptFeedback.blockReason"
                              },
                              {
                                "$ref": "response.candidates.0.finishReason"
                              }
                            ]
                          }
                        ]
                      }
                    }
                  }
                ],
                "default": null
              }
            },
            "response": {
              "candidates": [
                {
                  "content": {
                    "role": "model",
                    "parts": {
                      "$ref": "stream.append.parts"
                    }
                  },
                  "finishReason": "STOP"
                }
              ],
              "usageMetadata": {
                "$ref": "stream.merge.usage"
              }
            }
          },
          "body": {
            "contents": {
              "$map": {
                "from": {
                  "$filter": {
                    "from": {
                      "$ref": "request.messages"
                    },
                    "as": "message",
                    "where": {
                      "$ne": [
                        {
                          "$ref": "message.role"
                        },
                        "system"
                      ]
                    }
                  }
                },
                "as": "message",
                "in": {
                  "role": {
                    "$if": {
                      "condition": {
                        "$eq": [
                          {
                            "$ref": "message.role"
                          },
                          "assistant"
                        ]
                      },
                      "then": "model",
                      "else": "user"
                    }
                  },
                  "parts": [
                    {
                      "text": {
                        "$ref": "message.content"
                      }
                    }
                  ]
                }
              }
            },
            "systemInstruction": {
              "$if": {
                "condition": {
                  "$ref": "request.instructions"
                },
                "then": {
                  "parts": [
                    {
                      "text": {
                        "$ref": "request.instructions"
                      }
                    }
                  ]
                },
                "else": null
              }
            },
            "generationConfig": {
              "$omitEmpty": {
                "$ref": "request.providerOptions.gemini-generate-content.generationConfig"
              }
            },
            "safetySettings": {
              "$omitEmpty": {
                "$ref": "request.providerOptions.gemini-generate-content.safetySettings"
              }
            },
            "tools": {
              "$omitEmpty": {
                "$ref": "request.providerOptions.gemini-generate-content.tools"
              }
            },
            "toolConfig": {
              "$omitEmpty": {
                "$ref": "request.providerOptions.gemini-generate-content.toolConfig"
              }
            },
            "cachedContent": {
              "$omitEmpty": {
                "$ref": "request.providerOptions.gemini-generate-content.cachedContent"
              }
            }
          }
        },
        "agent": {
          "method": "POST",
          "path": "/v1beta/models/{{model}}:generateContent",
          "contentType": "application/json",
          "input": "conversation",
          "body": {
            "contents": {
              "$map": {
                "from": {
                  "$ref": "request.extra.agent.conversation.messages"
                },
                "as": "message",
                "in": {
                  "role": {
                    "$if": {
                      "condition": {
                        "$eq": [
                          {
                            "$ref": "message.role"
                          },
                          "assistant"
                        ]
                      },
                      "then": "model",
                      "else": "user"
                    }
                  },
                  "parts": {
                    "$map": {
                      "from": {
                        "$ref": "message.parts"
                      },
                      "as": "part",
                      "in": {
                        "$switch": {
                          "cases": [
                            {
                              "when": {
                                "$eq": [
                                  {
                                    "$ref": "part.kind"
                                  },
                                  "text"
                                ]
                              },
                              "then": {
                                "text": {
                                  "$ref": "part.text"
                                }
                              }
                            },
                            {
                              "when": {
                                "$eq": [
                                  {
                                    "$ref": "part.kind"
                                  },
                                  "media"
                                ]
                              },
                              "then": {
                                "$if": {
                                  "condition": {
                                    "$ref": "part.inline"
                                  },
                                  "then": {
                                    "inlineData": {
                                      "mimeType": {
                                        "$ref": "part.mimeType"
                                      },
                                      "data": {
                                        "$ref": "part.data"
                                      }
                                    }
                                  },
                                  "else": {
                                    "fileData": {
                                      "mimeType": {
                                        "$ref": "part.mimeType"
                                      },
                                      "fileUri": {
                                        "$ref": "part.url"
                                      }
                                    }
                                  }
                                }
                              }
                            },
                            {
                              "when": {
                                "$eq": [
                                  {
                                    "$ref": "part.kind"
                                  },
                                  "tool_call"
                                ]
                              },
                              "then": {
                                "functionCall": {
                                  "id": {
                                    "$ref": "part.id"
                                  },
                                  "name": {
                                    "$ref": "part.name"
                                  },
                                  "args": {
                                    "$ref": "part.arguments"
                                  }
                                },
                                "thoughtSignature": {
                                  "$omitEmpty": {
                                    "$ref": "part.signature"
                                  }
                                }
                              }
                            },
                            {
                              "when": {
                                "$eq": [
                                  {
                                    "$ref": "part.kind"
                                  },
                                  "tool_result"
                                ]
                              },
                              "then": {
                                "functionResponse": {
                                  "id": {
                                    "$ref": "part.id"
                                  },
                                  "name": {
                                    "$ref": "part.name"
                                  },
                                  "response": {
                                    "result": {
                                      "$ref": "part.result"
                                    }
                                  }
                                }
                              }
                            }
                          ],
                          "default": null
                        }
                      }
                    }
                  }
                }
              }
            },
            "systemInstruction": {
              "$if": {
                "condition": {
                  "$ref": "request.extra.agent.conversation.instructions"
                },
                "then": {
                  "parts": [
                    {
                      "text": {
                        "$ref": "request.extra.agent.conversation.instructions"
                      }
                    }
                  ]
                },
                "else": null
              }
            },
            "tools": {
              "$if": {
                "condition": {
                  "$ref": "request.extra.agent.conversation.tools"
                },
                "then": [
                  {
                    "functionDeclarations": {
                      "$map": {
                        "from": {
                          "$ref": "request.extra.agent.conversation.tools"
                        },
                        "as": "tool",
                        "in": {
                          "name": {
                            "$ref": "tool.name"
                          },
                          "description": {
                            "$omitEmpty": {
                              "$ref": "tool.description"
                            }
                          },
                          "parameters": {
                            "$schemaSubset": {
                              "from": {
                                "$ref": "tool.parameters"
                              },
                              "constToEnum": true,
                              "stringEnumOnly": true,
                              "inferStringEnumType": true,
                              "enumHintPrefix": " 可选值：",
                              "omitKeywords": [
                                "exclusiveMinimum",
                                "exclusiveMaximum",
                                "uniqueItems"
                              ]
                            }
                          }
                        }
                      }
                    }
                  }
                ],
                "else": null
              }
            },
            "toolConfig": {
              "$if": {
                "condition": {
                  "$ref": "request.extra.agent.conversation.tools"
                },
                "then": {
                  "functionCallingConfig": {
                    "$switch": {
                      "cases": [
                        {
                          "when": {
                            "$ref": "request.extra.agent.conversation.toolChoice.name"
                          },
                          "then": {
                            "mode": "ANY",
                            "allowedFunctionNames": [
                              {
                                "$ref": "request.extra.agent.conversation.toolChoice.name"
                              }
                            ]
                          }
                        },
                        {
                          "when": {
                            "$eq": [
                              {
                                "$ref": "request.extra.agent.conversation.toolChoice"
                              },
                              "required"
                            ]
                          },
                          "then": {
                            "mode": "ANY"
                          }
                        }
                      ],
                      "default": {
                        "mode": "AUTO"
                      }
                    }
                  }
                },
                "else": null
              }
            },
            "generationConfig": {
              "maxOutputTokens": {
                "$omitEmpty": {
                  "$ref": "request.extra.agent.conversation.options.maxOutputTokens"
                }
              },
              "thinkingConfig": {
                "$if": {
                  "condition": {
                    "$ref": "request.extra.agent.conversation.options.thinking"
                  },
                  "then": {
                    "includeThoughts": true
                  },
                  "else": null
                }
              }
            }
          },
          "stream": {
            "request": {
              "path": "/v1beta/models/{{model}}:streamGenerateContent",
              "query": {
                "alt": "sse"
              }
            },
            "text": {
              "$map": {
                "from": {
                  "$filter": {
                    "from": {
                      "$ref": "response.candidates.0.content.parts"
                    },
                    "as": "part",
                    "where": {
                      "$ne": [
                        {
                          "$ref": "part.thought"
                        },
                        true
                      ]
                    }
                  }
                },
                "as": "part",
                "in": {
                  "$ref": "part.text"
                }
              }
            },
            "reasoning": {
              "$map": {
                "from": {
                  "$filter": {
                    "from": {
                      "$ref": "response.candidates.0.content.parts"
                    },
                    "as": "part",
                    "where": {
                      "$eq": [
                        {
                          "$ref": "part.thought"
                        },
                        true
                      ]
                    }
                  }
                },
                "as": "part",
                "in": {
                  "$ref": "part.text"
                }
              }
            },
            "append": {
              "parts": {
                "$ref": "response.candidates.0.content.parts"
              }
            },
            "merge": {
              "usage": {
                "$ref": "response.usageMetadata"
              }
            },
            "complete": {
              "$eq": [
                {
                  "$ref": "response.candidates.0.finishReason"
                },
                "STOP"
              ]
            },
            "error": {
              "$switch": {
                "cases": [
                  {
                    "when": {
                      "$ref": "response.error"
                    },
                    "then": {
                      "code": {
                        "$coalesce": [
                          {
                            "$ref": "response.error.status"
                          },
                          {
                            "$ref": "response.error.code"
                          }
                        ]
                      },
                      "message": {
                        "$ref": "response.error.message"
                      }
                    }
                  },
                  {
                    "when": {
                      "$in": [
                        {
                          "$coalesce": [
                            {
                              "$ref": "response.promptFeedback.blockReason"
                            },
                            {
                              "$ref": "response.candidates.0.finishReason"
                            }
                          ]
                        },
                        [
                          "SAFETY",
                          "BLOCKLIST",
                          "PROHIBITED_CONTENT",
                          "SPII",
                          "IMAGE_SAFETY",
                          "IMAGE_PROHIBITED_CONTENT"
                        ]
                      ]
                    },
                    "then": {
                      "code": "content_policy_violation",
                      "message": "内容未通过上游安全审核，请修改提示词或参考素材后重试"
                    }
                  },
                  {
                    "when": {
                      "$eq": [
                        {
                          "$coalesce": [
                            {
                              "$ref": "response.promptFeedback.blockReason"
                            },
                            {
                              "$ref": "response.candidates.0.finishReason"
                            }
                          ]
                        },
                        "MAX_TOKENS"
                      ]
                    },
                    "then": {
                      "code": "output_token_limit",
                      "message": "Gemini 输出达到模型 Token 上限，内容未完整生成"
                    }
                  },
                  {
                    "when": {
                      "$and": [
                        {
                          "$coalesce": [
                            {
                              "$ref": "response.promptFeedback.blockReason"
                            },
                            {
                              "$ref": "response.candidates.0.finishReason"
                            }
                          ]
                        },
                        {
                          "$not": {
                            "$in": [
                              {
                                "$coalesce": [
                                  {
                                    "$ref": "response.promptFeedback.blockReason"
                                  },
                                  {
                                    "$ref": "response.candidates.0.finishReason"
                                  }
                                ]
                              },
                              [
                                "STOP",
                                "FINISH_REASON_UNSPECIFIED",
                                "BLOCK_REASON_UNSPECIFIED"
                              ]
                            ]
                          }
                        }
                      ]
                    },
                    "then": {
                      "code": {
                        "$coalesce": [
                          {
                            "$ref": "response.promptFeedback.blockReason"
                          },
                          {
                            "$ref": "response.candidates.0.finishReason"
                          }
                        ]
                      },
                      "message": {
                        "$concat": [
                          "Gemini 未正常完成生成，结束原因：",
                          {
                            "$coalesce": [
                              {
                                "$ref": "response.promptFeedback.blockReason"
                              },
                              {
                                "$ref": "response.candidates.0.finishReason"
                              }
                            ]
                          }
                        ]
                      }
                    }
                  }
                ],
                "default": null
              }
            },
            "response": {
              "candidates": [
                {
                  "content": {
                    "role": "model",
                    "parts": {
                      "$ref": "stream.append.parts"
                    }
                  },
                  "finishReason": "STOP"
                }
              ],
              "usageMetadata": {
                "$ref": "stream.merge.usage"
              }
            },
            "allowFullResponse": true
          },
          "promptCache": {
            "fields": [
              "systemInstruction",
              "tools",
              "toolConfig"
            ],
            "model": {
              "$trimPrefix": [
                {
                  "$ref": "request.model"
                },
                "models/"
              ]
            },
            "minimumTokens": {
              "$if": {
                "condition": {
                  "$contains": [
                    {
                      "$ref": "request.model"
                    },
                    "gemini-2.5"
                  ]
                },
                "then": 2048,
                "else": 4096
              }
            },
            "create": {
              "method": "POST",
              "path": "/v1beta/cachedContents",
              "contentType": "application/json",
              "body": {
                "$merge": [
                  {
                    "model": {
                      "$concat": [
                        "models/",
                        {
                          "$ref": "cache.model"
                        }
                      ]
                    },
                    "ttl": "3600s"
                  },
                  {
                    "$ref": "cache.prefix"
                  }
                ]
              }
            },
            "delete": {
              "method": "DELETE",
              "pathTemplate": {
                "$concat": [
                  "/v1beta/",
                  {
                    "$ref": "cache.resourceName"
                  }
                ]
              },
              "contentType": "application/json"
            },
            "resource": {
              "$ref": "response.name"
            },
            "expireTime": {
              "$ref": "response.expireTime"
            },
            "resourcePattern": "^cachedContents/[A-Za-z0-9_-][A-Za-z0-9_.-]*$",
            "applyField": "cachedContent",
            "missing": {
              "$and": [
                {
                  "$eq": [
                    {
                      "$ref": "error.status"
                    },
                    404
                  ]
                },
                {
                  "$contains": [
                    {
                      "$ref": "error.body"
                    },
                    {
                      "$ref": "error.resourceName"
                    }
                  ]
                },
                {
                  "$or": [
                    {
                      "$not": {
                        "$ref": "response.error.code"
                      }
                    },
                    {
                      "$eq": [
                        {
                          "$ref": "response.error.code"
                        },
                        404
                      ]
                    }
                  ]
                },
                {
                  "$or": [
                    {
                      "$not": {
                        "$ref": "response.error.status"
                      }
                    },
                    {
                      "$eq": [
                        {
                          "$ref": "response.error.status"
                        },
                        "NOT_FOUND"
                      ]
                    }
                  ]
                }
              ]
            }
          }
        },
        "response": {
          "status": "succeeded",
          "text": {
            "$map": {
              "from": {
                "$filter": {
                  "from": {
                    "$ref": "response.candidates.0.content.parts"
                  },
                  "as": "part",
                  "where": {
                    "$ne": [
                      {
                        "$ref": "part.thought"
                      },
                      true
                    ]
                  }
                }
              },
              "as": "part",
              "in": {
                "$ref": "part.text"
              }
            }
          },
          "reasoning": {
            "$map": {
              "from": {
                "$filter": {
                  "from": {
                    "$ref": "response.candidates.0.content.parts"
                  },
                  "as": "part",
                  "where": {
                    "$eq": [
                      {
                        "$ref": "part.thought"
                      },
                      true
                    ]
                  }
                }
              },
              "as": "part",
              "in": {
                "$ref": "part.text"
              }
            }
          },
          "error": {
            "$switch": {
              "cases": [
                {
                  "when": {
                    "$ref": "response.error"
                  },
                  "then": {
                    "code": {
                      "$coalesce": [
                        {
                          "$ref": "response.error.status"
                        },
                        {
                          "$ref": "response.error.code"
                        }
                      ]
                    },
                    "message": {
                      "$ref": "response.error.message"
                    }
                  }
                },
                {
                  "when": {
                    "$in": [
                      {
                        "$coalesce": [
                          {
                            "$ref": "response.promptFeedback.blockReason"
                          },
                          {
                            "$ref": "response.candidates.0.finishReason"
                          }
                        ]
                      },
                      [
                        "SAFETY",
                        "BLOCKLIST",
                        "PROHIBITED_CONTENT",
                        "SPII",
                        "IMAGE_SAFETY",
                        "IMAGE_PROHIBITED_CONTENT"
                      ]
                    ]
                  },
                  "then": {
                    "code": "content_policy_violation",
                    "message": "内容未通过上游安全审核，请修改提示词或参考素材后重试"
                  }
                },
                {
                  "when": {
                    "$eq": [
                      {
                        "$coalesce": [
                          {
                            "$ref": "response.promptFeedback.blockReason"
                          },
                          {
                            "$ref": "response.candidates.0.finishReason"
                          }
                        ]
                      },
                      "MAX_TOKENS"
                    ]
                  },
                  "then": {
                    "code": "output_token_limit",
                    "message": "Gemini 输出达到模型 Token 上限，内容未完整生成"
                  }
                },
                {
                  "when": {
                    "$and": [
                      {
                        "$coalesce": [
                          {
                            "$ref": "response.promptFeedback.blockReason"
                          },
                          {
                            "$ref": "response.candidates.0.finishReason"
                          }
                        ]
                      },
                      {
                        "$not": {
                          "$in": [
                            {
                              "$coalesce": [
                                {
                                  "$ref": "response.promptFeedback.blockReason"
                                },
                                {
                                  "$ref": "response.candidates.0.finishReason"
                                }
                              ]
                            },
                            [
                              "STOP",
                              "FINISH_REASON_UNSPECIFIED",
                              "BLOCK_REASON_UNSPECIFIED"
                            ]
                          ]
                        }
                      }
                    ]
                  },
                  "then": {
                    "code": {
                      "$coalesce": [
                        {
                          "$ref": "response.promptFeedback.blockReason"
                        },
                        {
                          "$ref": "response.candidates.0.finishReason"
                        }
                      ]
                    },
                    "message": {
                      "$concat": [
                        "Gemini 未正常完成生成，结束原因：",
                        {
                          "$coalesce": [
                            {
                              "$ref": "response.promptFeedback.blockReason"
                            },
                            {
                              "$ref": "response.candidates.0.finishReason"
                            }
                          ]
                        }
                      ]
                    }
                  }
                }
              ],
              "default": null
            }
          },
          "usage": {
            "$ref": "response.usageMetadata"
          },
          "errorPaths": [
            "error.code"
          ],
          "messagePaths": [
            "error.message"
          ]
        },
        "agentResponse": {
          "text": {
            "$map": {
              "from": {
                "$filter": {
                  "from": {
                    "$ref": "response.candidates.0.content.parts"
                  },
                  "as": "part",
                  "where": {
                    "$ne": [
                      {
                        "$ref": "part.thought"
                      },
                      true
                    ]
                  }
                }
              },
              "as": "part",
              "in": {
                "$ref": "part.text"
              }
            }
          },
          "reasoning": {
            "$map": {
              "from": {
                "$filter": {
                  "from": {
                    "$ref": "response.candidates.0.content.parts"
                  },
                  "as": "part",
                  "where": {
                    "$eq": [
                      {
                        "$ref": "part.thought"
                      },
                      true
                    ]
                  }
                }
              },
              "as": "part",
              "in": {
                "$ref": "part.text"
              }
            }
          },
          "error": {
            "$switch": {
              "cases": [
                {
                  "when": {
                    "$ref": "response.error"
                  },
                  "then": {
                    "code": {
                      "$coalesce": [
                        {
                          "$ref": "response.error.status"
                        },
                        {
                          "$ref": "response.error.code"
                        }
                      ]
                    },
                    "message": {
                      "$ref": "response.error.message"
                    }
                  }
                },
                {
                  "when": {
                    "$in": [
                      {
                        "$coalesce": [
                          {
                            "$ref": "response.promptFeedback.blockReason"
                          },
                          {
                            "$ref": "response.candidates.0.finishReason"
                          }
                        ]
                      },
                      [
                        "SAFETY",
                        "BLOCKLIST",
                        "PROHIBITED_CONTENT",
                        "SPII",
                        "IMAGE_SAFETY",
                        "IMAGE_PROHIBITED_CONTENT"
                      ]
                    ]
                  },
                  "then": {
                    "code": "content_policy_violation",
                    "message": "内容未通过上游安全审核，请修改提示词或参考素材后重试"
                  }
                },
                {
                  "when": {
                    "$eq": [
                      {
                        "$coalesce": [
                          {
                            "$ref": "response.promptFeedback.blockReason"
                          },
                          {
                            "$ref": "response.candidates.0.finishReason"
                          }
                        ]
                      },
                      "MAX_TOKENS"
                    ]
                  },
                  "then": {
                    "code": "output_token_limit",
                    "message": "Gemini 输出达到模型 Token 上限，内容未完整生成"
                  }
                },
                {
                  "when": {
                    "$and": [
                      {
                        "$coalesce": [
                          {
                            "$ref": "response.promptFeedback.blockReason"
                          },
                          {
                            "$ref": "response.candidates.0.finishReason"
                          }
                        ]
                      },
                      {
                        "$not": {
                          "$in": [
                            {
                              "$coalesce": [
                                {
                                  "$ref": "response.promptFeedback.blockReason"
                                },
                                {
                                  "$ref": "response.candidates.0.finishReason"
                                }
                              ]
                            },
                            [
                              "STOP",
                              "FINISH_REASON_UNSPECIFIED",
                              "BLOCK_REASON_UNSPECIFIED"
                            ]
                          ]
                        }
                      }
                    ]
                  },
                  "then": {
                    "code": {
                      "$coalesce": [
                        {
                          "$ref": "response.promptFeedback.blockReason"
                        },
                        {
                          "$ref": "response.candidates.0.finishReason"
                        }
                      ]
                    },
                    "message": {
                      "$concat": [
                        "Gemini 未正常完成生成，结束原因：",
                        {
                          "$coalesce": [
                            {
                              "$ref": "response.promptFeedback.blockReason"
                            },
                            {
                              "$ref": "response.candidates.0.finishReason"
                            }
                          ]
                        }
                      ]
                    }
                  }
                }
              ],
              "default": null
            }
          },
          "toolCallsPath": "candidates.0.content.parts",
          "toolCallIdPaths": [
            "functionCall.id"
          ],
          "toolCallNamePaths": [
            "functionCall.name"
          ],
          "toolCallArgumentsPaths": [
            "functionCall.args"
          ],
          "toolCallThoughtSignaturePaths": [
            "thoughtSignature",
            "thought_signature"
          ]
        }
      }
    ]
  }
}
```
<!-- YINGCE_MANIFEST_CONTRACT_END -->
