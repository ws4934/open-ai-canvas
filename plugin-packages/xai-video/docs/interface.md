# xAI Video 接口字段

## 协议身份

- 插件 ID：`xai-video`。
- Provider ID：`xai-video`。
- 能力：`video`。
- 默认 Base URL：`https://api.x.ai`。
- 鉴权驱动：`bearer`。
- 创建：`POST /v1/videos/generations`。
- 查询：`GET /v1/videos/{{taskId}}`。

## 配置字段

| 字段 | 类型 | 必填 | 含义 |
| --- | --- | --- | --- |
| `apiKey` | secret | 是 | API Key |

## 统一字段映射

| 统一字段 | 类型 | 必填 | 上游映射 | 说明 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | `model` | 视频模型 ID。 |
| `prompt` | string | 否 | `prompt` | 文生、参考及视频编辑需提示词；有首帧、尾帧或关键帧时可省略。 |
| `images` | media[] | 否 | `first/last/reference image` | 显式 role 图片输入。 |
| `videos` | media[] | 否 | `reference video` | 参考视频。 |
| `audios` | media[] | 否 | `reference audio/voice` | 参考音频或音色。 |
| `duration` | integer | 否 | `duration/seconds` | 时长秒数。 |
| `aspectRatio` | string | 否 | `ratio/aspect_ratio/size` | 画幅比例或尺寸。 |
| `resolution` | string | 否 | `resolution` | 分辨率档位。 |
| `generateAudio` | boolean | 否 | `generate_audio` | 是否生成音频。 |
| `watermark` | boolean | 否 | `watermark` | 水印开关。 |
| `providerOptions` | object | 否 | `provider-specific fields` | 插件命名空间内的厂商扩展字段。 |

## 上游请求模板逐字段清单

下表由插件请求模板生成，覆盖 body、query、headers 和 multipart 文件声明中的每个字段。

| 上游位置 | 值或转换表达式 |
| --- | --- |
| `create.method` | `"POST"` |
| `create.path` | `"/v1/videos/generations"` |
| `create.contentType` | `"application/json"` |
| `create.body.model` | `{"$ref":"request.model"}` |
| `create.body.prompt` | `{"$omitEmpty":{"$trim":{"$ref":"request.prompt"}}}` |
| `create.body.duration` | `{"$if":{"condition":{"$or":[{"$not":{"$or":[{"$gt":[{"$len":{"$ref":"request.videos"}},0]},{"$eq":[{"$ref":"request.operation"},"extend"]},{"$in":[{"$ref":"request.operation"},["edit","video_to_video","inpaint","replace_element","camera_motion","style_transfer"]]}]}},{"$eq":[{"$ref":"request.operation"},"extend"]}]},"then":{"$if":{"condition":{"$ne":[{"$ref":"request.duration"},0]},"then":{"$ref":"request.duration"},"else":{"$if":{"condition":{"$eq":[{"$ref":"request.operation"},"extend"]},"then":6,"else":8}}}},"else":null}}` |
| `create.body.aspect_ratio` | `{"$if":{"condition":{"$and":[{"$not":{"$or":[{"$gt":[{"$len":{"$ref":"request.videos"}},0]},{"$eq":[{"$ref":"request.operation"},"extend"]},{"$in":[{"$ref":"request.operation"},["edit","video_to_video","inpaint","replace_element","camera_motion","style_transfer"]]}]}},{"$or":[{"$not":{"$coalesce":[{"$first":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["first_frame"]]}}}},{"$if":{"condition":{"$and":[{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["first_frame"]]}}}},0]},{"$or":[{"$eq":[{"$ref":"request.operation"},"image_to_video"]},{"$and":[{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},[""]]}}}},1]},{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["reference_image","subject_reference","style_reference"]]}}}},0]},{"$ne":[{"$ref":"request.operation"},"reference_to_video"]}]}]}]},"then":{"$first":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},[""]]}}}},"else":null}}]}},{"$or":[{"$gt":[{"$len":{"$sortByOrder":{"$concatArrays":[{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["reference_image","subject_reference","style_reference"]]}}},{"$filter":{"from":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},[""]]}}},"as":"candidate","where":{"$or":[{"$not":{"$and":[{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["first_frame"]]}}}},0]},{"$or":[{"$eq":[{"$ref":"request.operation"},"image_to_video"]},{"$and":[{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},[""]]}}}},1]},{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["reference_image","subject_reference","style_reference"]]}}}},0]},{"$ne":[{"$ref":"request.operation"},"reference_to_video"]}]}]}]}},{"$gt":[{"$ref":"candidateIndex"},0]}]}}}]}}},0]},{"$gt":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["last_frame"]]}}}},0]},{"$gt":[{"$len":{"$ref":"request.providerOptions.xai-video.reference_audios"}},0]},{"$gt":[{"$len":{"$ref":"request.providerOptions.xai-video.keyframes"}},0]}]}]}]},"then":{"$coalesce":[{"$ref":"request.aspectRatio"},"16:9"]},"else":null}}` |
| `create.body.resolution` | `{"$if":{"condition":{"$not":{"$or":[{"$gt":[{"$len":{"$ref":"request.videos"}},0]},{"$eq":[{"$ref":"request.operation"},"extend"]},{"$in":[{"$ref":"request.operation"},["edit","video_to_video","inpaint","replace_element","camera_motion","style_transfer"]]}]}},"then":{"$coalesce":[{"$ref":"request.resolution"},"720p"]},"else":null}}` |
| `create.body.image` | `{"$if":{"condition":{"$coalesce":[{"$first":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["first_frame"]]}}}},{"$if":{"condition":{"$and":[{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["first_frame"]]}}}},0]},{"$or":[{"$eq":[{"$ref":"request.operation"},"image_to_video"]},{"$and":[{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},[""]]}}}},1]},{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["reference_image","subject_reference","style_reference"]]}}}},0]},{"$ne":[{"$ref":"request.operation"},"reference_to_video"]}]}]}]},"then":{"$first":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},[""]]}}}},"else":null}}]},"then":{"url":{"$first":{"$map":{"from":[{"$coalesce":[{"$first":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["first_frame"]]}}}},{"$if":{"condition":{"$and":[{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["first_frame"]]}}}},0]},{"$or":[{"$eq":[{"$ref":"request.operation"},"image_to_video"]},{"$and":[{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},[""]]}}}},1]},{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["reference_image","subject_reference","style_reference"]]}}}},0]},{"$ne":[{"$ref":"request.operation"},"reference_to_video"]}]}]}]},"then":{"$first":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},[""]]}}}},"else":null}}]}],"as":"image","in":{"$ref":"image.value"}}}}},"else":null}}` |
| `create.body.last_frame` | `{"$if":{"condition":{"$gt":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["last_frame"]]}}}},0]},"then":{"url":{"$first":{"$map":{"from":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["last_frame"]]}}},"as":"media","in":{"$ref":"media.value"}}}}},"else":null}}` |
| `create.body.reference_images` | `{"$omitEmpty":{"$map":{"from":{"$sortByOrder":{"$concatArrays":[{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["reference_image","subject_reference","style_reference"]]}}},{"$filter":{"from":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},[""]]}}},"as":"candidate","where":{"$or":[{"$not":{"$and":[{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["first_frame"]]}}}},0]},{"$or":[{"$eq":[{"$ref":"request.operation"},"image_to_video"]},{"$and":[{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},[""]]}}}},1]},{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["reference_image","subject_reference","style_reference"]]}}}},0]},{"$ne":[{"$ref":"request.operation"},"reference_to_video"]}]}]}]}},{"$gt":[{"$ref":"candidateIndex"},0]}]}}}]}},"as":"media","in":{"url":{"$ref":"media.value"}}}}}` |
| `create.body.reference_audios` | `{"$omitEmpty":{"$map":{"from":{"$ref":"request.providerOptions.xai-video.reference_audios"},"as":"voice","in":{"voice_id":{"$lower":{"$trim":{"$ref":"voice.voice_id"}}}}}}}` |
| `create.body.keyframes` | `{"$omitEmpty":{"$map":{"from":{"$ref":"request.providerOptions.xai-video.keyframes"},"as":"frame","in":{"image":{"url":{"$omitEmpty":{"$ref":"frame.image.url"}},"file_id":{"$omitEmpty":{"$ref":"frame.image.file_id"}}},"timestamp_s":{"$toFloat":{"$ref":"frame.timestamp_s"}}}}}}` |
| `create.body.video` | `{"$if":{"condition":{"$or":[{"$gt":[{"$len":{"$ref":"request.videos"}},0]},{"$eq":[{"$ref":"request.operation"},"extend"]},{"$in":[{"$ref":"request.operation"},["edit","video_to_video","inpaint","replace_element","camera_motion","style_transfer"]]}]},"then":{"url":{"$first":{"$map":{"from":{"$ref":"request.videos"},"as":"media","in":{"$ref":"media.value"}}}}},"else":null}}` |
| `poll.method` | `"GET"` |
| `poll.path` | `"/v1/videos/{{taskId}}"` |
| `poll.contentType` | `"application/json"` |

## Provider 扩展键

- `providerOptions.xai-video.keyframes`
- `providerOptions.xai-video.reference_audios`

动态模型或工作流允许使用文档声明的完整 `parameters/input/extra_body` 对象；该对象是协议本身的开放 schema，不会被宿主裁剪。

## 响应映射逐字段清单

| 映射位置 | 上游路径或转换表达式 |
| --- | --- |
| `response.taskId` | `{"$coalesce":[{"$ref":"response.request_id"},{"$ref":"response.id"},{"$ref":"response.task_id"},{"$ref":"response.taskId"},{"$ref":"response.data.id"},{"$ref":"response.data.task_id"},{"$ref":"response.data.taskId"},{"$ref":"response.data.request_id"},{"$ref":"response.task.id"},{"$ref":"response.task.task_id"},{"$ref":"response.task.taskId"},{"$ref":"response.task.request_id"},{"$ref":"taskId"}]}` |
| `response.status` | `{"$if":{"condition":{"$eq":[{"$lower":{"$coalesce":[{"$ref":"response.status"},{"$ref":"response.state"},{"$ref":"response.data.status"},{"$ref":"response.data.state"},{"$ref":"response.task.status"},{"$ref":"response.task.state"},"pending"]}},"expired"]},"then":"failed","else":{"$coalesce":[{"$ref":"response.status"},{"$ref":"response.state"},{"$ref":"response.data.status"},{"$ref":"response.data.state"},{"$ref":"response.task.status"},{"$ref":"response.task.state"},"pending"]}}}` |
| `response.message` | `{"$coalesce":[{"$ref":"response.error.message"},{"$ref":"response.message"},{"$ref":"response.fail_reason"},{"$if":{"condition":{"$eq":[{"$lower":{"$coalesce":[{"$ref":"response.status"},{"$ref":"response.state"},{"$ref":"response.data.status"},{"$ref":"response.data.state"},{"$ref":"response.task.status"},{"$ref":"response.task.state"},"pending"]}},"expired"]},"then":"xAI 视频任务已过期，请重新生成","else":null}}]}` |
| `response.videos` | `{"$coalesce":[{"$ref":"response.video.url"},{"$ref":"response.video_url"},{"$ref":"response.videoUrl"},{"$ref":"response.result_url"},{"$ref":"response.url"},{"$ref":"response.videos"},{"$ref":"response.data.video_url"},{"$ref":"response.data.video.url"},{"$ref":"response.data.videos"},{"$ref":"response.task.video.url"},{"$ref":"response.task.videos"},{"$ref":"response.content.video_url"},{"$ref":"response.content.url"},{"$ref":"response.output.url"}]}` |
| `response.errorPaths[0]` | `"error.code"` |
| `response.resultEphemeral` | `true` |

## 响应与错误

插件把上游 task/status/text/media/usage 映射为统一结果。临时媒体 URL 标记为 ephemeral，由宿主立即下载持久化。HTTP 错误、业务 code 和 error object 保持失败语义，不包装成成功。

## 兼容边界

支持官方 grok-imagine-video 与 grok-imagine-video-1.5，并明确区分只支持基础生成的 1.5-lite。新视频建议完整 1.5；编辑、延长已有视频使用旧版。官方 ID 精确识别，网关模型别名可在 metadata.providerOptions["xai-video"].modelProfile 显式选择 video / video-1.5 / video-1.5-lite，不按名称前缀推断。

普通生成默认 8 秒，范围 1–15 秒；旧版参考模式最多 7 图、10 秒，完整 1.5 参考模式最多 14 图、15 秒。完整 1.5 可同时携带首帧、14 张参考图、一个尾帧和最多 4 个关键帧。基础文生或仅首帧的 1.5 可用原生 1080p；其他参考与定帧模式最高 720p，不静默降档。图生画幅跟随首帧，基础首帧请求不发送无效的 aspect_ratio。未标记图片在图生模式下首张作首帧、其余作参考图；参考模式全部作参考图；图片不会被截断。

