# Google Gemini generateContent

该目录是独立官方协议插件源码。后端从生成的 `google-gemini-generate-content.yingce-plugin` 包加载，不依赖系统内置 `host:` 适配器。

普通文本与短剧任务启用流式后，宿主使用 `streamGenerateContent?alt=sse` 接收真实 SSE，逐段传递正文与思考摘要，结束后校验完整结果。经 sub2api 连接 Vertex 时，GCP 鉴权、项目和区域仍由中转管理。

完整接口见 [docs/interface.md](docs/interface.md)。
