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
| `prompt` | string | 否 | `prompt` | 有首帧、尾帧或关键帧时可省略。 |
| `images` | media[] | 否 | `first/last/reference image` | 显式 role 图片输入。 |
| `videos` | media[] | 否 | `reference video` | 参考视频。 |
| `duration` | integer | 否 | `duration/seconds` | 时长秒数。 |
| `aspectRatio` | string | 否 | `ratio/aspect_ratio/size` | 画幅比例或尺寸。 |
| `resolution` | string | 否 | `resolution` | 分辨率档位。 |
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
| `create.body.aspect_ratio` | `{"$if":{"condition":{"$and":[{"$not":{"$or":[{"$gt":[{"$len":{"$ref":"request.videos"}},0]},{"$eq":[{"$ref":"request.operation"},"extend"]},{"$in":[{"$ref":"request.operation"},["edit","video_to_video","inpaint","replace_element","camera_motion","style_transfer"]]}]}},{"$or":[{"$not":{"$coalesce":[{"$first":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["first_frame"]]}}}},{"$if":{"condition":{"$and":[{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["first_frame"]]}}}},0]},{"$or":[{"$eq":[{"$ref":"request.operation"},"image_to_video"]},{"$and":[{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},[""]]}}}},1]},{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["reference_image","subject_reference","style_reference"]]}}}},0]},{"$ne":[{"$ref":"request.operation"},"reference_to_video"]}]}]}]},"then":{"$first":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},[""]]}}}},"else":null}}]}},{"$or":[{"$gt":[{"$len":{"$sortByOrder":{"$concatArrays":[{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["reference_image","subject_reference","style_reference"]]}}},{"$filter":{"from":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},[""]]}}},"as":"candidate","where":{"$or":[{"$not":{"$and":[{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["first_frame"]]}}}},0]},{"$or":[{"$eq":[{"$ref":"request.operation"},"image_to_video"]},{"$and":[{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},[""]]}}}},1]},{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["reference_image","subject_reference","style_reference"]]}}}},0]},{"$ne":[{"$ref":"request.operation"},"reference_to_video"]}]}]}]}},{"$gt":[{"$ref":"candidateIndex"},0]}]}}}]}}},0]},{"$gt":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["last_frame"]]}}}},0]},{"$gt":[{"$len":{"$ref":"request.providerOptions.xai-video.reference_audios"}},0]},{"$gt":[{"$len":{"$concatArrays":[{"$ref":"request.providerOptions.xai-video.keyframes"},{"$map":{"from":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["keyframe"]]}}},"as":"frame","in":{"image":{"url":{"$ref":"frame.value"}},"timestamp_s":{"$ref":"frame.metadata.timestamp_s"}}}}]}},0]}]}]}]},"then":{"$coalesce":[{"$ref":"request.aspectRatio"},"16:9"]},"else":null}}` |
| `create.body.resolution` | `{"$if":{"condition":{"$not":{"$or":[{"$gt":[{"$len":{"$ref":"request.videos"}},0]},{"$eq":[{"$ref":"request.operation"},"extend"]},{"$in":[{"$ref":"request.operation"},["edit","video_to_video","inpaint","replace_element","camera_motion","style_transfer"]]}]}},"then":{"$coalesce":[{"$ref":"request.resolution"},"720p"]},"else":null}}` |
| `create.body.image` | `{"$if":{"condition":{"$coalesce":[{"$first":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["first_frame"]]}}}},{"$if":{"condition":{"$and":[{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["first_frame"]]}}}},0]},{"$or":[{"$eq":[{"$ref":"request.operation"},"image_to_video"]},{"$and":[{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},[""]]}}}},1]},{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["reference_image","subject_reference","style_reference"]]}}}},0]},{"$ne":[{"$ref":"request.operation"},"reference_to_video"]}]}]}]},"then":{"$first":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},[""]]}}}},"else":null}}]},"then":{"url":{"$first":{"$map":{"from":[{"$coalesce":[{"$first":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["first_frame"]]}}}},{"$if":{"condition":{"$and":[{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["first_frame"]]}}}},0]},{"$or":[{"$eq":[{"$ref":"request.operation"},"image_to_video"]},{"$and":[{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},[""]]}}}},1]},{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["reference_image","subject_reference","style_reference"]]}}}},0]},{"$ne":[{"$ref":"request.operation"},"reference_to_video"]}]}]}]},"then":{"$first":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},[""]]}}}},"else":null}}]}],"as":"image","in":{"$ref":"image.value"}}}}},"else":null}}` |
| `create.body.last_frame` | `{"$if":{"condition":{"$gt":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["last_frame"]]}}}},0]},"then":{"url":{"$first":{"$map":{"from":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["last_frame"]]}}},"as":"media","in":{"$ref":"media.value"}}}}},"else":null}}` |
| `create.body.reference_images` | `{"$omitEmpty":{"$map":{"from":{"$sortByOrder":{"$concatArrays":[{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["reference_image","subject_reference","style_reference"]]}}},{"$filter":{"from":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},[""]]}}},"as":"candidate","where":{"$or":[{"$not":{"$and":[{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["first_frame"]]}}}},0]},{"$or":[{"$eq":[{"$ref":"request.operation"},"image_to_video"]},{"$and":[{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},[""]]}}}},1]},{"$eq":[{"$len":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["reference_image","subject_reference","style_reference"]]}}}},0]},{"$ne":[{"$ref":"request.operation"},"reference_to_video"]}]}]}]}},{"$gt":[{"$ref":"candidateIndex"},0]}]}}}]}},"as":"media","in":{"url":{"$ref":"media.value"}}}}}` |
| `create.body.reference_audios` | `{"$omitEmpty":{"$map":{"from":{"$ref":"request.providerOptions.xai-video.reference_audios"},"as":"voice","in":{"voice_id":{"$lower":{"$trim":{"$ref":"voice.voice_id"}}}}}}}` |
| `create.body.keyframes` | `{"$omitEmpty":{"$map":{"from":{"$concatArrays":[{"$ref":"request.providerOptions.xai-video.keyframes"},{"$map":{"from":{"$filter":{"from":{"$sortByOrder":{"$ref":"request.images"}},"as":"media","where":{"$in":[{"$ref":"media.role"},["keyframe"]]}}},"as":"frame","in":{"image":{"url":{"$ref":"frame.value"}},"timestamp_s":{"$ref":"frame.metadata.timestamp_s"}}}}]},"as":"frame","in":{"image":{"url":{"$omitEmpty":{"$ref":"frame.image.url"}},"file_id":{"$omitEmpty":{"$ref":"frame.image.file_id"}}},"timestamp_s":{"$toFloat":{"$ref":"frame.timestamp_s"}}}}}}` |
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