扩展字段放在 metadata.providerOptions["xai-video"]：reference_audios 为最多 3 个 {"voice_id":"eve"}，只支持官方预设音色；keyframes 为最多 4 个 {"image":{"url":"https://..."},"timestamp_s":2.0}，image 也支持 file_id。时间必须在 (0,duration) 内，不能落在同一个 1/3 秒网格。图片与音色在提示词用 <IMAGE_0> / <AUDIO_0> 指定；同时提供首帧时首帧占 <IMAGE_0>，参考图从 <IMAGE_1> 开始。

视频操作复用 metadata.videoEditOperation：extend 调用 /v1/videos/extensions，新增部分 2–10 秒、缺省 6 秒；inpaint / replace_element / camera_motion / style_transfer 调用 /v1/videos/edits。标准 videos 传一个源视频，已知 durationMs 时分别校验编辑最多 8.7 秒、延长 2–15 秒。没有探测时长的外部源仍由上游校验，不在插件中虚构时长。编辑不发 duration；两种源视频操作均不发 aspect_ratio / resolution，输出沿用源视频并受上游 720p 上限约束。

xAI 原生生成音轨，不支持通用 generate_audio / watermark 字段，插件不发送这两个不存在的开关。完整 1.5 支持对口型与预设音色；自有音频音色需官方额外权限，普通参考音频文件明确拒绝。不会按官方美元报价改写站点运营计价。

