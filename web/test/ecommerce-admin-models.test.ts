import { expect, test } from "bun:test";
import { defaultModelCapabilityConfig } from "../src/lib/model-capabilities";
import { modelQuoteRequest } from "../src/lib/model-pricing";
import { systemChannelModelChannels } from "../src/lib/user-session";
import { matchCommerceAdminModel } from "../src/pages/ecommerce/admin-models";
import { quoteCommerce } from "../src/pages/ecommerce/workflow";
import { emptyEcommerceDraft } from "../src/services/api/ecommerce";
import type { PublicChannelModel } from "../src/services/api/logical-models";
import { apiClient } from "../src/services/api/request";
import { defaultConfig, selectableModelsByCapability } from "../src/stores/use-config-store";

function catalogModel(capability: "text" | "image" | "video" | "audio", available = true): PublicChannelModel {
    const profile = defaultModelCapabilityConfig();
    profile.text!.references.maxImages = 6;
    return {
        id: `model-${capability}`,
        modelKey: `${capability}-model`,
        displayName: capability,
        icon: "",
        capability,
        protocol: capability === "text" ? "openai-response" : "openai-image",
        available,
        priceLabel: "1 积分",
        pricingMode: "fixed_request",
        priceTiers: [{ id: `tier-${capability}`, resolution: "*", videoSeconds: 0, billingMode: "fixed_request", unitPriceMicrocredits: 1_000_000, inputTokenPriceMicrocredits: 0, outputTokenPriceMicrocredits: 0, cachedTokenPriceMicrocredits: 0 }],
        capabilityConfig: profile,
    };
}

function config() {
    return {
        ...defaultConfig,
        channels: systemChannelModelChannels([
            { id: "channel-a", name: "A", displayName: "A", models: [catalogModel("text"), catalogModel("image"), catalogModel("video"), catalogModel("audio")] },
            { id: "channel-b", name: "B", displayName: "B", models: [{ ...catalogModel("text"), id: "other-text" }] },
        ]),
    };
}

test("direct system models of every capability quote without a logical ID", () => {
    const value = config();
    for (const capability of ["text", "image", "video", "audio"] as const) {
        expect(modelQuoteRequest(value, `channel-a::${capability}-model`, capability)).toMatchObject({ channelId: "channel-a", modelKey: `${capability}-model`, intent: { capability } });
    }
});

test("ecommerce matches the saved channel and model rather than a same-named model", () => {
    const value = config();
    const choices = selectableModelsByCapability(value, "text");
    const saved = { channelModelId: "other-text", channelId: "channel-b", modelKey: "text-model" };
    expect(matchCommerceAdminModel(value, choices, saved)).toBe("channel-b::text-model");
    expect(
        matchCommerceAdminModel(
            value,
            choices.filter((item) => !item.startsWith("channel-b::")),
            saved,
        ),
    ).toBe("");
});

test("logical routing remains intact and missing, custom or mismatched models cannot quote", () => {
    const value = config();
    value.channels[0].modelCosts![0].logicalModelId = "logical-text";
    expect(modelQuoteRequest(value, "channel-a::text-model", "text")).toMatchObject({ logicalModelID: "logical-text" });
    expect(modelQuoteRequest(value, "channel-a::text-model", "text")?.channelId).toBeUndefined();
    expect(matchCommerceAdminModel(value, ["channel-a::text-model"], { logicalModelId: "logical-text" })).toBe("channel-a::text-model");
    expect(modelQuoteRequest(value, "channel-a::image-model", "text")).toBeUndefined();
    expect(modelQuoteRequest(value, "channel-a::missing", "text")).toBeUndefined();
    value.channels[0].scope = "user";
    expect(modelQuoteRequest(value, "channel-a::text-model", "text")).toBeUndefined();
});

test("ecommerce planning and image quotes both reach the channel quote API", async () => {
    const original = apiClient.request;
    const calls: any[] = [];
    apiClient.request = (async (request: any) => {
        calls.push(request);
        return { data: { code: 0, data: { quote: { amountMicrocredits: request.data.intent.capability === "text" ? 2_000_000 : 1_000_000 } }, msg: "ok" }, status: 200 };
    }) as typeof apiClient.request;
    try {
        const draft = { ...emptyEcommerceDraft(), agentModel: "channel-a::text-model", imageModel: "channel-a::image-model", references: [{ resourceId: "product", role: "primary" as const }] };
        expect(await quoteCommerce(config(), draft, 3)).toEqual({ image: 1, agent: 2, total: 5 });
        expect(calls).toHaveLength(2);
        for (const call of calls) {
            expect(call).toMatchObject({ url: "/model-catalog/quote", data: { channelId: "channel-a", intent: { inputs: { image: 1 } } } });
        }
        expect(calls.map((call) => call.data.modelKey).sort()).toEqual(["image-model", "text-model"]);
    } finally {
        apiClient.request = original;
    }
});
