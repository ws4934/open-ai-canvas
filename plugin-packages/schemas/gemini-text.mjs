const ref = (path) => ({ $ref: path });
const map = (from, as, value) => ({ $map: { from, as, in: value } });
const filter = (from, as, where) => ({ $filter: { from, as, where } });
const eq = (a, b) => ({ $eq: [a, b] });
const ne = (a, b) => ({ $ne: [a, b] });
const when = (condition, yes, no = null) => ({ $if: { condition, then: yes, else: no } });
const omit = (value) => ({ $omitEmpty: value });
const coalesce = (...values) => ({ $coalesce: values });
const merge = (...values) => ({ $merge: values });
const concat = (...values) => ({ $concat: values });
const choice = (cases, fallback = null) => ({ $switch: { cases: cases.map(([when, then]) => ({ when, then })), default: fallback } });

export function geminiTextProtocol() {
  const parts = ref("response.candidates.0.content.parts");
  const text = map(filter(parts, "part", ne(ref("part.thought"), true)), "part", ref("part.text"));
  const reasoning = map(filter(parts, "part", eq(ref("part.thought"), true)), "part", ref("part.text"));
  const finish = coalesce(ref("response.promptFeedback.blockReason"), ref("response.candidates.0.finishReason"));
  const error = choice([
    [ref("response.error"), { code: coalesce(ref("response.error.status"), ref("response.error.code")), message: ref("response.error.message") }],
    [{ $in: [finish, ["SAFETY", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII", "IMAGE_SAFETY", "IMAGE_PROHIBITED_CONTENT"]] }, { code: "content_policy_violation", message: "内容未通过上游安全审核，请修改提示词或参考素材后重试" }],
    [eq(finish, "MAX_TOKENS"), { code: "output_token_limit", message: "Gemini 输出达到模型 Token 上限，内容未完整生成" }],
    [{ $and: [finish, { $not: { $in: [finish, ["STOP", "FINISH_REASON_UNSPECIFIED", "BLOCK_REASON_UNSPECIFIED"]] } }] }, { code: finish, message: concat("Gemini 未正常完成生成，结束原因：", finish) }],
  ]);
  const stream = {
    request: { path: "/v1beta/models/{{model}}:streamGenerateContent", query: { alt: "sse" } },
    text, reasoning, append: { parts }, merge: { usage: ref("response.usageMetadata") },
    complete: eq(ref("response.candidates.0.finishReason"), "STOP"), error,
    response: { candidates: [{ content: { role: "model", parts: ref("stream.append.parts") }, finishReason: "STOP" }], usageMetadata: ref("stream.merge.usage") },
  };
  const conversation = "request.extra.agent.conversation";
  const mediaPart = when(ref("part.inline"),
    { inlineData: { mimeType: ref("part.mimeType"), data: ref("part.data") } },
    { fileData: { mimeType: ref("part.mimeType"), fileUri: ref("part.url") } });
  const agentPart = choice([
    [eq(ref("part.kind"), "text"), { text: ref("part.text") }],
    [eq(ref("part.kind"), "media"), mediaPart],
    [eq(ref("part.kind"), "tool_call"), { functionCall: { id: ref("part.id"), name: ref("part.name"), args: ref("part.arguments") }, thoughtSignature: omit(ref("part.signature")) }],
    [eq(ref("part.kind"), "tool_result"), { functionResponse: { id: ref("part.id"), name: ref("part.name"), response: { result: ref("part.result") } } }],
  ]);
  const schema = { $schemaSubset: { from: ref("tool.parameters"), constToEnum: true, stringEnumOnly: true, inferStringEnumType: true, enumHintPrefix: " 可选值：", omitKeywords: ["exclusiveMinimum", "exclusiveMaximum", "uniqueItems"] } };
  const agentBody = {
    contents: map(ref(`${conversation}.messages`), "message", { role: when(eq(ref("message.role"), "assistant"), "model", "user"), parts: map(ref("message.parts"), "part", agentPart) }),
    systemInstruction: when(ref(`${conversation}.instructions`), { parts: [{ text: ref(`${conversation}.instructions`) }] }),
    tools: when(ref(`${conversation}.tools`), [{ functionDeclarations: map(ref(`${conversation}.tools`), "tool", { name: ref("tool.name"), description: omit(ref("tool.description")), parameters: schema }) }]),
    toolConfig: when(ref(`${conversation}.tools`), { functionCallingConfig: choice([
      [ref(`${conversation}.toolChoice.name`), { mode: "ANY", allowedFunctionNames: [ref(`${conversation}.toolChoice.name`)] }],
      [eq(ref(`${conversation}.toolChoice`), "required"), { mode: "ANY" }],
    ], { mode: "AUTO" }) }),
    generationConfig: { maxOutputTokens: omit(ref(`${conversation}.options.maxOutputTokens`)), thinkingConfig: when(ref(`${conversation}.options.thinking`), { includeThoughts: true }) },
  };
  const promptCache = {
    fields: ["systemInstruction", "tools", "toolConfig"],
    model: { $trimPrefix: [ref("request.model"), "models/"] },
    minimumTokens: when({ $contains: [ref("request.model"), "gemini-2.5"] }, 2048, 4096),
    create: { method: "POST", path: "/v1beta/cachedContents", contentType: "application/json", body: merge({ model: concat("models/", ref("cache.model")), ttl: "3600s" }, ref("cache.prefix")) },
    delete: { method: "DELETE", pathTemplate: concat("/v1beta/", ref("cache.resourceName")), contentType: "application/json" },
    resource: ref("response.name"), expireTime: ref("response.expireTime"),
    resourcePattern: "^cachedContents/[A-Za-z0-9_-][A-Za-z0-9_.-]*$", applyField: "cachedContent",
    missing: { $and: [eq(ref("error.status"), 404), { $contains: [ref("error.body"), ref("error.resourceName")] }, { $or: [{ $not: ref("response.error.code") }, eq(ref("response.error.code"), 404)] }, { $or: [{ $not: ref("response.error.status") }, eq(ref("response.error.status"), "NOT_FOUND")] }] },
  };
  return {
    apiVersion: "yingce.plugin/v3",
    version: "2.1.0",
    notes: "v3 合同：插件负责 conversation 到 Gemini 回合、工具 JSON Schema 子集、签名、正文/思考、SSE 终态与 Prompt Cache 接口。宿主只执行通用映射、HTTP、会话、审批、幂等和缓存隔离。Agent SSE 可接受完整 JSON；普通文本 SSE 不接受非流式响应。缓存创建失败按无缓存请求继续，不修改 Vertex IAM、已有模型能力或价格。",
    create: {
      method: "POST", path: "/v1beta/models/{{model}}:generateContent", contentType: "application/json", stream,
      body: {
        contents: map(filter(ref("request.messages"), "message", ne(ref("message.role"), "system")), "message", { role: when(eq(ref("message.role"), "assistant"), "model", "user"), parts: [{ text: ref("message.content") }] }),
        systemInstruction: when(ref("request.instructions"), { parts: [{ text: ref("request.instructions") }] }),
        generationConfig: omit(ref("request.providerOptions.gemini-generate-content.generationConfig")),
        safetySettings: omit(ref("request.providerOptions.gemini-generate-content.safetySettings")),
        tools: omit(ref("request.providerOptions.gemini-generate-content.tools")), toolConfig: omit(ref("request.providerOptions.gemini-generate-content.toolConfig")), cachedContent: omit(ref("request.providerOptions.gemini-generate-content.cachedContent")),
      },
    },
    agent: { method: "POST", path: "/v1beta/models/{{model}}:generateContent", contentType: "application/json", input: "conversation", body: agentBody, stream: { ...stream, allowFullResponse: true }, promptCache },
    response: { status: "succeeded", text, reasoning, error, usage: ref("response.usageMetadata"), errorPaths: ["error.code"], messagePaths: ["error.message"] },
    agentResponse: { text, reasoning, error, toolCallsPath: "candidates.0.content.parts", toolCallIdPaths: ["functionCall.id"], toolCallNamePaths: ["functionCall.name"], toolCallArgumentsPaths: ["functionCall.args"], toolCallThoughtSignaturePaths: ["thoughtSignature", "thought_signature"] },
  };
}