inputSchema 声明协议字段、素材角色与可选能力模板。旧版、Video 1.5 和 Lite 是可编辑的配置预设，不是型号白名单。新增同协议型号可以选择模板并调整数量、时长、清晰度与组合规则，不需要修改宿主或插件。已保存模型以自身 inputConfig 为准；没有配置的已知型号可使用包内推荐模板，未知型号不会被归类为旧版。协议只负责创建、轮询、字段编码及结构校验。

预设音色用 reference_audios 的 voice_id；关键帧可用 keyframe 图片角色和 inputFields.timestamp_s，也保留 providerOptions.xai-video.keyframes 的原生输入。所有媒体使用原资源归属、上传与持久化链路。模板与能力配置不修改运营价格。

官方资料：[参考与定帧](https://docs.x.ai/developers/model-capabilities/video/reference-to-video)、[生成](https://docs.x.ai/developers/model-capabilities/video/generation)、[编辑](https://docs.x.ai/developers/model-capabilities/video/editing)、[延长](https://docs.x.ai/developers/model-capabilities/video/extension)。

<!-- YINGCE_MANIFEST_CONTRACT_START -->
## Manifest 完整接口定义

以下 JSON 与插件包内实际 `manifest.json` 逐字段一致，覆盖插件身份、权限、配置、鉴权、参数、校验、创建、Agent、查询、取消、结果下载、响应和 Agent 响应映射。`documentation` 字段的值就是当前完整文档；为避免文档在自身内部无限递归，JSON 中仅用等义占位文本表示正文。

```json
{
  "apiVersion": "yingce.plugin/v2",
  "id": "xai-video",
  "name": "xAI Video",
  "version": "2.2.0",
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
        "inputSchema": {
          "fields": [
            {
              "name": "videoSeconds",
              "label": "视频时长（秒）",
              "type": "integer",
              "binding": "duration",
              "pricing": true,
              "operations": [
                "text_to_video",
                "image_to_video",
                "reference_to_video",
                "extend"
              ]
            },
            {
              "name": "vquality",
              "label": "清晰度",
              "type": "string",
              "binding": "resolution",
              "pricing": true,
              "operations": [
                "text_to_video",
                "image_to_video",
                "reference_to_video"
              ]
            },
            {
              "name": "size",
              "label": "画面比例",
              "type": "string",
              "binding": "aspectRatio",
              "operations": [
                "text_to_video",
                "image_to_video",
                "reference_to_video"
              ]
            },
            {
              "name": "videoGenerateAudio",
              "label": "原生音频",
              "type": "boolean",
              "binding": "generateAudio",
              "readOnly": true,
              "default": true
            },
            {
              "name": "operation",
              "label": "生成方式",
              "type": "string",
              "binding": "operation",
              "hidden": true
            },
            {
              "name": "reference_audios",
              "label": "参考音色",
              "type": "object[]",
              "binding": "providerOptions.xai-video.reference_audios",
              "description": "填写官方预设 voice_id；在提示词中用 <AUDIO_0> 等引用。",
              "fields": [
                {
                  "name": "voice_id",
                  "label": "音色 ID",
                  "type": "string",
                  "required": true,
                  "maxLength": 64
                }
              ]
            }
          ],
          "mediaRoles": [
            {
              "role": "first_frame",
              "label": "首帧",
              "kind": "image",
              "single": true,
              "maxCount": 0
            },
            {
              "role": "last_frame",
              "label": "尾帧",
              "kind": "image",
              "single": true,
              "maxCount": 0
            },
            {
              "role": "reference_image",
              "label": "参考图",
              "kind": "image",
              "maxCount": 0
            },
            {
              "role": "keyframe",
              "label": "关键帧",
              "kind": "image",
              "maxCount": 0,
              "fields": [
                {
                  "name": "timestamp_s",
                  "label": "时间点（秒）",
                  "type": "number",
                  "required": true,
                  "min": 0,
                  "minExclusive": true,
                  "upperBoundField": "videoSeconds"
                }
              ],
              "uniqueField": "timestamp_s",
              "grid": 0.3333333333333333,
              "collectionBinding": "providerOptions.xai-video.keyframes"
            },
            {
              "role": "reference_video",
              "label": "源视频",
              "kind": "video",
              "single": true,
              "maxCount": 0
            }
          ],
          "templates": [
            {
              "id": "video-1.5",
              "label": "Video 1.5：生成与定帧",
              "models": [
                "grok-imagine-video-1.5"
              ],
              "config": {
                "operations": [
                  "text_to_video",
                  "image_to_video",
                  "reference_to_video"
                ],
                "fields": [
                  {
                    "name": "videoSeconds",
                    "label": "视频时长（秒）",
                    "type": "integer",
                    "binding": "duration",
                    "pricing": true,
                    "operations": [
                      "text_to_video",
                      "image_to_video",
                      "reference_to_video",
                      "extend"
                    ],
                    "min": 1,
                    "max": 15,
                    "step": 1,
                    "default": 8
                  },
                  {
                    "name": "vquality",
                    "label": "清晰度",
                    "type": "string",
                    "binding": "resolution",
                    "pricing": true,
                    "operations": [
                      "text_to_video",
                      "image_to_video",
                      "reference_to_video"
                    ],
                    "values": [
                      "480p",
                      "720p",
                      "1080p"
                    ],
                    "default": "720p"
                  },
                  {
                    "name": "size",
                    "label": "画面比例",
                    "type": "string",
                    "binding": "aspectRatio",
                    "operations": [
                      "text_to_video",
                      "image_to_video",
                      "reference_to_video"
                    ],
                    "values": [
                      "1:1",
                      "16:9",
                      "9:16",
                      "4:3",
                      "3:4",
                      "3:2",
                      "2:3",
                      "21:9",
                      "5:2"
                    ],
                    "default": "16:9"
                  },
                  {
                    "name": "videoGenerateAudio",
                    "label": "原生音频",
                    "type": "boolean",
                    "binding": "generateAudio",
                    "readOnly": true,
                    "default": true
                  },
                  {
                    "name": "operation",
                    "label": "生成方式",
                    "type": "string",
                    "binding": "operation",
                    "hidden": true
                  },
                  {
                    "name": "reference_audios",
                    "label": "参考音色",
                    "type": "object[]",
                    "binding": "providerOptions.xai-video.reference_audios",
                    "description": "填写官方预设 voice_id；在提示词中用 <AUDIO_0> 等引用。",
                    "fields": [
                      {
                        "name": "voice_id",
                        "label": "音色 ID",
                        "type": "string",
                        "required": true,
                        "maxLength": 64
                      }
                    ],
                    "enabled": true,
                    "maxItems": 3,
                    "default": []
                  }
                ],
                "mediaRoles": [
                  {
                    "role": "first_frame",
                    "label": "首帧",
                    "kind": "image",
                    "single": true,
                    "maxCount": 1
                  },
                  {
                    "role": "last_frame",
                    "label": "尾帧",
                    "kind": "image",
                    "single": true,
                    "maxCount": 1
                  },
                  {
                    "role": "reference_image",
                    "label": "参考图",
                    "kind": "image",
                    "maxCount": 14
                  },
                  {
                    "role": "keyframe",
                    "label": "关键帧",
                    "kind": "image",
                    "maxCount": 4,
                    "fields": [
                      {
                        "name": "timestamp_s",
                        "label": "时间点（秒）",
                        "type": "number",
                        "required": true,
                        "min": 0,
                        "minExclusive": true,
                        "upperBoundField": "videoSeconds"
                      }
                    ],
                    "uniqueField": "timestamp_s",
                    "grid": 0.3333333333333333,
                    "collectionBinding": "providerOptions.xai-video.keyframes"
                  },
                  {
                    "role": "reference_video",
                    "label": "源视频",
                    "kind": "video",
                    "single": true,
                    "maxCount": 0
                  }
                ],
                "rules": [
                  {
                    "match": "any",
                    "when": [
                      {
                        "mediaRole": "reference_image"
                      },
                      {
                        "mediaRole": "last_frame"
                      },
                      {
                        "mediaRole": "keyframe"
                      },
                      {
                        "parameter": "reference_audios"
                      }
                    ],
                    "limits": [
                      {
                        "name": "vquality",
                        "values": [
                          "480p",
                          "720p"
                        ]
                      }
                    ],
                    "message": "此模型的参考、音色或定帧模式最高 720p，请调整清晰度"
                  }
                ]
              }
            },
            {
              "id": "video",
              "label": "Video：生成、编辑与延长",
              "models": [
                "grok-imagine-video"
              ],
              "config": {
                "operations": [
                  "text_to_video",
                  "image_to_video",
                  "reference_to_video",
                  "extend",
                  "edit",
                  "video_to_video",
                  "inpaint",
                  "replace_element",
                  "camera_motion",
                  "style_transfer"
                ],
                "fields": [
                  {
                    "name": "videoSeconds",
                    "label": "视频时长（秒）",
                    "type": "integer",
                    "binding": "duration",
                    "pricing": true,
                    "operations": [
                      "text_to_video",
                      "image_to_video",
                      "reference_to_video",
                      "extend"
                    ],
                    "min": 1,
                    "max": 15,
                    "step": 1,
                    "default": 8
                  },
                  {
                    "name": "vquality",
                    "label": "清晰度",
                    "type": "string",
                    "binding": "resolution",
                    "pricing": true,
                    "operations": [
                      "text_to_video",
                      "image_to_video",
                      "reference_to_video"
                    ],
                    "values": [
                      "480p",
                      "720p"
                    ],
                    "default": "720p"
                  },
                  {
                    "name": "size",
                    "label": "画面比例",
                    "type": "string",
                    "binding": "aspectRatio",
                    "operations": [
                      "text_to_video",
                      "image_to_video",
                      "reference_to_video"
                    ],
                    "values": [
                      "1:1",
                      "16:9",
                      "9:16",
                      "4:3",
                      "3:4",
                      "3:2",
                      "2:3",
                      "21:9",
                      "5:2"
                    ],
                    "default": "16:9"
                  },
                  {
                    "name": "videoGenerateAudio",
                    "label": "原生音频",
                    "type": "boolean",
                    "binding": "generateAudio",
                    "readOnly": true,
                    "default": true
                  },
                  {
                    "name": "operation",
                    "label": "生成方式",
                    "type": "string",
                    "binding": "operation",
                    "hidden": true
                  },
                  {
                    "name": "reference_audios",
                    "label": "参考音色",
                    "type": "object[]",
                    "binding": "providerOptions.xai-video.reference_audios",
                    "description": "填写官方预设 voice_id；在提示词中用 <AUDIO_0> 等引用。",
                    "fields": [
                      {
                        "name": "voice_id",
                        "label": "音色 ID",
                        "type": "string",
                        "required": true,
                        "maxLength": 64
                      }
                    ],
                    "enabled": false,
                    "maxItems": 3,
                    "default": []
                  }
                ],
                "mediaRoles": [
                  {
                    "role": "first_frame",
                    "label": "首帧",
                    "kind": "image",
                    "single": true,
                    "maxCount": 1
                  },
                  {
                    "role": "last_frame",
                    "label": "尾帧",
                    "kind": "image",
                    "single": true,
                    "maxCount": 0
                  },
                  {
                    "role": "reference_image",
                    "label": "参考图",
                    "kind": "image",
                    "maxCount": 7
                  },
                  {
                    "role": "keyframe",
                    "label": "关键帧",
                    "kind": "image",
                    "maxCount": 0,
                    "fields": [
                      {
                        "name": "timestamp_s",
                        "label": "时间点（秒）",
                        "type": "number",
                        "required": true,
                        "min": 0,
                        "minExclusive": true,
                        "upperBoundField": "videoSeconds"
                      }
                    ],
                    "uniqueField": "timestamp_s",
                    "grid": 0.3333333333333333,
                    "collectionBinding": "providerOptions.xai-video.keyframes"
                  },
                  {
                    "role": "reference_video",
                    "label": "源视频",
                    "kind": "video",
                    "single": true,
                    "maxCount": 1,
                    "maxSeconds": 8.7
                  }
                ],
                "rules": [
                  {
                    "when": [
                      {
                        "mediaRole": "first_frame"
                      }
                    ],
                    "forbidRoles": [
                      "reference_image"
                    ],
                    "message": "此模型不能混用首帧和参考图"
                  },
                  {
                    "when": [
                      {
                        "mediaRole": "reference_image"
                      }
                    ],
                    "limits": [
                      {
                        "name": "videoSeconds",
                        "max": 10
                      }
                    ],
                    "message": "此模型的参考图模式最多 10 秒"
                  },
                  {
                    "when": [
                      {
                        "parameter": "operation",
                        "values": [
                          "extend"
                        ]
                      }
                    ],
                    "limits": [
                      {
                        "name": "videoSeconds",
                        "min": 2,
                        "max": 10,
                        "default": 6
                      }
                    ],
                    "mediaLimits": [
                      {
                        "role": "reference_video",
                        "minSeconds": 2,
                        "maxSeconds": 15
                      }
                    ],
                    "message": "此模型的视频延长新增时长为 2–10 秒"
                  },
                  {
                    "when": [
                      {
                        "parameter": "operation",
                        "values": [
                          "inpaint",
                          "edit",
                          "video_to_video",
                          "replace_element",
                          "camera_motion",
                          "style_transfer"
                        ]
                      }
                    ],
                    "mediaLimits": [
                      {
                        "role": "reference_video",
                        "maxSeconds": 8.7
                      }
                    ],
                    "message": "此模型的编辑源视频最多 8.7 秒"
                  }
                ]
              }
            },
            {
              "id": "video-1.5-lite",
              "label": "Video 1.5 Lite：基础生成",
              "models": [
                "grok-imagine-video-1.5-lite"
              ],
              "config": {
                "operations": [
                  "text_to_video",
                  "image_to_video"
                ],
                "fields": [
                  {
                    "name": "videoSeconds",
                    "label": "视频时长（秒）",
                    "type": "integer",
                    "binding": "duration",
                    "pricing": true,
                    "operations": [
                      "text_to_video",
                      "image_to_video",
                      "reference_to_video",
                      "extend"
                    ],
                    "min": 1,
                    "max": 15,
                    "step": 1,
                    "default": 8
                  },
                  {
                    "name": "vquality",
                    "label": "清晰度",
                    "type": "string",
                    "binding": "resolution",
                    "pricing": true,
                    "operations": [
                      "text_to_video",
                      "image_to_video",
                      "reference_to_video"
                    ],
                    "values": [
                      "480p",
                      "720p",
                      "1080p"
                    ],
                    "default": "720p"
                  },
                  {
                    "name": "size",
                    "label": "画面比例",
                    "type": "string",
                    "binding": "aspectRatio",
                    "operations": [
                      "text_to_video",
                      "image_to_video",
                      "reference_to_video"
                    ],
                    "values": [
                      "1:1",
                      "16:9",
                      "9:16",
                      "4:3",
                      "3:4",
                      "3:2",
                      "2:3",
                      "21:9",
                      "5:2"
                    ],
                    "default": "16:9"
                  },
                  {
                    "name": "videoGenerateAudio",
                    "label": "原生音频",
                    "type": "boolean",
                    "binding": "generateAudio",
                    "readOnly": true,
                    "default": true
                  },
                  {
                    "name": "operation",
                    "label": "生成方式",
                    "type": "string",
                    "binding": "operation",
                    "hidden": true
                  },
                  {
                    "name": "reference_audios",
                    "label": "参考音色",
                    "type": "object[]",
                    "binding": "providerOptions.xai-video.reference_audios",
                    "description": "填写官方预设 voice_id；在提示词中用 <AUDIO_0> 等引用。",
                    "fields": [
                      {
                        "name": "voice_id",
                        "label": "音色 ID",
                        "type": "string",
                        "required": true,
                        "maxLength": 64
                      }
                    ],
                    "enabled": false,
                    "maxItems": 3,
                    "default": []
                  }
                ],
                "mediaRoles": [
                  {
                    "role": "first_frame",
                    "label": "首帧",
                    "kind": "image",
                    "single": true,
                    "maxCount": 1
                  },
                  {
                    "role": "last_frame",
                    "label": "尾帧",
                    "kind": "image",
                    "single": true,
                    "maxCount": 0
                  },
                  {
                    "role": "reference_image",
                    "label": "参考图",
                    "kind": "image",
                    "maxCount": 0
                  },
                  {
                    "role": "keyframe",
                    "label": "关键帧",
                    "kind": "image",
                    "maxCount": 0,
                    "fields": [
                      {
                        "name": "timestamp_s",
                        "label": "时间点（秒）",
                        "type": "number",
                        "required": true,
                        "min": 0,
                        "minExclusive": true,
                        "upperBoundField": "videoSeconds"
                      }
                    ],
                    "uniqueField": "timestamp_s",
                    "grid": 0.3333333333333333,
                    "collectionBinding": "providerOptions.xai-video.keyframes"
                  },
                  {
                    "role": "reference_video",
                    "label": "源视频",
                    "kind": "video",
                    "single": true,
                    "maxCount": 0
                  }
                ],
                "rules": []
              }
            }
          ]
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
            "description": "有首帧、尾帧或关键帧时可省略。"
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
                              "keyframe",
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
              "$eq": [
                {
                  "$len": {
                    "$ref": "request.audios"
                  }
                },
                0
              ]
            },
            "message": "xAI 公共视频接口不接收音频文件；请使用预设 voice_id"
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
            "message": "reference_audios 仅支持非空 voice_id"
          },
          {
            "assert": {
              "$eq": [
                {
                  "$len": {
                    "$filter": {
                      "from": {
                        "$concatArrays": [
                          {
                            "$ref": "request.providerOptions.xai-video.keyframes"
                          },
                          {
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
                                        "keyframe"
                                      ]
                                    ]
                                  }
                                }
                              },
                              "as": "frame",
                              "in": {
                                "image": {
                                  "url": {
                                    "$ref": "frame.value"
                                  }
                                },
                                "timestamp_s": {
                                  "$ref": "frame.metadata.timestamp_s"
                                }
                              }
                            }
                          }
                        ]
                      },
                      "as": "frame",
                      "where": {
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
                    "$concatArrays": [
                      {
                        "$ref": "request.providerOptions.xai-video.keyframes"
                      },
                      {
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
                                    "keyframe"
                                  ]
                                ]
                              }
                            }
                          },
                          "as": "frame",
                          "in": {
                            "image": {
                              "url": {
                                "$ref": "frame.value"
                              }
                            },
                            "timestamp_s": {
                              "$ref": "frame.metadata.timestamp_s"
                            }
                          }
                        }
                      }
                    ]
                  }
                }
              ]
            },
            "message": "每个关键帧需 image.url 或 image.file_id，timestamp_s 必须大于 0 且小于视频时长"
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
                            "$concatArrays": [
                              {
                                "$ref": "request.providerOptions.xai-video.keyframes"
                              },
                              {
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
                                            "keyframe"
                                          ]
                                        ]
                                      }
                                    }
                                  },
                                  "as": "frame",
                                  "in": {
                                    "image": {
                                      "url": {
                                        "$ref": "frame.value"
                                      }
                                    },
                                    "timestamp_s": {
                                      "$ref": "frame.metadata.timestamp_s"
                                    }
                                  }
                                }
                              }
                            ]
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
                                "$concatArrays": [
                                  {
                                    "$ref": "request.providerOptions.xai-video.keyframes"
                                  },
                                  {
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
                                                "keyframe"
                                              ]
                                            ]
                                          }
                                        }
                                      },
                                      "as": "frame",
                                      "in": {
                                        "image": {
                                          "url": {
                                            "$ref": "frame.value"
                                          }
                                        },
                                        "timestamp_s": {
                                          "$ref": "frame.metadata.timestamp_s"
                                        }
                                      }
                                    }
                                  }
                                ]
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
                                    "$concatArrays": [
                                      {
                                        "$ref": "request.providerOptions.xai-video.keyframes"
                                      },
                                      {
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
                                                    "keyframe"
                                                  ]
                                                ]
                                              }
                                            }
                                          },
                                          "as": "frame",
                                          "in": {
                                            "image": {
                                              "url": {
                                                "$ref": "frame.value"
                                              }
                                            },
                                            "timestamp_s": {
                                              "$ref": "frame.metadata.timestamp_s"
                                            }
                                          }
                                        }
                                      }
                                    ]
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
                    "$concatArrays": [
                      {
                        "$ref": "request.providerOptions.xai-video.keyframes"
                      },
                      {
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
                                    "keyframe"
                                  ]
                                ]
                              }
                            }
                          },
                          "as": "frame",
                          "in": {
                            "image": {
                              "url": {
                                "$ref": "frame.value"
                              }
                            },
                            "timestamp_s": {
                              "$ref": "frame.metadata.timestamp_s"
                            }
                          }
                        }
                      }
                    ]
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
