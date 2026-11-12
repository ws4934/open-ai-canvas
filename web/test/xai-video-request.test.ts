import { expect, test } from "bun:test";
import { createOpenAIVideoTask } from "../src/services/api/video-provider-openai";
import type { ReferenceImage } from "../src/types/image";

test("xAI custom channel preserves frame roles, references and official field names", async () => {
    const calls: any[] = [];
    const deps: any = {
        transport: {
            apiUrl: (path: string) => path,
            post: async (path: string, body: unknown) => {
                calls.push({ path, body });
                return { request_id: "xai-1" };
            },
        },
        response: { unwrapVideoResponse: (value: unknown) => value, videoTaskId: (value: any) => value.request_id, readAxiosError: (error: Error) => error.message },
    };
    const config: any = { interfaceType: "xai-video", videoSeconds: "10", size: "9:16", vquality: "1080p" };
    const refs: ReferenceImage[] = ["first", "last", "character"].map((id) => ({ id, name: id, type: "image/png", dataUrl: `data:image/png;base64,${btoa(id)}` }));
    const task = await createOpenAIVideoTask(deps, config, "grok-imagine-video-1.5", "keep the character", refs, { videoStartFrameNodeId: "first", videoEndFrameNodeId: "last" });
    expect(task.id).toBe("xai-1");
    expect(calls[0]).toEqual({
        path: "/videos/generations",
        body: { model: "grok-imagine-video-1.5", prompt: "keep the character", duration: 10, aspect_ratio: "9:16", resolution: "1080p", image: { url: refs[0].dataUrl }, last_frame: { url: refs[1].dataUrl }, reference_images: [{ url: refs[2].dataUrl }] },
    });
    await createOpenAIVideoTask(deps, config, "grok-imagine-video-1.5", "reference", refs, { videoEditOperation: "reference_to_video", videoStartFrameNodeId: "first" });
    expect(calls[1].body.image).toBeUndefined();
    expect(calls[1].body.reference_images).toHaveLength(3);
    await createOpenAIVideoTask(deps, config, "grok-imagine-video-1.5", "last", [{ ...refs[1], role: "last_frame" }]);
    expect(calls[2].body.image).toBeUndefined();
    expect(calls[2].body.last_frame).toEqual({ url: refs[1].dataUrl });
    await expect(
        createOpenAIVideoTask(
            deps,
            config,
            "grok-imagine-video-1.5",
            "duplicate",
            refs.map((image) => ({ ...image, role: "first_frame" })),
        ),
    ).rejects.toThrow("每种帧最多一张");
    expect(calls).toHaveLength(3);
});
