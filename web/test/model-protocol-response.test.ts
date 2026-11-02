import { describe, expect, test } from "bun:test";
import { modelProtocolResponseLabel } from "../src/lib/model-protocols";
import { registerPlugin, getRegisteredPlugin, unregisterPlugin } from "../src/lib/plugins/plugin-registry";
import type { RegisteredPlugin } from "../src/lib/plugins/plugin-types";

describe("protocol response capabilities", () => {
    test("distinguishes complete responses, streaming and polling", () => {
        expect(modelProtocolResponseLabel({})).toBe("完整响应");
        expect(modelProtocolResponseLabel({ streaming: true })).toBe("完整响应 / 流式响应");
        expect(modelProtocolResponseLabel({ poll: "GET /tasks/{id}", streaming: true })).toBe("异步轮询");
    });
    test("registers the v3 server protocol contract without changing other contributions", () => {
        const plugin: RegisteredPlugin = {
            manifest: {
                apiVersion: "yingce.plugin/v3",
                id: "test-stream-contract",
                name: "Test",
                version: "1.0.0",
                author: "test",
                description: "test",
                permissions: [],
                surfaces: ["settings"],
                trusted: false,
                contributes: { providers: [{ id: "test-stream", label: "Test", capabilities: ["text"], scopes: ["agent"], create: { method: "POST", path: "/model" }, response: {} }] },
            },
        };
        try {
            registerPlugin(plugin);
            expect(getRegisteredPlugin(plugin.manifest.id)?.manifest.apiVersion).toBe("yingce.plugin/v3");
        } finally {
            unregisterPlugin(plugin.manifest.id);
        }
    });
});