官方文档：[能力总表](https://docs.x.ai/developers/model-capabilities/video/overview)、[参考图、音色与定帧](https://docs.x.ai/developers/model-capabilities/video/reference-to-video)、[生成参数](https://docs.x.ai/developers/model-capabilities/video/generation)、[视频编辑](https://docs.x.ai/developers/model-capabilities/video/editing)、[视频延长](https://docs.x.ai/developers/model-capabilities/video/extension)。

<!-- YINGCE_MANIFEST_CONTRACT_START -->
## Manifest 完整接口定义

以下 JSON 与插件包内实际 `manifest.json` 逐字段一致，覆盖插件身份、权限、配置、鉴权、参数、校验、创建、Agent、查询、取消、结果下载、响应和 Agent 响应映射。`documentation` 字段的值就是当前完整文档；为避免文档在自身内部无限递归，JSON 中仅用等义占位文本表示正文。

```json
{
  "apiVersion": "yingce.plugin/v2",
  "id": "xai-video",
  "name": "xAI Video",
  "version": "2.1.0",
  "author": "xAI / 影策",
  "description": "xAI Video 独立请求协议插件。",
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
        "id": "xai-video",
        "label": "xAI Video",
        "capabilities": [
          "video"
        ],
        "scopes": [
          "admin.system-channel",
          "user.custom-channel",
          "canvas",
          "creation",
          "agent"
        ],
        "baseUrl": "https://api.x.ai",
        "requiresPublicMediaUrls": true,
        "auth": {
          "type": "bearer",
          "field": "apiKey"
        },
        "parameters": [
          {
            "name": "model",
            "type": "string",
            "required": true,
            "mapping": "model",
            "description": "视频模型 ID。"
          },
          {
            "name": "prompt",
            "type": "string",
            "required": false,
            "mapping": "prompt",
            "description": "文生、参考及视频编辑需提示词；有首帧、尾帧或关键帧时可省略。"
          },
          {
            "name": "images",
            "type": "media[]",
            "required": false,
            "mapping": "first/last/reference image",
            "description": "显式 role 图片输入。"
          },
          {
            "name": "videos",
            "type": "media[]",
            "required": false,
            "mapping": "reference video",
            "description": "参考视频。"
          },
          {
            "name": "audios",
            "type": "media[]",
            "required": false,
            "mapping": "reference audio/voice",
            "description": "参考音频或音色。"
          },
          {
            "name": "duration",
            "type": "integer",
            "required": false,
            "mapping": "duration/seconds",
            "description": "时长秒数。"
          },
          {
            "name": "aspectRatio",
            "type": "string",
            "required": false,
            "mapping": "ratio/aspect_ratio/size",
            "description": "画幅比例或尺寸。"
          },
          {
            "name": "resolution",
            "type": "string",
            "required": false,
            "mapping": "resolution",
            "description": "分辨率档位。"
          },
          {
            "name": "generateAudio",
            "type": "boolean",
            "required": false,
            "mapping": "generate_audio",
            "description": "是否生成音频。"
          },
          {
            "name": "watermark",
            "type": "boolean",
            "required": false,
            "mapping": "watermark",
            "description": "水印开关。"
          },
          {
            "name": "providerOptions",
            "type": "object",
            "required": false,
            "mapping": "provider-specific fields",
            "description": "插件命名空间内的厂商扩展字段。"
          }
        ],
        "validations": [
          {
            "assert": {
              "$in": [
                {
                  "$coalesce": [
                    {
                      "$ref": "request.providerOptions.xai-video.modelProfile"
                    },
                    {
                      "$if": {
                        "condition": {
                          "$eq": [
                            {
                              "$ref": "request.model"
                            },
                            "grok-imagine-video-1.5"
                          ]
                        },
                        "then": "video-1.5",
                        "else": {
                          "$if": {
                            "condition": {
                              "$eq": [
                                {
                                  "$ref": "request.model"
                                },
                                "grok-imagine-video-1.5-lite"
                              ]
                            },
                            "then": "video-1.5-lite",
                            "else": "video"
                          }
                        }
                      }
                    }
                  ]
                },
                [
                  "video",
                  "video-1.5",
                  "video-1.5-lite"
                ]
              ]
            },
            "message": "xAI modelProfile 只能是 video、video-1.5 或 video-1.5-lite"
          },
          {
            "assert": {
              "$or": [
                {
                  "$not": {
                    "$in": [
                      {
                        "$ref": "request.model"
                      },
                      [
                        "grok-imagine-video",
                        "grok-imagine-video-1.5",
                        "grok-imagine-video-1.5-lite"
                      ]
                    ]
                  }
                },
                {
                  "$eq": [
                    {
                      "$coalesce": [
                        {
                          "$ref": "request.providerOptions.xai-video.modelProfile"
                        },
                        {
                          "$if": {
                            "condition": {
                              "$eq": [
                                {
                                  "$ref": "request.model"
                                },
                                "grok-imagine-video-1.5"
                              ]
                            },
                            "then": "video-1.5",
                            "else": {
                              "$if": {
                                "condition": {
                                  "$eq": [
                                    {
                                      "$ref": "request.model"
                                    },
                                    "grok-imagine-video-1.5-lite"
                                  ]
                                },
                                "then": "video-1.5-lite",
                                "else": "video"
                              }
                            }
                          }
                        }
                      ]
                    },
                    {
                      "$if": {
                        "condition": {
                          "$eq": [
                            {
                              "$ref": "request.model"
                            },
                            "grok-imagine-video-1.5"
                          ]
                        },
                        "then": "video-1.5",
                        "else": {
                          "$if": {
                            "condition": {
                              "$eq": [
                                {
                                  "$ref": "request.model"
                                },
                                "grok-imagine-video-1.5-lite"
                              ]
                            },
                            "then": "video-1.5-lite",
                            "else": "video"
                          }
                        }
                      }
                    }
                  ]
                }
              ]
            },
            "message": "xAI 官方模型与显式 modelProfile 不匹配"
          },
          {
            "assert": {
              "$lte": [
                {
                  "$len": {
                    "$filter": {
                      "from": {
                        "$sortByOrder": {
                          "$ref": "request.images"
                        }
                      },
                      "as": "media",
                      "where": {
                        "$in": [
                          {
                            "$ref": "media.role"
                          },
                          [
                            "first_frame"
                          ]
                        ]
                      }
                    }
                  }
                },
                1
              ]
            },
            "message": "xAI Video 最多支持一个首帧"
          },
          {
            "assert": {
              "$lte": [
                {
                  "$len": {
                    "$filter": {
                      "from": {
                        "$sortByOrder": {
                          "$ref": "request.images"
                        }
                      },
                      "as": "media",
                      "where": {
                        "$in": [
                          {
                            "$ref": "media.role"
                          },
                          [
                            "last_frame"
                          ]
                        ]
                      }
                    }
                  }
                },
                1
              ]
            },
            "message": "xAI Video 最多支持一个尾帧"
          },
          {
            "assert": {
              "$eq": [
                {
                  "$len": {
                    "$filter": {
                      "from": {
                        "$ref": "request.images"
                      },
                      "as": "media",
                      "where": {
                        "$not": {
                          "$in": [
                            {
                              "$ref": "media.role"
                            },
                            [
                              "",
                              "first_frame",
                              "last_frame",
                              "reference_image",
                              "subject_reference",
                              "style_reference"
                            ]
                          ]
                        }
                      }
                    }
                  }
                },
                0
              ]
            },
            "message": "xAI Video 收到了不支持的图片角色"
          },
          {
            "assert": {
              "$eq": [
                {
                  "$len": {
                    "$filter": {
                      "from": {
                        "$ref": "request.images"
                      },
                      "as": "media",
                      "where": {
                        "$gt": [
                          {
                            "$len": {
                              "$trim": {
                                "$ref": "media.value"
                              }
                            }
                          },
                          0
                        ]
                      }
                    }
                  }
                },
                {
                  "$len": {
                    "$ref": "request.images"
                  }
                }
              ]
            },
            "message": "xAI 图片输入不能为空"
          },
          {
            "assert": {
              "$or": [
                {
                  "$eq": [
                    {
                      "$coalesce": [
                        {
                          "$ref": "request.providerOptions.xai-video.modelProfile"
                        },
                        {
                          "$if": {
                            "condition": {
                              "$eq": [
                                {
                                  "$ref": "request.model"
                                },
                                "grok-imagine-video-1.5"
                              ]
                            },
                            "then": "video-1.5",
                            "else": {
                              "$if": {
                                "condition": {
                                  "$eq": [
                                    {
                                      "$ref": "request.model"
                                    },
                                    "grok-imagine-video-1.5-lite"
                                  ]
                                },
                                "then": "video-1.5-lite",
                                "else": "video"
                              }
                            }
                          }
                        }
                      ]
                    },
                    "video-1.5"
                  ]
                },
                {
                  "$eq": [
                    {
                      "$len": {
                        "$filter": {
                          "from": {
                            "$sortByOrder": {
                              "$ref": "request.images"
                            }
                          },
                          "as": "media",
                          "where": {
                            "$in": [
                              {
                                "$ref": "media.role"
                              },
                              [
                                "last_frame"
                              ]
                            ]
                          }
                        }
                      }
                    },
                    0
                  ]
                }
              ]
            },
            "message": "尾帧需要完整的 Video 1.5 模型"
          },
          {
            "assert": {
              "$or": [
                {
                  "$eq": [
                    {
                      "$coalesce": [
                        {
                          "$ref": "request.providerOptions.xai-video.modelProfile"
                        },
                        {
                          "$if": {
                            "condition": {
                              "$eq": [
                                {
                                  "$ref": "request.model"
                                },
                                "grok-imagine-video-1.5"
                              ]
                            },
                            "then": "video-1.5",
                            "else": {
                              "$if": {
                                "condition": {
                                  "$eq": [
                                    {
                                      "$ref": "request.model"
                                    },
                                    "grok-imagine-video-1.5-lite"
                                  ]
                                },
                                "then": "video-1.5-lite",
                                "else": "video"
                              }
                            }
                          }
                        }
                      ]
                    },
                    "video-1.5"
                  ]
                },
                {
                  "$eq": [
                    {
                      "$len": {
                        "$ref": "request.providerOptions.xai-video.keyframes"
                      }
                    },
                    0
                  ]
                }
              ]
            },
            "message": "关键帧需要完整的 Video 1.5 模型"
          },
          {
            "assert": {
              "$or": [
                {
                  "$eq": [
                    {
                      "$coalesce": [
                        {
                          "$ref": "request.providerOptions.xai-video.modelProfile"
                        },
                        {
                          "$if": {
                            "condition": {
                              "$eq": [
                                {
                                  "$ref": "request.model"
                                },
                                "grok-imagine-video-1.5"
                              ]
                            },
                            "then": "video-1.5",
                            "else": {
                              "$if": {
                                "condition": {
                                  "$eq": [
                                    {
                                      "$ref": "request.model"
                                    },
                                    "grok-imagine-video-1.5-lite"
                                  ]
                                },
                                "then": "video-1.5-lite",
                                "else": "video"
                              }
                            }
                          }
                        }
                      ]
                    },
                    "video-1.5"
                  ]
                },
                {
                  "$eq": [
                    {
                      "$len": {
                        "$ref": "request.providerOptions.xai-video.reference_audios"
                      }
                    },
                    0
                  ]
                }
              ]
            },
            "message": "预设音色需要完整的 Video 1.5 模型"
          },
          {
            "assert": {
              "$or": [
                {
                  "$not": {
                    "$eq": [
                      {
                        "$coalesce": [
                          {
                            "$ref": "request.providerOptions.xai-video.modelProfile"
                          },
                          {
                            "$if": {
                              "condition": {
                                "$eq": [
                                  {
                                    "$ref": "request.model"
                                  },
                                  "grok-imagine-video-1.5"
                                ]
                              },
                              "then": "video-1.5",
                              "else": {
                                "$if": {
                                  "condition": {
                                    "$eq": [
                                      {
                                        "$ref": "request.model"
                                      },
                                      "grok-imagine-video-1.5-lite"
                                    ]
                                  },
                                  "then": "video-1.5-lite",
                                  "else": "video"
                                }
                              }
                            }
                          }
                        ]
                      },
                      "video"
                    ]
                  }
                },
                {
                  "$not": {
                    "$and": [
                      {
                        "$coalesce": [
                          {
                            "$first": {
                              "$filter": {
                                "from": {
                                  "$sortByOrder": {
                                    "$ref": "request.images"
                                  }
                                },
                                "as": "media",
                                "where": {
                                  "$in": [
                                    {
                                      "$ref": "media.role"
                                    },
                                    [
                                      "first_frame"
                                    ]
                                  ]
                                }
                              }
                            }
                          },
                          {
                            "$if": {
                              "condition": {
                                "$and": [
                                  {
                                    "$eq": [
                                      {
                                        "$len": {
                                          "$filter": {
                                            "from": {
                                              "$sortByOrder": {
                                                "$ref": "request.images"
                                              }
                                            },
                                            "as": "media",
                                            "where": {
                                              "$in": [
                                                {
                                                  "$ref": "media.role"
                                                },
                                                [
                                                  "first_frame"
                                                ]
                                              ]
                                            }
                                          }
                                        }
                                      },
                                      0
                                    ]
                                  },
                                  {
                                    "$or": [
                                      {
                                        "$eq": [
                                          {
                                            "$ref": "request.operation"
                                          },
                                          "image_to_video"
                                        ]
                                      },
                                      {
                                        "$and": [
                                          {
                                            "$eq": [
                                              {
                                                "$len": {
                                                  "$filter": {
                                                    "from": {
                                                      "$sortByOrder": {
                                                        "$ref": "request.images"
                                                      }
                                                    },
                                                    "as": "media",
                                                    "where": {
                                                      "$in": [
                                                        {
                                                          "$ref": "media.role"
                                                        },
                                                        [
                                                          ""
                                                        ]
                                                      ]
                                                    }
                                                  }
                                                }
                                              },
                                              1
                                            ]
                                          },
                                          {
                                            "$eq": [
                                              {
                                                "$len": {
                                                  "$filter": {
                                                    "from": {
                                                      "$sortByOrder": {
                                                        "$ref": "request.images"
                                                      }
                                                    },
                                                    "as": "media",
                                                    "where": {
                                                      "$in": [
                                                        {
                                                          "$ref": "media.role"
                                                        },
                                                        [
                                                          "reference_image",
                                                          "subject_reference",
                                                          "style_reference"
                                                        ]
                                                      ]
                                                    }
                                                  }
                                                }
                                              },
                                              0
                                            ]
                                          },
                                          {
                                            "$ne": [
                                              {
                                                "$ref": "request.operation"
                                              },
                                              "reference_to_video"
                                            ]
                                          }
                                        ]
                                      }
                                    ]
                                  }
                                ]
                              },
                              "then": {
                                "$first": {
                                  "$filter": {
                                    "from": {
                                      "$sortByOrder": {
                                        "$ref": "request.images"
                                      }
                                    },
                                    "as": "media",
                                    "where": {
                                      "$in": [
                                        {
                                          "$ref": "media.role"
                                        },
                                        [
                                          ""
                                        ]
                                      ]
                                    }
                                  }
                                }
                              },
                              "else": null
                            }
                          }
                        ]
                      },
                      {
                        "$gt": [
                          {
                            "$len": {
                              "$sortByOrder": {
                                "$concatArrays": [
                                  {
                                    "$filter": {
                                      "from": {
                                        "$sortByOrder": {
                                          "$ref": "request.images"
                                        }
                                      },
                                      "as": "media",
                                      "where": {
                                        "$in": [
                                          {
                                            "$ref": "media.role"
                                          },
                                          [
                                            "reference_image",
                                            "subject_reference",
                                            "style_reference"
                                          ]
                                        ]
                                      }
                                    }
                                  },
                                  {
                                    "$filter": {
                                      "from": {
                                        "$filter": {
                                          "from": {
                                            "$sortByOrder": {
                                              "$ref": "request.images"
                                            }
                                          },
                                          "as": "media",
                                          "where": {
                                            "$in": [
                                              {
                                                "$ref": "media.role"
                                              },
                                              [
                                                ""
                                              ]
                                            ]
                                          }
                                        }
                                      },
                                      "as": "candidate",
                                      "where": {
                                        "$or": [
                                          {
                                            "$not": {
                                              "$and": [
                                                {
                                                  "$eq": [
                                                    {
                                                      "$len": {
                                                        "$filter": {
                                                          "from": {
                                                            "$sortByOrder": {
                                                              "$ref": "request.images"
                                                            }
                                                          },
                                                          "as": "media",
                                                          "where": {
                                                            "$in": [
                                                              {
                                                                "$ref": "media.role"
                                                              },
                                                              [
                                                                "first_frame"
                                                              ]
                                                            ]
                                                          }
                                                        }
                                                      }
                                                    },
                                                    0
                                                  ]
                                                },
                                                {
                                                  "$or": [
                                                    {
                                                      "$eq": [
                                                        {
                                                          "$ref": "request.operation"
                                                        },
                                                        "image_to_video"
                                                      ]
                                                    },
                                                    {
                                                      "$and": [
                                                        {
                                                          "$eq": [
                                                            {
                                                              "$len": {
                                                                "$filter": {
                                                                  "from": {
                                                                    "$sortByOrder": {
                                                                      "$ref": "request.images"
                                                                    }
                                                                  },
                                                                  "as": "media",
                                                                  "where": {
                                                                    "$in": [
                                                                      {
                                                                        "$ref": "media.role"
                                                                      },
                                                                      [
                                                                        ""
                                                                      ]
                                                                    ]
                                                                  }
                                                                }
                                                              }
                                                            },
                                                            1
                                                          ]
                                                        },
                                                        {
                                                          "$eq": [
                                                            {
                                                              "$len": {
                                                                "$filter": {
                                                                  "from": {
                                                                    "$sortByOrder": {
                                                                      "$ref": "request.images"
                                                                    }
                                                                  },
                                                                  "as": "media",
                                                                  "where": {
                                                                    "$in": [
                                                                      {
                                                                        "$ref": "media.role"
                                                                      },
                                                                      [
                                                                        "reference_image",
                                                                        "subject_reference",
                                                                        "style_reference"
                                                                      ]
                                                                    ]
                                                                  }
                                                                }
                                                              }
                                                            },
                                                            0
                                                          ]
                                                        },
                                                        {
                                                          "$ne": [
                                                            {
                                                              "$ref": "request.operation"
                                                            },
                                                            "reference_to_video"
                                                          ]
                                                        }
                                                      ]
                                                    }
                                                  ]
                                                }
                                              ]
                                            }
                                          },
                                          {
                                            "$gt": [
                                              {
                                                "$ref": "candidateIndex"
                                              },
                                              0
                                            ]
                                          }
                                        ]
                                      }
                                    }
                                  }
                                ]
                              }
                            }
                          },
                          0
                        ]
                      }
                    ]
                  }
                }
              ]
            },
            "message": "旧版 xAI Video 不能同时混用起始帧和角色参考图，请改用 Video 1.5"
          },
          {
            "assert": {
              "$lte": [
                {
                  "$len": {
                    "$sortByOrder": {
                      "$concatArrays": [
                        {
                          "$filter": {
                            "from": {
                              "$sortByOrder": {
                                "$ref": "request.images"
                              }
                            },
                            "as": "media",
                            "where": {
                              "$in": [
                                {
                                  "$ref": "media.role"
                                },
                                [
                                  "reference_image",
                                  "subject_reference",
                                  "style_reference"
                                ]
                              ]
                            }
                          }
                        },
                        {
                          "$filter": {
                            "from": {
                              "$filter": {
                                "from": {
                                  "$sortByOrder": {
                                    "$ref": "request.images"
                                  }
                                },
                                "as": "media",
                                "where": {
                                  "$in": [
                                    {
                                      "$ref": "media.role"
                                    },
                                    [
                                      ""
                                    ]
                                  ]
                                }
                              }
                            },
                            "as": "candidate",
                            "where": {
                              "$or": [
                                {
                                  "$not": {
                                    "$and": [
                                      {
                                        "$eq": [
                                          {
                                            "$len": {
                                              "$filter": {
                                                "from": {
                                                  "$sortByOrder": {
                                                    "$ref": "request.images"
                                                  }
                                                },
                                                "as": "media",
                                                "where": {
                                                  "$in": [
                                                    {
                                                      "$ref": "media.role"
                                                    },
                                                    [
                                                      "first_frame"
                                                    ]
                                                  ]
                                                }
                                              }
                                            }
                                          },
                                          0
                                        ]
                                      },
                                      {
                                        "$or": [
                                          {
                                            "$eq": [
                                              {
                                                "$ref": "request.operation"
                                              },
                                              "image_to_video"
                                            ]
                                          },
                                          {
                                            "$and": [
                                              {
                                                "$eq": [
                                                  {
                                                    "$len": {
                                                      "$filter": {
                                                        "from": {
                                                          "$sortByOrder": {
                                                            "$ref": "request.images"
                                                          }
                                                        },
                                                        "as": "media",
                                                        "where": {
                                                          "$in": [
                                                            {
                                                              "$ref": "media.role"
                                                            },
                                                            [
                                                              ""
                                                            ]
                                                          ]
                                                        }
                                                      }
                                                    }
                                                  },
                                                  1
                                                ]
                                              },
                                              {
                                                "$eq": [
                                                  {
                                                    "$len": {
                                                      "$filter": {
                                                        "from": {
                                                          "$sortByOrder": {
                                                            "$ref": "request.images"
                                                          }
                                                        },
                                                        "as": "media",
                                                        "where": {
                                                          "$in": [
                                                            {
                                                              "$ref": "media.role"
                                                            },
                                                            [
                                                              "reference_image",
                                                              "subject_reference",
                                                              "style_reference"
                                                            ]
                                                          ]
                                                        }
                                                      }
                                                    }
                                                  },
                                                  0
                                                ]
                                              },
                                              {
                                                "$ne": [
                                                  {
                                                    "$ref": "request.operation"
                                                  },
                                                  "reference_to_video"
                                                ]
                                              }
                                            ]
                                          }
                                        ]
                                      }
                                    ]
                                  }
                                },
                                {
                                  "$gt": [
                                    {
                                      "$ref": "candidateIndex"
                                    },
                                    0
                                  ]
                                }
                              ]
                            }
                          }
                        }
                      ]
                    }
                  }
                },
                {
                  "$if": {
                    "condition": {
                      "$eq": [
                        {
                          "$coalesce": [
                            {
                              "$ref": "request.providerOptions.xai-video.modelProfile"
                            },
                            {
                              "$if": {
                                "condition": {
                                  "$eq": [
                                    {
                                      "$ref": "request.model"
                                    },
                                    "grok-imagine-video-1.5"
                                  ]
                                },
                                "then": "video-1.5",
                                "else": {
                                  "$if": {
                                    "condition": {
                                      "$eq": [
                                        {
                                          "$ref": "request.model"
                                        },
                                        "grok-imagine-video-1.5-lite"
                                      ]
                                    },
                                    "then": "video-1.5-lite",
                                    "else": "video"
                                  }
                                }
                              }
                            }
                          ]
                        },
                        "video-1.5"
                      ]
                    },
                    "then": 14,
                    "else": {
                      "$if": {
                        "condition": {
                          "$eq": [
                            {
                              "$coalesce": [
                                {
                                  "$ref": "request.providerOptions.xai-video.modelProfile"
                                },
                                {
                                  "$if": {
                                    "condition": {
                                      "$eq": [
                                        {
                                          "$ref": "request.model"
                                        },
                                        "grok-imagine-video-1.5"
                                      ]
                                    },
                                    "then": "video-1.5",
                                    "else": {
                                      "$if": {
                                        "condition": {
                                          "$eq": [
                                            {
                                              "$ref": "request.model"
                                            },
                                            "grok-imagine-video-1.5-lite"
                                          ]
                                        },
                                        "then": "video-1.5-lite",
                                        "else": "video"
                                      }
                                    }
                                  }
                                }
                              ]
                            },
                            "video"
                          ]
                        },
                        "then": 7,
                        "else": 0
                      }
                    }
                  }
                }
              ]
            },
            "message": "xAI Video 1.5 最多 14 张参考图，旧版最多 7 张，Lite 不支持参考图"
          },
          {
            "assert": {
              "$eq": [
                {
                  "$len": {
                    "$ref": "request.audios"
                  }
                },
                0
              ]
            },
            "message": "xAI 公共视频接口不接收参考音频文件；Video 1.5 请用 reference_audios 的预设 voice_id"
          },
          {
            "assert": {
              "$lte": [
                {
                  "$len": {
                    "$ref": "request.providerOptions.xai-video.reference_audios"
                  }
                },
                3
              ]
            },
            "message": "Video 1.5 最多使用 3 个预设音色"
          },
          {
            "assert": {
              "$eq": [
                {
                  "$len": {
                    "$filter": {
                      "from": {
                        "$ref": "request.providerOptions.xai-video.reference_audios"
                      },
                      "as": "voice",
                      "where": {
                        "$and": [
                          {
                            "$gt": [
                              {
                                "$len": {
                                  "$trim": {
                                    "$ref": "voice.voice_id"
                                  }
                                }
                              },
                              0
                            ]
                          },
                          {
                            "$not": {
                              "$ref": "voice.url"
                            }
                          },
                          {
                            "$not": {
                              "$ref": "voice.file_id"
                            }
                          }
                        ]
                      }
                    }
                  }
                },
                {
                  "$len": {
                    "$ref": "request.providerOptions.xai-video.reference_audios"
                  }
                }
              ]
            },
            "message": "reference_audios 仅支持非空 voice_id；自有音频文件需要 xAI 额外授权"
          },
          {
            "assert": {
              "$lte": [
                {
                  "$len": {
                    "$ref": "request.providerOptions.xai-video.keyframes"
                  }
                },
                4
              ]
            },
            "message": "Video 1.5 最多使用 4 个中间关键帧"
          },
          {
            "assert": {
              "$eq": [
                {
                  "$len": {
                    "$filter": {
                      "from": {
                        "$ref": "request.providerOptions.xai-video.keyframes"
                      },
                      "as": "frame",
                      "where": {
                        "$and": [
                          {
                            "$and": [
                              {
                                "$or": [
                                  {
                                    "$gt": [
                                      {
                                        "$len": {
                                          "$trim": {
                                            "$ref": "frame.image.url"
                                          }
                                        }
                                      },
                                      0
                                    ]
                                  },
                                  {
                                    "$gt": [
                                      {
                                        "$len": {
                                          "$trim": {
                                            "$ref": "frame.image.file_id"
                                          }
                                        }
                                      },
                                      0
                                    ]
                                  }
                                ]
                              },
                              {
                                "$not": {
                                  "$and": [
                                    {
                                      "$gt": [
                                        {
                                          "$len": {
                                            "$trim": {
                                              "$ref": "frame.image.url"
                                            }
                                          }
                                        },
                                        0
                                      ]
                                    },
                                    {
                                      "$gt": [
                                        {
                                          "$len": {
                                            "$trim": {
                                              "$ref": "frame.image.file_id"
                                            }
                                          }
                                        },
                                        0
                                      ]
                                    }
                                  ]
                                }
                              }
                            ]
                          },
                          {
                            "$gt": [
                              {
                                "$toFloat": {
                                  "$ref": "frame.timestamp_s"
                                }
                              },
                              0
                            ]
                          },
                          {
                            "$lt": [
                              {
                                "$toFloat": {
                                  "$ref": "frame.timestamp_s"
                                }
                              },
                              {
                                "$if": {
                                  "condition": {
                                    "$ne": [
                                      {
                                        "$ref": "request.duration"
                                      },
                                      0
                                    ]
                                  },
                                  "then": {
                                    "$ref": "request.duration"
                                  },
                                  "else": {
                                    "$if": {
                                      "condition": {
                                        "$eq": [
                                          {
                                            "$ref": "request.operation"
                                          },
                                          "extend"
                                        ]
                                      },
                                      "then": 6,
                                      "else": 8
                                    }
                                  }
                                }
                              }
                            ]
                          }
                        ]
                      }
                    }
                  }
                },
                {
                  "$len": {
                    "$ref": "request.providerOptions.xai-video.keyframes"
                  }
                }
              ]
            },
            "message": "每个关键帧需 image.url 或 image.file_id，timestamp_s 必须大于 0 且小于视频时长"
          },
          {
            "assert": {
              "$eq": [
                {
                  "$len": {
                    "$filter": {
                      "from": {
                        "$ref": "request.providerOptions.xai-video.keyframes"
                      },
                      "as": "frame",
                      "where": {
                        "$gt": [
                          {
                            "$len": {
                              "$filter": {
                                "from": {
                                  "$ref": "request.providerOptions.xai-video.keyframes"
                                },
                                "as": "other",
                                "where": {
                                  "$eq": [
                                    {
                                      "$toInt": {
                                        "$add": [
                                          {
                                            "$multiply": [
                                              {
                                                "$ref": "frame.timestamp_s"
                                              },
                                              3
                                            ]
                                          },
                                          0.5
                                        ]
                                      }
                                    },
                                    {
                                      "$toInt": {
                                        "$add": [
                                          {
                                            "$multiply": [
                                              {
                                                "$ref": "other.timestamp_s"
                                              },
                                              3
                                            ]
                                          },
                                          0.5
                                        ]
                                      }
                                    }
                                  ]
                                }
                              }
                            }
                          },
                          1
                        ]
                      }
                    }
                  }
                },
                0
              ]
            },
            "message": "关键帧不能落在同一个 1/3 秒时间网格内"
          },
          {
            "assert": {
              "$or": [
                {
                  "$or": [
                    {
                      "$gt": [
                        {
                          "$len": {
                            "$ref": "request.videos"
                          }
                        },
                        0
                      ]
                    },
                    {
                      "$eq": [
                        {
                          "$ref": "request.operation"
                        },
                        "extend"
                      ]
                    },
                    {
                      "$in": [
                        {
                          "$ref": "request.operation"
                        },
                        [
                          "edit",
                          "video_to_video",
                          "inpaint",
                          "replace_element",
                          "camera_motion",
                          "style_transfer"
                        ]
                      ]
                    }
                  ]
                },
                {
                  "$and": [
                    {
                      "$gte": [
                        {
                          "$if": {
                            "condition": {
                              "$ne": [
                                {
                                  "$ref": "request.duration"
                                },
                                0
                              ]
                            },
                            "then": {
                              "$ref": "request.duration"
                            },
                            "else": {
                              "$if": {
                                "condition": {
                                  "$eq": [
                                    {
                                      "$ref": "request.operation"
                                    },
                                    "extend"
                                  ]
                                },
                                "then": 6,
                                "else": 8
                              }
                            }
                          }
                        },
                        1
                      ]
                    },
                    {
                      "$lte": [
                        {
                          "$if": {
                            "condition": {
                              "$ne": [
                                {
                                  "$ref": "request.duration"
                                },
                                0
                              ]
                            },
                            "then": {
                              "$ref": "request.duration"
                            },
                            "else": {
                              "$if": {
                                "condition": {
                                  "$eq": [
                                    {
                                      "$ref": "request.operation"
                                    },
                                    "extend"
                                  ]
                                },
                                "then": 6,
                                "else": 8
                              }
                            }
                          }
                        },
                        {
                          "$if": {
                            "condition": {
                              "$and": [
                                {
                                  "$eq": [
                                    {
                                      "$coalesce": [
                                        {
                                          "$ref": "request.providerOptions.xai-video.modelProfile"
                                        },
                                        {
                                          "$if": {
                                            "condition": {
                                              "$eq": [
                                                {
                                                  "$ref": "request.model"
                                                },
                                                "grok-imagine-video-1.5"
                                              ]
                                            },
                                            "then": "video-1.5",
                                            "else": {
                                              "$if": {
                                                "condition": {
                                                  "$eq": [
                                                    {
                                                      "$ref": "request.model"
                                                    },
                                                    "grok-imagine-video-1.5-lite"
                                                  ]
                                                },
                                                "then": "video-1.5-lite",
                                                "else": "video"
                                              }
                                            }
                                          }
                                        }
                                      ]
                                    },
                                    "video"
                                  ]
                                },
                                {
                                  "$or": [
                                    {
                                      "$gt": [
                                        {
                                          "$len": {
                                            "$sortByOrder": {
                                              "$concatArrays": [
                                                {
                                                  "$filter": {
                                                    "from": {
                                                      "$sortByOrder": {
                                                        "$ref": "request.images"
                                                      }
                                                    },
                                                    "as": "media",
                                                    "where": {
                                                      "$in": [
                                                        {
                                                          "$ref": "media.role"
                                                        },
                                                        [
                                                          "reference_image",
                                                          "subject_reference",
                                                          "style_reference"
                                                        ]
                                                      ]
                                                    }
                                                  }
                                                },
                                                {
                                                  "$filter": {
                                                    "from": {
                                                      "$filter": {
                                                        "from": {
                                                          "$sortByOrder": {
                                                            "$ref": "request.images"
                                                          }
                                                        },
                                                        "as": "media",
                                                        "where": {
                                                          "$in": [
                                                            {
                                                              "$ref": "media.role"
                                                            },
                                                            [
                                                              ""
                                                            ]
                                                          ]
                                                        }
                                                      }
                                                    },
                                                    "as": "candidate",
                                                    "where": {
                                                      "$or": [
                                                        {
                                                          "$not": {
                                                            "$and": [
                                                              {
                                                                "$eq": [
                                                                  {
                                                                    "$len": {
                                                                      "$filter": {
                                                                        "from": {
                                                                          "$sortByOrder": {
                                                                            "$ref": "request.images"
                                                                          }
                                                                        },
                                                                        "as": "media",
                                                                        "where": {
                                                                          "$in": [
                                                                            {
                                                                              "$ref": "media.role"
                                                                            },
                                                                            [
                                                                              "first_frame"
                                                                            ]
                                                                          ]
                                                                        }
                                                                      }
                                                                    }
                                                                  },
                                                                  0
                                                                ]
                                                              },
                                                              {
                                                                "$or": [
                                                                  {
                                                                    "$eq": [
                                                                      {
                                                                        "$ref": "request.operation"
                                                                      },
                                                                      "image_to_video"
                                                                    ]
                                                                  },
                                                                  {
                                                                    "$and": [
                                                                      {
                                                                        "$eq": [
                                                                          {
                                                                            "$len": {
                                                                              "$filter": {
                                                                                "from": {
                                                                                  "$sortByOrder": {
                                                                                    "$ref": "request.images"
                                                                                  }
                                                                                },
                                                                                "as": "media",
                                                                                "where": {
                                                                                  "$in": [
                                                                                    {
                                                                                      "$ref": "media.role"
                                                                                    },
                                                                                    [
                                                                                      ""
                                                                                    ]
                                                                                  ]
                                                                                }
                                                                              }
                                                                            }
                                                                          },
                                                                          1
                                                                        ]
                                                                      },
                                                                      {
                                                                        "$eq": [
                                                                          {
                                                                            "$len": {
                                                                              "$filter": {
                                                                                "from": {
                                                                                  "$sortByOrder": {
                                                                                    "$ref": "request.images"
                                                                                  }
                                                                                },
                                                                                "as": "media",
                                                                                "where": {
                                                                                  "$in": [
                                                                                    {
                                                                                      "$ref": "media.role"
                                                                                    },
                                                                                    [
                                                                                      "reference_image",
                                                                                      "subject_reference",
                                                                                      "style_reference"
                                                                                    ]
                                                                                  ]
                                                                                }
                                                                              }
                                                                            }
                                                                          },
                                                                          0
                                                                        ]
                                                                      },
                                                                      {
                                                                        "$ne": [
                                                                          {
                                                                            "$ref": "request.operation"
                                                                          },
                                                                          "reference_to_video"
                                                                        ]
                                                                      }
                                                                    ]
                                                                  }
                                                                ]
                                                              }
                                                            ]
                                                          }
                                                        },
                                                        {
                                                          "$gt": [
                                                            {
                                                              "$ref": "candidateIndex"
                                                            },
                                                            0
                                                          ]
                                                        }
                                                      ]
                                                    }
                                                  }
                                                }
                                              ]
                                            }
                                          }
                                        },
                                        0
                                      ]
                                    },
                                    {
                                      "$gt": [
                                        {
                                          "$len": {
                                            "$filter": {
                                              "from": {
                                                "$sortByOrder": {
                                                  "$ref": "request.images"
                                                }
                                              },
                                              "as": "media",
                                              "where": {
                                                "$in": [
                                                  {
                                                    "$ref": "media.role"
                                                  },
                                                  [
                                                    "last_frame"
                                                  ]
                                                ]
                                              }
                                            }
                                          }
                                        },
                                        0
                                      ]
                                    },
                                    {
                                      "$gt": [
                                        {
                                          "$len": {
                                            "$ref": "request.providerOptions.xai-video.reference_audios"
                                          }
                                        },
                                        0
                                      ]
                                    },
                                    {
                                      "$gt": [
                                        {
                                          "$len": {
                                            "$ref": "request.providerOptions.xai-video.keyframes"
                                          }
                                        },
                                        0
                                      ]
                                    }
                                  ]
                                }
                              ]
                            },
                            "then": 10,
                            "else": 15
                          }
                        }
                      ]
                    }
                  ]
                }
              ]
            },
            "message": "xAI 视频生成时长为 1–15 秒，旧版参考图模式最多 10 秒"
          },
          {
            "assert": {
              "$or": [
                {
                  "$or": [
                    {
                      "$gt": [
                        {
                          "$len": {
                            "$ref": "request.videos"
                          }
                        },
                        0
                      ]
                    },
                    {
                      "$eq": [
                        {
                          "$ref": "request.operation"
                        },
                        "extend"
                      ]
                    },
                    {
                      "$in": [
                        {
                          "$ref": "request.operation"
                        },
                        [
                          "edit",
                          "video_to_video",
                          "inpaint",
                          "replace_element",
                          "camera_motion",
                          "style_transfer"
                        ]
                      ]
                    }
                  ]
                },
                {
                  "$in": [
                    {
                      "$coalesce": [
                        {
                          "$ref": "request.resolution"
                        },
                        "720p"
                      ]
                    },
                    {
                      "$if": {
                        "condition": {
                          "$eq": [
                            {
                              "$coalesce": [
                                {
                                  "$ref": "request.providerOptions.xai-video.modelProfile"
                                },
                                {
                                  "$if": {
                                    "condition": {
                                      "$eq": [
                                        {
                                          "$ref": "request.model"
                                        },
                                        "grok-imagine-video-1.5"
                                      ]
                                    },
                                    "then": "video-1.5",
                                    "else": {
                                      "$if": {
                                        "condition": {
                                          "$eq": [
                                            {
                                              "$ref": "request.model"
                                            },
                                            "grok-imagine-video-1.5-lite"
                                          ]
                                        },
                                        "then": "video-1.5-lite",
                                        "else": "video"
                                      }
                                    }
                                  }
                                }
                              ]
                            },
                            "video"
                          ]
                        },
                        "then": [
                          "480p",
                          "720p"
                        ],
                        "else": [
                          "480p",
                          "720p",
                          "1080p"
                        ]
                      }
                    }
                  ]
                }
              ]
            },
            "message": "所选 xAI 模型不支持该清晰度；旧版不支持 1080p"
          },
          {
            "assert": {
              "$or": [
                {
                  "$or": [
                    {
                      "$gt": [
                        {
                          "$len": {
                            "$ref": "request.videos"
                          }
                        },
                        0
                      ]
                    },
                    {
                      "$eq": [
                        {
                          "$ref": "request.operation"
                        },
                        "extend"
                      ]
                    },
                    {
                      "$in": [
                        {
                          "$ref": "request.operation"
                        },
                        [
                          "edit",
                          "video_to_video",
                          "inpaint",
                          "replace_element",
                          "camera_motion",
                          "style_transfer"
                        ]
                      ]
                    }
                  ]
                },
                {
                  "$not": {
                    "$or": [
                      {
                        "$gt": [
                          {
                            "$len": {
                              "$sortByOrder": {
                                "$concatArrays": [
                                  {
                                    "$filter": {
                                      "from": {
                                        "$sortByOrder": {
                                          "$ref": "request.images"
                                        }
                                      },
                                      "as": "media",
                                      "where": {
                                        "$in": [
                                          {
                                            "$ref": "media.role"
                                          },
                                          [
                                            "reference_image",
                                            "subject_reference",
                                            "style_reference"
                                          ]
                                        ]
                                      }
                                    }
                                  },
                                  {
                                    "$filter": {
                                      "from": {
                                        "$filter": {
                                          "from": {
                                            "$sortByOrder": {
                                              "$ref": "request.images"
                                            }
                                          },
                                          "as": "media",
                                          "where": {
                                            "$in": [
                                              {
                                                "$ref": "media.role"
                                              },
                                              [
                                                ""
                                              ]
                                            ]
                                          }
                                        }
                                      },
                                      "as": "candidate",
                                      "where": {
                                        "$or": [
                                          {
                                            "$not": {
                                              "$and": [
                                                {
                                                  "$eq": [
                                                    {
                                                      "$len": {
                                                        "$filter": {
                                                          "from": {
                                                            "$sortByOrder": {
                                                              "$ref": "request.images"
                                                            }
                                                          },
                                                          "as": "media",
                                                          "where": {
                                                            "$in": [
                                                              {
                                                                "$ref": "media.role"
                                                              },
                                                              [
                                                                "first_frame"
                                                              ]
                                                            ]
                                                          }
                                                        }
                                                      }
                                                    },
                                                    0
                                                  ]
                                                },
                                                {
                                                  "$or": [
                                                    {
                                                      "$eq": [
                                                        {
                                                          "$ref": "request.operation"
                                                        },
                                                        "image_to_video"
                                                      ]
                                                    },
                                                    {
                                                      "$and": [
                                                        {
                                                          "$eq": [
                                                            {
                                                              "$len": {
                                                                "$filter": {
                                                                  "from": {
                                                                    "$sortByOrder": {
                                                                      "$ref": "request.images"
                                                                    }
                                                                  },
                                                                  "as": "media",
                                                                  "where": {
                                                                    "$in": [
                                                                      {
                                                                        "$ref": "media.role"
                                                                      },
                                                                      [
                                                                        ""
                                                                      ]
                                                                    ]
                                                                  }
                                                                }
                                                              }
                                                            },
                                                            1
                                                          ]
                                                        },
                                                        {
                                                          "$eq": [
                                                            {
                                                              "$len": {
                                                                "$filter": {
                                                                  "from": {
                                                                    "$sortByOrder": {
                                                                      "$ref": "request.images"
                                                                    }
                                                                  },
                                                                  "as": "media",
                                                                  "where": {
                                                                    "$in": [
                                                                      {
                                                                        "$ref": "media.role"
                                                                      },
                                                                      [
                                                                        "reference_image",
                                                                        "subject_reference",
                                                                        "style_reference"
                                                                      ]
                                                                    ]
                                                                  }
                                                                }
                                                              }
                                                            },
                                                            0
                                                          ]
                                                        },
                                                        {
                                                          "$ne": [
                                                            {
                                                              "$ref": "request.operation"
                                                            },
                                                            "reference_to_video"
                                                          ]
                                                        }
                                                      ]
                                                    }
                                                  ]
                                                }
                                              ]
                                            }
                                          },
                                          {
                                            "$gt": [
                                              {
                                                "$ref": "candidateIndex"
                                              },
                                              0
                                            ]
                                          }
                                        ]
                                      }
                                    }
                                  }
                                ]
                              }
                            }
                          },
                          0
                        ]
                      },
                      {
                        "$gt": [
                          {
                            "$len": {
                              "$filter": {
                                "from": {
                                  "$sortByOrder": {
                                    "$ref": "request.images"
                                  }
                                },
                                "as": "media",
                                "where": {
                                  "$in": [
                                    {
                                      "$ref": "media.role"
                                    },
                                    [
                                      "last_frame"
                                    ]
                                  ]
                                }
                              }
                            }
                          },
                          0
                        ]
                      },
                      {
                        "$gt": [
                          {
                            "$len": {
                              "$ref": "request.providerOptions.xai-video.reference_audios"
                            }
                          },
                          0
                        ]
                      },
                      {
                        "$gt": [
                          {
                            "$len": {
                              "$ref": "request.providerOptions.xai-video.keyframes"
                            }
                          },
                          0
                        ]
                      }
                    ]
                  }
                },
                {
                  "$ne": [
                    {
                      "$coalesce": [
                        {
                          "$ref": "request.resolution"
                        },
                        "720p"
                      ]
                    },
                    "1080p"
                  ]
                }
              ]
            },
            "message": "参考图、音色、尾帧或关键帧模式最高 720p，请调整清晰度"
          },
          {
            "assert": {
              "$or": [
                {
                  "$or": [
                    {
                      "$gt": [
                        {
                          "$len": {
                            "$ref": "request.videos"
                          }
                        },
                        0
                      ]
                    },
                    {
                      "$eq": [
                        {
                          "$ref": "request.operation"
                        },
                        "extend"
                      ]
                    },
                    {
                      "$in": [
                        {
                          "$ref": "request.operation"
                        },
                        [
                          "edit",
                          "video_to_video",
                          "inpaint",
                          "replace_element",
                          "camera_motion",
                          "style_transfer"
                        ]
                      ]
                    }
                  ]
                },
                {
                  "$not": {
                    "$ref": "request.aspectRatio"
                  }
                },
                {
                  "$in": [
                    {
                      "$ref": "request.aspectRatio"
                    },
                    [
                      "1:1",
                      "16:9",
                      "9:16",
                      "4:3",
                      "3:4",
                      "3:2",
                      "2:3",
                      "21:9",
                      "5:2"
                    ]
                  ]
                }
              ]
            },
            "message": "xAI Video 不支持所选画幅"
          },
          {
            "assert": {
              "$or": [
                {
                  "$not": {
                    "$or": [
                      {
                        "$gt": [
                          {
                            "$len": {
                              "$ref": "request.videos"
                            }
                          },
                          0
                        ]
                      },
                      {
                        "$eq": [
                          {
                            "$ref": "request.operation"
                          },
                          "extend"
                        ]
                      },
                      {
                        "$in": [
                          {
                            "$ref": "request.operation"
                          },
                          [
                            "edit",
                            "video_to_video",
                            "inpaint",
                            "replace_element",
                            "camera_motion",
                            "style_transfer"
                          ]
                        ]
                      }
                    ]
                  }
                },
                {
                  "$eq": [
                    {
                      "$coalesce": [
                        {
                          "$ref": "request.providerOptions.xai-video.modelProfile"
                        },
                        {
                          "$if": {
                            "condition": {
                              "$eq": [
                                {
                                  "$ref": "request.model"
                                },
                                "grok-imagine-video-1.5"
                              ]
                            },
                            "then": "video-1.5",
                            "else": {
                              "$if": {
                                "condition": {
                                  "$eq": [
                                    {
                                      "$ref": "request.model"
                                    },
                                    "grok-imagine-video-1.5-lite"
                                  ]
                                },
                                "then": "video-1.5-lite",
                                "else": "video"
                              }
                            }
                          }
                        }
                      ]
                    },
                    "video"
                  ]
                }
              ]
            },
            "message": "视频编辑和延长仅旧版 grok-imagine-video 支持，Video 1.5 不支持"
          },
          {
            "assert": {
              "$or": [
                {
                  "$not": {
                    "$or": [
                      {
                        "$gt": [
                          {
                            "$len": {
                              "$ref": "request.videos"
                            }
                          },
                          0
                        ]
                      },
                      {
                        "$eq": [
                          {
                            "$ref": "request.operation"
                          },
                          "extend"
                        ]
                      },
                      {
                        "$in": [
                          {
                            "$ref": "request.operation"
                          },
                          [
                            "edit",
                            "video_to_video",
                            "inpaint",
                            "replace_element",
                            "camera_motion",
                            "style_transfer"
                          ]
                        ]
                      }
                    ]
                  }
                },
                {
                  "$eq": [
                    {
                      "$len": {
                        "$ref": "request.videos"
                      }
                    },
                    1
                  ]
                }
              ]
            },
            "message": "编辑或延长需且仅需一个源视频"
          },
          {
            "assert": {
              "$or": [
                {
                  "$not": {
                    "$or": [
                      {
                        "$gt": [
                          {
                            "$len": {
                              "$ref": "request.videos"
                            }
                          },
                          0
                        ]
                      },
                      {
                        "$eq": [
                          {
                            "$ref": "request.operation"
                          },
                          "extend"
                        ]
                      },
                      {
                        "$in": [
                          {
                            "$ref": "request.operation"
                          },
                          [
                            "edit",
                            "video_to_video",
                            "inpaint",
                            "replace_element",
                            "camera_motion",
                            "style_transfer"
                          ]
                        ]
                      }
                    ]
                  }
                },
                {
                  "$gt": [
                    {
                      "$len": {
                        "$trim": {
                          "$first": {
                            "$map": {
                              "from": {
                                "$ref": "request.videos"
                              },
                              "as": "media",
                              "in": {
                                "$ref": "media.value"
                              }
                            }
                          }
                        }
                      }
                    },
                    0
                  ]
                }
              ]
            },
            "message": "xAI 源视频输入不能为空"
          },
          {
            "assert": {
              "$or": [
                {
                  "$not": {
                    "$or": [
                      {
                        "$gt": [
                          {
                            "$len": {
                              "$ref": "request.videos"
                            }
                          },
                          0
                        ]
                      },
                      {
                        "$eq": [
                          {
                            "$ref": "request.operation"
                          },
                          "extend"
                        ]
                      },
                      {
                        "$in": [
                          {
                            "$ref": "request.operation"
                          },
                          [
                            "edit",
                            "video_to_video",
                            "inpaint",
                            "replace_element",
                            "camera_motion",
                            "style_transfer"
                          ]
                        ]
                      }
                    ]
                  }
                },
                {
                  "$and": [
                    {
                      "$eq": [
                        {
                          "$len": {
                            "$ref": "request.images"
                          }
                        },
                        0
                      ]
                    },
                    {
                      "$eq": [
                        {
                          "$len": {
                            "$ref": "request.providerOptions.xai-video.reference_audios"
                          }
                        },
                        0
                      ]
                    },
                    {
                      "$eq": [
                        {
                          "$len": {
                            "$ref": "request.providerOptions.xai-video.keyframes"
                          }
                        },
                        0
                      ]
                    }
                  ]
                }
              ]
            },
            "message": "视频编辑和延长不能混入图片、音色或关键帧"
          },
          {
            "assert": {
              "$or": [
                {
                  "$not": {
                    "$or": [
                      {
                        "$gt": [
                          {
                            "$len": {
                              "$ref": "request.videos"
                            }
                          },
                          0
                        ]
                      },
                      {
                        "$eq": [
                          {
                            "$ref": "request.operation"
                          },
                          "extend"
                        ]
                      },
                      {
                        "$in": [
                          {
                            "$ref": "request.operation"
                          },
                          [
                            "edit",
                            "video_to_video",
                            "inpaint",
                            "replace_element",
                            "camera_motion",
                            "style_transfer"
                          ]
                        ]
                      }
                    ]
                  }
                },
                {
                  "$not": {
                    "$ref": "request.operation"
                  }
                },
                {
                  "$in": [
                    {
                      "$ref": "request.operation"
                    },
                    [
                      "extend",
                      "edit",
                      "video_to_video",
                      "inpaint",
                      "replace_element",
                      "camera_motion",
                      "style_transfer"
                    ]
                  ]
                }
              ]
            },
            "message": "源视频需选择编辑或延长操作"
          },
          {
            "assert": {
              "$or": [
                {
                  "$not": {
                    "$eq": [
                      {
                        "$ref": "request.operation"
                      },
                      "extend"
                    ]
                  }
                },
                {
                  "$and": [
                    {
                      "$gte": [
                        {
                          "$if": {
                            "condition": {
                              "$ne": [
                                {
                                  "$ref": "request.duration"
                                },
                                0
                              ]
                            },
                            "then": {
                              "$ref": "request.duration"
                            },
                            "else": {
                              "$if": {
                                "condition": {
                                  "$eq": [
                                    {
                                      "$ref": "request.operation"
                                    },
                                    "extend"
                                  ]
                                },
                                "then": 6,
                                "else": 8
                              }
                            }
                          }
                        },
                        2
                      ]
                    },
                    {
                      "$lte": [
                        {
                          "$if": {
                            "condition": {
                              "$ne": [
                                {
                                  "$ref": "request.duration"
                                },
                                0
                              ]
                            },
                            "then": {
                              "$ref": "request.duration"
                            },
                            "else": {
                              "$if": {
                                "condition": {
                                  "$eq": [
                                    {
                                      "$ref": "request.operation"
                                    },
                                    "extend"
                                  ]
                                },
                                "then": 6,
                                "else": 8
                              }
                            }
                          }
                        },
                        10
                      ]
                    }
                  ]
                }
              ]
            },
            "message": "视频延长的新增时长为 2–10 秒，不是总时长"
          },
          {
            "assert": {
              "$or": [
                {
                  "$not": {
                    "$or": [
                      {
                        "$gt": [
                          {
                            "$len": {
                              "$ref": "request.videos"
                            }
                          },
                          0
                        ]
                      },
                      {
                        "$eq": [
                          {
                            "$ref": "request.operation"
                          },
                          "extend"
                        ]
                      },
                      {
                        "$in": [
                          {
                            "$ref": "request.operation"
                          },
                          [
                            "edit",
                            "video_to_video",
                            "inpaint",
                            "replace_element",
                            "camera_motion",
                            "style_transfer"
                          ]
                        ]
                      }
                    ]
                  }
                },
                {
                  "$not": {
                    "$ref": "request.videos.0.metadata.durationMs"
                  }
                },
                {
                  "$if": {
                    "condition": {
                      "$eq": [
                        {
                          "$ref": "request.operation"
                        },
                        "extend"
                      ]
                    },
                    "then": {
                      "$and": [
                        {
                          "$gte": [
                            {
                              "$ref": "request.videos.0.metadata.durationMs"
                            },
                            2000
                          ]
                        },
                        {
                          "$lte": [
                            {
                              "$ref": "request.videos.0.metadata.durationMs"
                            },
                            15000
                          ]
                        }
                      ]
                    },
                    "else": {
                      "$lte": [
                        {
                          "$ref": "request.videos.0.metadata.durationMs"
                        },
                        8700
                      ]
                    }
                  }
                }
              ]
            },
            "message": "编辑源视频最多 8.7 秒，延长源视频需 2–15 秒"
          },
          {
            "assert": {
              "$or": [
                {
                  "$gt": [
                    {
                      "$len": {
                        "$trim": {
                          "$ref": "request.prompt"
                        }
                      }
                    },
                    0
                  ]
                },
                {
                  "$and": [
                    {
                      "$not": {
                        "$or": [
                          {
                            "$gt": [
                              {
                                "$len": {
                                  "$ref": "request.videos"
                                }
                              },
                              0
                            ]
                          },
                          {
                            "$eq": [
                              {
                                "$ref": "request.operation"
                              },
                              "extend"
                            ]
                          },
                          {
                            "$in": [
                              {
                                "$ref": "request.operation"
                              },
                              [
                                "edit",
                                "video_to_video",
                                "inpaint",
                                "replace_element",
                                "camera_motion",
                                "style_transfer"
                              ]
                            ]
                          }
                        ]
                      }
                    },
                    {
                      "$or": [
                        {
                          "$coalesce": [
                            {
                              "$first": {
                                "$filter": {
                                  "from": {
                                    "$sortByOrder": {
                                      "$ref": "request.images"
                                    }
                                  },
                                  "as": "media",
                                  "where": {
                                    "$in": [
                                      {
                                        "$ref": "media.role"
                                      },
                                      [
                                        "first_frame"
                                      ]
                                    ]
                                  }
                                }
                              }
                            },
                            {
                              "$if": {
                                "condition": {
                                  "$and": [
                                    {
                                      "$eq": [
                                        {
                                          "$len": {
                                            "$filter": {
                                              "from": {
                                                "$sortByOrder": {
                                                  "$ref": "request.images"
                                                }
                                              },
                                              "as": "media",
                                              "where": {
                                                "$in": [
                                                  {
                                                    "$ref": "media.role"
                                                  },
                                                  [
                                                    "first_frame"
                                                  ]
                                                ]
                                              }
                                            }
                                          }
                                        },
                                        0
                                      ]
                                    },
                                    {
                                      "$or": [
                                        {
                                          "$eq": [
                                            {
                                              "$ref": "request.operation"
                                            },
                                            "image_to_video"
                                          ]
                                        },
                                        {
                                          "$and": [
                                            {
                                              "$eq": [
                                                {
                                                  "$len": {
                                                    "$filter": {
                                                      "from": {
                                                        "$sortByOrder": {
                                                          "$ref": "request.images"
                                                        }
                                                      },
                                                      "as": "media",
                                                      "where": {
                                                        "$in": [
                                                          {
                                                            "$ref": "media.role"
                                                          },
                                                          [
                                                            ""
                                                          ]
                                                        ]
                                                      }
                                                    }
                                                  }
                                                },
                                                1
                                              ]
                                            },
                                            {
                                              "$eq": [
                                                {
                                                  "$len": {
                                                    "$filter": {
                                                      "from": {
                                                        "$sortByOrder": {
                                                          "$ref": "request.images"
                                                        }
                                                      },
                                                      "as": "media",
                                                      "where": {
                                                        "$in": [
                                                          {
                                                            "$ref": "media.role"
                                                          },
                                                          [
                                                            "reference_image",
                                                            "subject_reference",
                                                            "style_reference"
                                                          ]
                                                        ]
                                                      }
                                                    }
                                                  }
                                                },
                                                0
                                              ]
                                            },
                                            {
                                              "$ne": [
                                                {
                                                  "$ref": "request.operation"
                                                },
                                                "reference_to_video"
                                              ]
                                            }
                                          ]
                                        }
                                      ]
                                    }
                                  ]
                                },
                                "then": {
                                  "$first": {
                                    "$filter": {
                                      "from": {
                                        "$sortByOrder": {
                                          "$ref": "request.images"
                                        }
                                      },
                                      "as": "media",
                                      "where": {
                                        "$in": [
                                          {
                                            "$ref": "media.role"
                                          },
                                          [
                                            ""
                                          ]
                                        ]
                                      }
                                    }
                                  }
                                },
                                "else": null
                              }
                            }
                          ]
                        },
                        {
                          "$gt": [
                            {
                              "$len": {
                                "$filter": {
                                  "from": {
                                    "$sortByOrder": {
                                      "$ref": "request.images"
                                    }
                                  },
                                  "as": "media",
                                  "where": {
                                    "$in": [
                                      {
                                        "$ref": "media.role"
                                      },
                                      [
                                        "last_frame"
                                      ]
                                    ]
                                  }
                                }
                              }
                            },
                            0
                          ]
                        },
                        {
                          "$gt": [
                            {
                              "$len": {
                                "$ref": "request.providerOptions.xai-video.keyframes"
                              }
                            },
                            0
                          ]
                        }
                      ]
                    }
                  ]
                }
              ]
            },
            "message": "请填写视频提示词，或提供首帧、尾帧或关键帧"
          }
        ],
        "create": {
          "method": "POST",
          "path": "/v1/videos/generations",
          "contentType": "application/json",
          "body": {
            "model": {
              "$ref": "request.model"
            },
            "prompt": {
              "$omitEmpty": {
                "$trim": {
                  "$ref": "request.prompt"
                }
              }
            },
            "duration": {
              "$if": {
                "condition": {
                  "$or": [
                    {
                      "$not": {
                        "$or": [
                          {
                            "$gt": [
                              {
                                "$len": {
                                  "$ref": "request.videos"
                                }
                              },
                              0
                            ]
                          },
                          {
                            "$eq": [
                              {
                                "$ref": "request.operation"
                              },
                              "extend"
                            ]
                          },
                          {
                            "$in": [
                              {
                                "$ref": "request.operation"
                              },
                              [
                                "edit",
                                "video_to_video",
                                "inpaint",
                                "replace_element",
                                "camera_motion",
                                "style_transfer"
                              ]
                            ]
                          }
                        ]
                      }
                    },
                    {
                      "$eq": [
                        {
                          "$ref": "request.operation"
                        },
                        "extend"
                      ]
                    }
                  ]
                },
                "then": {
                  "$if": {
                    "condition": {
                      "$ne": [
                        {
                          "$ref": "request.duration"
                        },
                        0
                      ]
                    },
                    "then": {
                      "$ref": "request.duration"
                    },
                    "else": {
                      "$if": {
                        "condition": {
                          "$eq": [
                            {
                              "$ref": "request.operation"
                            },
                            "extend"
                          ]
                        },
                        "then": 6,
                        "else": 8
                      }
                    }
                  }
                },
                "else": null
              }
            },
            "aspect_ratio": {
              "$if": {
                "condition": {
                  "$and": [
                    {
                      "$not": {
                        "$or": [
                          {
                            "$gt": [
                              {
                                "$len": {
                                  "$ref": "request.videos"
                                }
                              },
                              0
                            ]
                          },
                          {
                            "$eq": [
                              {
                                "$ref": "request.operation"
                              },
                              "extend"
                            ]
                          },
                          {
                            "$in": [
                              {
                                "$ref": "request.operation"
                              },
                              [
                                "edit",
                                "video_to_video",
                                "inpaint",
                                "replace_element",
                                "camera_motion",
                                "style_transfer"
                              ]
                            ]
                          }
                        ]
                      }
                    },
                    {
                      "$or": [
                        {
                          "$not": {
                            "$coalesce": [
                              {
                                "$first": {
                                  "$filter": {
                                    "from": {
                                      "$sortByOrder": {
                                        "$ref": "request.images"
                                      }
                                    },
                                    "as": "media",
                                    "where": {
                                      "$in": [
                                        {
                                          "$ref": "media.role"
                                        },
                                        [
                                          "first_frame"
                                        ]
                                      ]
                                    }
                                  }
                                }
                              },
                              {
                                "$if": {
                                  "condition": {
                                    "$and": [
                                      {
                                        "$eq": [
                                          {
                                            "$len": {
                                              "$filter": {
                                                "from": {
                                                  "$sortByOrder": {
                                                    "$ref": "request.images"
                                                  }
                                                },
                                                "as": "media",
                                                "where": {
                                                  "$in": [
                                                    {
                                                      "$ref": "media.role"
                                                    },
                                                    [
                                                      "first_frame"
                                                    ]
                                                  ]
                                                }
                                              }
                                            }
                                          },
                                          0
                                        ]
                                      },
                                      {
                                        "$or": [
                                          {
                                            "$eq": [
                                              {
                                                "$ref": "request.operation"
                                              },
                                              "image_to_video"
                                            ]
                                          },
                                          {
                                            "$and": [
                                              {
                                                "$eq": [
                                                  {
                                                    "$len": {
                                                      "$filter": {
                                                        "from": {
                                                          "$sortByOrder": {
                                                            "$ref": "request.images"
                                                          }
                                                        },
                                                        "as": "media",
                                                        "where": {
                                                          "$in": [
                                                            {
                                                              "$ref": "media.role"
                                                            },
                                                            [
                                                              ""
                                                            ]
                                                          ]
                                                        }
                                                      }
                                                    }
                                                  },
                                                  1
                                                ]
                                              },
                                              {
                                                "$eq": [
                                                  {
                                                    "$len": {
                                                      "$filter": {
                                                        "from": {
                                                          "$sortByOrder": {
                                                            "$ref": "request.images"
                                                          }
                                                        },
                                                        "as": "media",
                                                        "where": {
                                                          "$in": [
                                                            {
                                                              "$ref": "media.role"
                                                            },
                                                            [
                                                              "reference_image",
                                                              "subject_reference",
                                                              "style_reference"
                                                            ]
                                                          ]
                                                        }
                                                      }
                                                    }
                                                  },
                                                  0
                                                ]
                                              },
                                              {
                                                "$ne": [
                                                  {
                                                    "$ref": "request.operation"
                                                  },
                                                  "reference_to_video"
                                                ]
                                              }
                                            ]
                                          }
                                        ]
                                      }
                                    ]
                                  },
                                  "then": {
                                    "$first": {
                                      "$filter": {
                                        "from": {
                                          "$sortByOrder": {
                                            "$ref": "request.images"
                                          }
                                        },
                                        "as": "media",
                                        "where": {
                                          "$in": [
                                            {
                                              "$ref": "media.role"
                                            },
                                            [
                                              ""
                                            ]
                                          ]
                                        }
                                      }
                                    }
                                  },
                                  "else": null
                                }
                              }
                            ]
                          }
                        },
                        {
                          "$or": [
                            {
                              "$gt": [
                                {
                                  "$len": {
                                    "$sortByOrder": {
                                      "$concatArrays": [
                                        {
                                          "$filter": {
                                            "from": {
                                              "$sortByOrder": {
                                                "$ref": "request.images"
                                              }
                                            },
                                            "as": "media",
                                            "where": {
                                              "$in": [
                                                {
                                                  "$ref": "media.role"
                                                },
                                                [
                                                  "reference_image",
                                                  "subject_reference",
                                                  "style_reference"
                                                ]
                                              ]
                                            }
                                          }
                                        },
                                        {
                                          "$filter": {
                                            "from": {
                                              "$filter": {
                                                "from": {
                                                  "$sortByOrder": {
                                                    "$ref": "request.images"
                                                  }
                                                },
                                                "as": "media",
                                                "where": {
                                                  "$in": [
                                                    {
                                                      "$ref": "media.role"
                                                    },
                                                    [
                                                      ""
                                                    ]
                                                  ]
                                                }
                                              }
                                            },
                                            "as": "candidate",
                                            "where": {
                                              "$or": [
                                                {
                                                  "$not": {
                                                    "$and": [
                                                      {
                                                        "$eq": [
                                                          {
                                                            "$len": {
                                                              "$filter": {
                                                                "from": {
                                                                  "$sortByOrder": {
                                                                    "$ref": "request.images"
                                                                  }
                                                                },
                                                                "as": "media",
                                                                "where": {
                                                                  "$in": [
                                                                    {
                                                                      "$ref": "media.role"
                                                                    },
                                                                    [
                                                                      "first_frame"
                                                                    ]
                                                                  ]
                                                                }
                                                              }
                                                            }
                                                          },
                                                          0
                                                        ]
                                                      },
                                                      {
                                                        "$or": [
                                                          {
                                                            "$eq": [
                                                              {
                                                                "$ref": "request.operation"
                                                              },
                                                              "image_to_video"
                                                            ]
                                                          },
                                                          {
                                                            "$and": [
                                                              {
                                                                "$eq": [
                                                                  {
                                                                    "$len": {
                                                                      "$filter": {
                                                                        "from": {
                                                                          "$sortByOrder": {
                                                                            "$ref": "request.images"
                                                                          }
                                                                        },
                                                                        "as": "media",
                                                                        "where": {
                                                                          "$in": [
                                                                            {
                                                                              "$ref": "media.role"
                                                                            },
                                                                            [
                                                                              ""
                                                                            ]
                                                                          ]
                                                                        }
                                                                      }
                                                                    }
                                                                  },
                                                                  1
                                                                ]
                                                              },
                                                              {
                                                                "$eq": [
                                                                  {
                                                                    "$len": {
                                                                      "$filter": {
                                                                        "from": {
                                                                          "$sortByOrder": {
                                                                            "$ref": "request.images"
                                                                          }
                                                                        },
                                                                        "as": "media",
                                                                        "where": {
                                                                          "$in": [
                                                                            {
                                                                              "$ref": "media.role"
                                                                            },
                                                                            [
                                                                              "reference_image",
                                                                              "subject_reference",
                                                                              "style_reference"
                                                                            ]
                                                                          ]
                                                                        }
                                                                      }
                                                                    }
                                                                  },
                                                                  0
                                                                ]
                                                              },
                                                              {
                                                                "$ne": [
                                                                  {
                                                                    "$ref": "request.operation"
                                                                  },
                                                                  "reference_to_video"
                                                                ]
                                                              }
                                                            ]
                                                          }
                                                        ]
                                                      }
                                                    ]
                                                  }
                                                },
                                                {
                                                  "$gt": [
                                                    {
                                                      "$ref": "candidateIndex"
                                                    },
                                                    0
                                                  ]
                                                }
                                              ]
                                            }
                                          }
                                        }
                                      ]
                                    }
                                  }
                                },
                                0
                              ]
                            },
                            {
                              "$gt": [
                                {
                                  "$len": {
                                    "$filter": {
                                      "from": {
                                        "$sortByOrder": {
                                          "$ref": "request.images"
                                        }
                                      },
                                      "as": "media",
                                      "where": {
                                        "$in": [
                                          {
                                            "$ref": "media.role"
                                          },
                                          [
                                            "last_frame"
                                          ]
                                        ]
                                      }
                                    }
                                  }
                                },
                                0
                              ]
                            },
                            {
                              "$gt": [
                                {
                                  "$len": {
                                    "$ref": "request.providerOptions.xai-video.reference_audios"
                                  }
                                },
                                0
                              ]
                            },
                            {
                              "$gt": [
                                {
                                  "$len": {
                                    "$ref": "request.providerOptions.xai-video.keyframes"
                                  }
                                },
                                0
                              ]
                            }
                          ]
                        }
                      ]
                    }
                  ]
                },
                "then": {
                  "$coalesce": [
                    {
                      "$ref": "request.aspectRatio"
                    },
                    "16:9"
                  ]
                },
                "else": null
              }
            },
            "resolution": {
              "$if": {
                "condition": {
                  "$not": {
                    "$or": [
                      {
                        "$gt": [
                          {
                            "$len": {
                              "$ref": "request.videos"
                            }
                          },
                          0
                        ]
                      },
                      {
                        "$eq": [
                          {
                            "$ref": "request.operation"
                          },
                          "extend"
                        ]
                      },
                      {
                        "$in": [
                          {
                            "$ref": "request.operation"
                          },
                          [
                            "edit",
                            "video_to_video",
                            "inpaint",
                            "replace_element",
                            "camera_motion",
                            "style_transfer"
                          ]
                        ]
                      }
                    ]
                  }
                },
                "then": {
                  "$coalesce": [
                    {
                      "$ref": "request.resolution"
                    },
                    "720p"
                  ]
                },
                "else": null
              }
            },
            "image": {
              "$if": {
                "condition": {
                  "$coalesce": [
                    {
                      "$first": {
                        "$filter": {
                          "from": {
                            "$sortByOrder": {
                              "$ref": "request.images"
                            }
                          },
                          "as": "media",
                          "where": {
                            "$in": [
                              {
                                "$ref": "media.role"
                              },
                              [
                                "first_frame"
                              ]
                            ]
                          }
                        }
                      }
                    },
                    {
                      "$if": {
                        "condition": {
                          "$and": [
                            {
                              "$eq": [
                                {
                                  "$len": {
                                    "$filter": {
                                      "from": {
                                        "$sortByOrder": {
                                          "$ref": "request.images"
                                        }
                                      },
                                      "as": "media",
                                      "where": {
                                        "$in": [
                                          {
                                            "$ref": "media.role"
                                          },
                                          [
                                            "first_frame"
                                          ]
                                        ]
                                      }
                                    }
                                  }
                                },
                                0
                              ]
                            },
                            {
                              "$or": [
                                {
                                  "$eq": [
                                    {
                                      "$ref": "request.operation"
                                    },
                                    "image_to_video"
                                  ]
                                },
                                {
                                  "$and": [
                                    {
                                      "$eq": [
                                        {
                                          "$len": {
                                            "$filter": {
                                              "from": {
                                                "$sortByOrder": {
                                                  "$ref": "request.images"
                                                }
                                              },
                                              "as": "media",
                                              "where": {
                                                "$in": [
                                                  {
                                                    "$ref": "media.role"
                                                  },
                                                  [
                                                    ""
                                                  ]
                                                ]
                                              }
                                            }
                                          }
                                        },
                                        1
                                      ]
                                    },
                                    {
                                      "$eq": [
                                        {
                                          "$len": {
                                            "$filter": {
                                              "from": {
                                                "$sortByOrder": {
                                                  "$ref": "request.images"
                                                }
                                              },
                                              "as": "media",
                                              "where": {
                                                "$in": [
                                                  {
                                                    "$ref": "media.role"
                                                  },
                                                  [
                                                    "reference_image",
                                                    "subject_reference",
                                                    "style_reference"
                                                  ]
                                                ]
                                              }
                                            }
                                          }
                                        },
                                        0
                                      ]
                                    },
                                    {
                                      "$ne": [
                                        {
                                          "$ref": "request.operation"
                                        },
                                        "reference_to_video"
                                      ]
                                    }
                                  ]
                                }
                              ]
                            }
                          ]
                        },
                        "then": {
                          "$first": {
                            "$filter": {
                              "from": {
                                "$sortByOrder": {
                                  "$ref": "request.images"
                                }
                              },
                              "as": "media",
                              "where": {
                                "$in": [
                                  {
                                    "$ref": "media.role"
                                  },
                                  [
                                    ""
                                  ]
                                ]
                              }
                            }
                          }
                        },
                        "else": null
                      }
                    }
                  ]
                },
                "then": {
                  "url": {
                    "$first": {
                      "$map": {
                        "from": [
                          {
                            "$coalesce": [
                              {
                                "$first": {
                                  "$filter": {
                                    "from": {
                                      "$sortByOrder": {
                                        "$ref": "request.images"
                                      }
                                    },
                                    "as": "media",
                                    "where": {
                                      "$in": [
                                        {
                                          "$ref": "media.role"
                                        },
                                        [
                                          "first_frame"
                                        ]
                                      ]
                                    }
                                  }
                                }
                              },
                              {
                                "$if": {
                                  "condition": {
                                    "$and": [
                                      {
                                        "$eq": [
                                          {
                                            "$len": {
                                              "$filter": {
                                                "from": {
                                                  "$sortByOrder": {
                                                    "$ref": "request.images"
                                                  }
                                                },
                                                "as": "media",
                                                "where": {
                                                  "$in": [
                                                    {
                                                      "$ref": "media.role"
                                                    },
                                                    [
                                                      "first_frame"
                                                    ]
                                                  ]
                                                }
                                              }
                                            }
                                          },
                                          0
                                        ]
                                      },
                                      {
                                        "$or": [
                                          {
                                            "$eq": [
                                              {
                                                "$ref": "request.operation"
                                              },
                                              "image_to_video"
                                            ]
                                          },
                                          {
                                            "$and": [
                                              {
                                                "$eq": [
                                                  {
                                                    "$len": {
                                                      "$filter": {
                                                        "from": {
                                                          "$sortByOrder": {
                                                            "$ref": "request.images"
                                                          }
                                                        },
                                                        "as": "media",
                                                        "where": {
                                                          "$in": [
                                                            {
                                                              "$ref": "media.role"
                                                            },
                                                            [
                                                              ""
                                                            ]
                                                          ]
                                                        }
                                                      }
                                                    }
                                                  },
                                                  1
                                                ]
                                              },
                                              {
                                                "$eq": [
                                                  {
                                                    "$len": {
                                                      "$filter": {
                                                        "from": {
                                                          "$sortByOrder": {
                                                            "$ref": "request.images"
                                                          }
                                                        },
                                                        "as": "media",
                                                        "where": {
                                                          "$in": [
                                                            {
                                                              "$ref": "media.role"
                                                            },
                                                            [
                                                              "reference_image",
                                                              "subject_reference",
                                                              "style_reference"
                                                            ]
                                                          ]
                                                        }
                                                      }
                                                    }
                                                  },
                                                  0
                                                ]
                                              },
                                              {
                                                "$ne": [
                                                  {
                                                    "$ref": "request.operation"
                                                  },
                                                  "reference_to_video"
                                                ]
                                              }
                                            ]
                                          }
                                        ]
                                      }
                                    ]
                                  },
                                  "then": {
                                    "$first": {
                                      "$filter": {
                                        "from": {
                                          "$sortByOrder": {
                                            "$ref": "request.images"
                                          }
                                        },
                                        "as": "media",
                                        "where": {
                                          "$in": [
                                            {
                                              "$ref": "media.role"
                                            },
                                            [
                                              ""
                                            ]
                                          ]
                                        }
                                      }
                                    }
                                  },
                                  "else": null
                                }
                              }
                            ]
                          }
                        ],
                        "as": "image",
                        "in": {
                          "$ref": "image.value"
                        }
                      }
                    }
                  }
                },
                "else": null
              }
            },
            "last_frame": {
              "$if": {
                "condition": {
                  "$gt": [
                    {
                      "$len": {
                        "$filter": {
                          "from": {
                            "$sortByOrder": {
                              "$ref": "request.images"
                            }
                          },
                          "as": "media",
                          "where": {
                            "$in": [
                              {
                                "$ref": "media.role"
                              },
                              [
                                "last_frame"
                              ]
                            ]
                          }
                        }
                      }
                    },
                    0
                  ]
                },
                "then": {
                  "url": {
                    "$first": {
                      "$map": {
                        "from": {
                          "$filter": {
                            "from": {
                              "$sortByOrder": {
                                "$ref": "request.images"
                              }
                            },
                            "as": "media",
                            "where": {
                              "$in": [
                                {
                                  "$ref": "media.role"
                                },
                                [
                                  "last_frame"
                                ]
                              ]
                            }
                          }
                        },
                        "as": "media",
                        "in": {
                          "$ref": "media.value"
                        }
                      }
                    }
                  }
                },
                "else": null
              }
            },
            "reference_images": {
              "$omitEmpty": {
                "$map": {
                  "from": {
                    "$sortByOrder": {
                      "$concatArrays": [
                        {
                          "$filter": {
                            "from": {
                              "$sortByOrder": {
                                "$ref": "request.images"
                              }
                            },
                            "as": "media",
                            "where": {
                              "$in": [
                                {
                                  "$ref": "media.role"
                                },
                                [
                                  "reference_image",
                                  "subject_reference",
                                  "style_reference"
                                ]
                              ]
                            }
                          }
                        },
                        {
                          "$filter": {
                            "from": {
                              "$filter": {
                                "from": {
                                  "$sortByOrder": {
                                    "$ref": "request.images"
                                  }
                                },
                                "as": "media",
                                "where": {
                                  "$in": [
                                    {
                                      "$ref": "media.role"
                                    },
                                    [
                                      ""
                                    ]
                                  ]
                                }
                              }
                            },
                            "as": "candidate",
                            "where": {
                              "$or": [
                                {
                                  "$not": {
                                    "$and": [
                                      {
                                        "$eq": [
                                          {
                                            "$len": {
                                              "$filter": {
                                                "from": {
                                                  "$sortByOrder": {
                                                    "$ref": "request.images"
                                                  }
                                                },
                                                "as": "media",
                                                "where": {
                                                  "$in": [
                                                    {
                                                      "$ref": "media.role"
                                                    },
                                                    [
                                                      "first_frame"
                                                    ]
                                                  ]
                                                }
                                              }
                                            }
                                          },
                                          0
                                        ]
                                      },
                                      {
                                        "$or": [
                                          {
                                            "$eq": [
                                              {
                                                "$ref": "request.operation"
                                              },
                                              "image_to_video"
                                            ]
                                          },
                                          {
                                            "$and": [
                                              {
                                                "$eq": [
                                                  {
                                                    "$len": {
                                                      "$filter": {
                                                        "from": {
                                                          "$sortByOrder": {
                                                            "$ref": "request.images"
                                                          }
                                                        },
                                                        "as": "media",
                                                        "where": {
                                                          "$in": [
                                                            {
                                                              "$ref": "media.role"
                                                            },
                                                            [
                                                              ""
                                                            ]
                                                          ]
                                                        }
                                                      }
                                                    }
                                                  },
                                                  1
                                                ]
                                              },
                                              {
                                                "$eq": [
                                                  {
                                                    "$len": {
                                                      "$filter": {
                                                        "from": {
                                                          "$sortByOrder": {
                                                            "$ref": "request.images"
                                                          }
                                                        },
                                                        "as": "media",
                                                        "where": {
                                                          "$in": [
                                                            {
                                                              "$ref": "media.role"
                                                            },
                                                            [
                                                              "reference_image",
                                                              "subject_reference",
                                                              "style_reference"
                                                            ]
                                                          ]
                                                        }
                                                      }
                                                    }
                                                  },
                                                  0
                                                ]
                                              },
                                              {
                                                "$ne": [
                                                  {
                                                    "$ref": "request.operation"
                                                  },
                                                  "reference_to_video"
                                                ]
                                              }
                                            ]
                                          }
                                        ]
                                      }
                                    ]
                                  }
                                },
                                {
                                  "$gt": [
                                    {
                                      "$ref": "candidateIndex"
                                    },
                                    0
                                  ]
                                }
                              ]
                            }
                          }
                        }
                      ]
                    }
                  },
                  "as": "media",
                  "in": {
                    "url": {
                      "$ref": "media.value"
                    }
                  }
                }
              }
            },
            "reference_audios": {
              "$omitEmpty": {
                "$map": {
                  "from": {
                    "$ref": "request.providerOptions.xai-video.reference_audios"
                  },
                  "as": "voice",
                  "in": {
                    "voice_id": {
                      "$lower": {
                        "$trim": {
                          "$ref": "voice.voice_id"
                        }
                      }
                    }
                  }
                }
              }
            },
            "keyframes": {
              "$omitEmpty": {
                "$map": {
                  "from": {
                    "$ref": "request.providerOptions.xai-video.keyframes"
                  },
                  "as": "frame",
                  "in": {
                    "image": {
                      "url": {
                        "$omitEmpty": {
                          "$ref": "frame.image.url"
                        }
                      },
                      "file_id": {
                        "$omitEmpty": {
                          "$ref": "frame.image.file_id"
                        }
                      }
                    },
                    "timestamp_s": {
                      "$toFloat": {
                        "$ref": "frame.timestamp_s"
                      }
                    }
                  }
                }
              }
            },
            "video": {
              "$if": {
                "condition": {
                  "$or": [
                    {
                      "$gt": [
                        {
                          "$len": {
                            "$ref": "request.videos"
                          }
                        },
                        0
                      ]
                    },
                    {
                      "$eq": [
                        {
                          "$ref": "request.operation"
                        },
                        "extend"
                      ]
                    },
                    {
                      "$in": [
                        {
                          "$ref": "request.operation"
                        },
                        [
                          "edit",
                          "video_to_video",
                          "inpaint",
                          "replace_element",
                          "camera_motion",
                          "style_transfer"
                        ]
                      ]
                    }
                  ]
                },
                "then": {
                  "url": {
                    "$first": {
                      "$map": {
                        "from": {
                          "$ref": "request.videos"
                        },
                        "as": "media",
                        "in": {
                          "$ref": "media.value"
                        }
                      }
                    }
                  }
                },
                "else": null
              }
            }
          },
          "pathTemplate": {
            "$if": {
              "condition": {
                "$or": [
                  {
                    "$gt": [
                      {
                        "$len": {
                          "$ref": "request.videos"
                        }
                      },
                      0
                    ]
                  },
                  {
                    "$eq": [
                      {
                        "$ref": "request.operation"
                      },
                      "extend"
                    ]
                  },
                  {
                    "$in": [
                      {
                        "$ref": "request.operation"
                      },
                      [
                        "edit",
                        "video_to_video",
                        "inpaint",
                        "replace_element",
                        "camera_motion",
                        "style_transfer"
                      ]
                    ]
                  }
                ]
              },
              "then": {
                "$if": {
                  "condition": {
                    "$eq": [
                      {
                        "$ref": "request.operation"
                      },
                      "extend"
                    ]
                  },
                  "then": "/v1/videos/extensions",
                  "else": "/v1/videos/edits"
                }
              },
              "else": "/v1/videos/generations"
            }
          }
        },
        "poll": {
          "method": "GET",
          "path": "/v1/videos/{{taskId}}"
        },
        "response": {
          "taskId": {
            "$coalesce": [
              {
                "$ref": "response.request_id"
              },
              {
                "$ref": "response.id"
              },
              {
                "$ref": "response.task_id"
              },
              {
                "$ref": "response.taskId"
              },
              {
                "$ref": "response.data.id"
              },
              {
                "$ref": "response.data.task_id"
              },
              {
                "$ref": "response.data.taskId"
              },
              {
                "$ref": "response.data.request_id"
              },
              {
                "$ref": "response.task.id"
              },
              {
                "$ref": "response.task.task_id"
              },
              {
                "$ref": "response.task.taskId"
              },
              {
                "$ref": "response.task.request_id"
              },
              {
                "$ref": "taskId"
              }
            ]
          },
          "status": {
            "$if": {
              "condition": {
                "$eq": [
                  {
                    "$lower": {
                      "$coalesce": [
                        {
                          "$ref": "response.status"
                        },
                        {
                          "$ref": "response.state"
                        },
                        {
                          "$ref": "response.data.status"
                        },
                        {
                          "$ref": "response.data.state"
                        },
                        {
                          "$ref": "response.task.status"
                        },
                        {
                          "$ref": "response.task.state"
                        },
                        "pending"
                      ]
                    }
                  },
                  "expired"
                ]
              },
              "then": "failed",
              "else": {
                "$coalesce": [
                  {
                    "$ref": "response.status"
                  },
                  {
                    "$ref": "response.state"
                  },
                  {
                    "$ref": "response.data.status"
                  },
                  {
                    "$ref": "response.data.state"
                  },
                  {
                    "$ref": "response.task.status"
                  },
                  {
                    "$ref": "response.task.state"
                  },
                  "pending"
                ]
              }
            }
          },
          "message": {
            "$coalesce": [
              {
                "$ref": "response.error.message"
              },
              {
                "$ref": "response.message"
              },
              {
                "$ref": "response.fail_reason"
              },
              {
                "$if": {
                  "condition": {
                    "$eq": [
                      {
                        "$lower": {
                          "$coalesce": [
                            {
                              "$ref": "response.status"
                            },
                            {
                              "$ref": "response.state"
                            },
                            {
                              "$ref": "response.data.status"
                            },
                            {
                              "$ref": "response.data.state"
                            },
                            {
                              "$ref": "response.task.status"
                            },
                            {
                              "$ref": "response.task.state"
                            },
                            "pending"
                          ]
                        }
                      },
                      "expired"
                    ]
                  },
                  "then": "xAI 视频任务已过期，请重新生成",
                  "else": null
                }
              }
            ]
          },
          "videos": {
            "$coalesce": [
              {
                "$ref": "response.video.url"
              },
              {
                "$ref": "response.video_url"
              },
              {
                "$ref": "response.videoUrl"
              },
              {
                "$ref": "response.result_url"
              },
              {
                "$ref": "response.url"
              },
              {
                "$ref": "response.videos"
              },
              {
                "$ref": "response.data.video_url"
              },
              {
                "$ref": "response.data.video.url"
              },
              {
                "$ref": "response.data.videos"
              },
              {
                "$ref": "response.task.video.url"
              },
              {
                "$ref": "response.task.videos"
              },
              {
                "$ref": "response.content.video_url"
              },
              {
                "$ref": "response.content.url"
              },
              {
                "$ref": "response.output.url"
              }
            ]
          },
          "errorPaths": [
            "error.code"
          ],
          "resultEphemeral": true
        }
      }
    ]
  }
}
```
<!-- YINGCE_MANIFEST_CONTRACT_END -->
