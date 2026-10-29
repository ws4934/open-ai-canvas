# xAI 视频生成

新视频优先使用 `grok-imagine-video-1.5`；需要编辑、延长已有视频时使用 `grok-imagine-video`。Provider ID 保持 `xai-video`，独立插件与内置入口复用同一份生成的声明式合同。

## 接口与鉴权

{{OPERATIONS}}

```http
POST /v1/videos/generations
POST /v1/videos/edits
POST /v1/videos/extensions
GET  /v1/videos/{request_id}
Authorization: Bearer <XAI_API_KEY>
Content-Type: application/json
```

## 模型配置与后续型号

插件的 `inputSchema` 定义字段类型、请求映射、素材角色和逐项参数；渠道模型的 `capabilityConfig.inputConfig` 保存支持值、默认值、数量、时长及组合限制。后台「能力与参数」可应用插件模板或创建自定义配置，再按供应商实际能力调整。应用模板只修改当前模型能力，不修改价格；保存前检查已有价格档是否覆盖新规格。

`Video`、`Video 1.5`、`Lite` 是可编辑模板，模板里的模型 ID 只用于推荐，不是白名单。同协议的新型号只需填写模型 ID，选接近的模板并调整参数；时长、参考数量、分辨率不被旧模板锁死。未知型号不会默认当作旧版。只有字段或报文格式变化时才更新插件；已有模型不会因插件升级自动换模板。

## 随包模板的素材与清晰度

两代普通生成均为 1–15 秒，未指定时为 8 秒。旧版参考模式最多 7 张图、10 秒；完整 1.5 为 14 张参考图、15 秒，并支持首帧与参考图同用、尾帧、最多 4 个中间关键帧和 3 个预设音色。

完整 1.5 的基础文生及仅首帧图生可使用原生 1080p；参考图、音色、尾帧或关键帧模式最高 720p。插件明确拒绝无效组合，不截断素材、不降清晰度、不改选模型。纯首帧模式沿用图片画幅，不发送无效的 `aspect_ratio`；其他生成支持官方九种比例。

`grok-imagine-video-1.5-lite` 模板仅开放基础文生、首帧。API 直调也可通过 `metadata.providerOptions["xai-video"].modelProfile` 选择包内模板；应用内以已保存的模型配置为准。

总图片槽包含首帧、尾帧、参考图和关键帧；完整 1.5 模板为 20 个，参考图自身最多 14 张。按角色分别校验，不以总槽数代替各角色上限。站点价格不随官方美元报价自动改动。

## 参数与字段映射

{{PARAMETERS}}

首帧为 `image`，尾帧为 `last_frame`，角色、场景等参考图为 `reference_images`。图生模式的未标记图片按首张为首帧、其余为参考图处理；参考模式全部作为参考图，不丢弃多余输入。

视频工作台和画布视频设置按模型配置显示参考音色；选择图片的「关键帧」用途后填写时间点。模型的 720p 等组合规则在表单联动提示，提交时服务端再次验证，参数不静默截断或降级。

工作台 API 使用 `parameters.inputParameters.reference_audios`，素材项使用 `role=keyframe` 与 `inputFields.timestamp_s`。画布任务使用 `config.inputParameters`，素材仍在原 `referenceImages` 列表中；画布与 Agent 审批使用 JSON 字符串 `inputMediaRoles`，按已有引用 ID 指定用途，不能借此添加新资源。所有引用仍走归属、上传与持久化检查。

原生接口参数也可通过统一任务 API 的 `metadata.providerOptions` 传入：

```json
{
  "providerOptions": {
    "xai-video": {
      "reference_audios": [{ "voice_id": "eve" }],
      "keyframes": [
        { "image": { "url": "https://example.com/frame.png" }, "timestamp_s": 2.0 }
      ]
    }
  }
}
```

关键帧时间需严格处于 `(0, duration)` 内，不能落在同一个 1/3 秒网格；图片使用 `url` 或 `file_id` 二选一。`voice_id` 不区分大小写，最多 3 个预设；自有音频音色需 xAI 额外授权，普通音频文件不冒充预设音色。插件不发送上游未定义的 `generate_audio` 或 `watermark` 开关，音轨由 xAI 原生生成。默认能力用 `generateAudio.fixed=true` 让界面只读显示原生声音，不提供无效的关闭开关；不改变已有任务输入的接受条件。

## 旧版编辑与延长

统一 `videos` 传一个源视频。`metadata.videoEditOperation=extend` 选择延长接口，`duration` 是新增部分，2–10 秒、默认 6 秒；`inpaint`、`replace_element`、`camera_motion`、`style_transfer` 选择编辑接口。

已知源时长时校验编辑最多 8.7 秒、延长 2–15 秒；未探测的外部源由上游校验，不虚构媒体时长。编辑不发送 `duration`，两种源视频操作都不发送 `aspect_ratio` / `resolution`；输出沿用源视频并受上游 720p 上限约束。1.5 不提供视频编辑或延长。

## 查询与错误

创建响应读取 `request_id`，完成时优先读取 `video.url`；仍接受已有网关的平铺结果字段。临时地址由宿主持久化。`expired` 作为失败结束，不无限轮询；上游错误保留具体原因。关闭页面不取消已提交任务。

## 官方资料

- [能力总表](https://docs.x.ai/developers/model-capabilities/video/overview)
- [参考图、音色与定帧](https://docs.x.ai/developers/model-capabilities/video/reference-to-video)
- [生成参数](https://docs.x.ai/developers/model-capabilities/video/generation)
- [视频编辑](https://docs.x.ai/developers/model-capabilities/video/editing)
- [视频延长](https://docs.x.ai/developers/model-capabilities/video/extension)

{{CONTRACT}}
