import { describe, expect, test } from "bun:test";
import { CONTENT_MODERATION_MESSAGE, generationErrorMessage, generationFailureMetadata } from "../src/lib/generation-error";

describe("safe upstream error details", () => {
    const detail = "非常抱歉，生成的图片可能违反了关于裸露、色情或情色内容的防护限制。请重试或修改提示语。";
    test("turns an explicit safety refusal into a readable reason without echoing upstream text", () => {
        const message = `模型服务拒绝了请求，请检查模型和参数；上游：${detail}`;
        expect(generationErrorMessage(new Error(message))).toBe(CONTENT_MODERATION_MESSAGE);
    });
    test("does not replace a safe upstream message with a generic HTTP hint", () => {
        const message = `模型服务暂时不可用（HTTP 500）；上游：${detail}`;
        expect(generationErrorMessage(message)).toBe(CONTENT_MODERATION_MESSAGE);
        expect(generationErrorMessage("模型服务暂时不可用（HTTP 500）")).toBe("模型服务暂时不可用，请稍后再试。");
    });
    test("keeps details when moderation metadata is stored", () => {
        const message = `内容审核未通过；上游：${detail}`;
        const metadata = generationFailureMetadata(message);
        expect(metadata.errorDetails).toBe(CONTENT_MODERATION_MESSAGE);
        expect(metadata.generationErrorCode).toBe("sensitive_words_detected");
    });
    test("gives actionable hints for known upstream failures", () => {
        expect(generationErrorMessage(`模型服务暂时不可用（HTTP 503）；上游：model_not_found request id abc`)).toContain("更换模型");
        expect(generationErrorMessage(`模型服务拒绝请求；上游：image file too large 25MB`)).toContain("压缩图片");
    });
    test("upstream marker does not bypass URL hiding", () => {
        expect(generationErrorMessage("模型服务请求失败；上游：https://private.test")).toBe("模型服务请求失败");
    });

    test("recognizes a Grok content policy rejection before dropping upstream text", () => {
        const message = "模型服务拒绝了请求，请检查模型和参数；上游：内容未通过上游安全审核，请修改提示词或参考素材后重试。";
        expect(generationErrorMessage(message)).toBe(CONTENT_MODERATION_MESSAGE);
        expect(generationErrorMessage({ error: { code: "content_policy_violation", message: "内容未通过上游安全审核，请修改提示词或参考素材后重试。" } })).toBe(CONTENT_MODERATION_MESSAGE);
        expect(generationFailureMetadata(message).generationErrorCode).toBe("sensitive_words_detected");
    });

    test("keeps a readable parameter reason and does not erase the backend projection", () => {
        const projected = "模型服务拒绝请求；参考图片只能使用 PNG 格式，请转换后重试。";
        expect(generationErrorMessage("模型服务拒绝请求；上游：参考图片只能使用 PNG 格式，请转换后重试。")).toBe(projected);
        expect(generationErrorMessage(projected)).toBe(projected);
        const unavailable = "模型服务暂时不可用（HTTP 503）；当前模型暂无可用渠道，请更换模型或联系管理员。";
        expect(generationErrorMessage(unavailable)).toBe(unavailable);
    });

    test("readable upstream details still exclude credentials and raw diagnostics", () => {
        for (const detail of ["鉴权失败，api_key=private-key", "服务失败，request_id=private-id", "internal stack trace 私密诊断", "<html>私密网关</html>"]) {
            expect(generationErrorMessage(`模型服务拒绝请求；上游：${detail}`)).toBe("模型服务拒绝请求");
        }
    });
});
