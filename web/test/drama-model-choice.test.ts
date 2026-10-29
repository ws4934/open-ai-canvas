import { expect, test } from "bun:test";

import { dramaVideoOptionFits, dramaVideoRatioIssue } from "@/lib/drama/model-choice";
import type { DramaModelOption } from "@/services/api/drama";

const option: DramaModelOption = { choice: { channelId: "video", modelKey: "video" }, capability: "video", label: "视频模型", priceLabel: "", videoRatios: ["9:16", "16:9"], tailFrameChain: true, generateAudio: false };

test("短剧视频按作品画幅和衔接方式筛选，允许不出声的模型", () => {
    expect(dramaVideoOptionFits(option, "9:16", "tail_frame")).toBe(true);
    expect(dramaVideoOptionFits(option, "16:9", "parallel")).toBe(true);
    expect(dramaVideoOptionFits(option, "1:1", "parallel")).toBe(false);
    expect(dramaVideoRatioIssue(option, "1:1")).toContain("可选 9:16、16:9");
    const parallelOnly = { ...option, tailFrameChain: undefined };
    expect(dramaVideoOptionFits(parallelOnly, "9:16", "tail_frame")).toBe(false);
    expect(dramaVideoOptionFits(parallelOnly, "9:16", "parallel")).toBe(true);
});

test("模型目录未加载时保留服务端校验，明确空画幅列表不可用", () => {
    expect(dramaVideoRatioIssue(undefined, "9:16")).toBe("");
    expect(dramaVideoOptionFits({ ...option, videoRatios: [] }, "9:16", "parallel")).toBe(false);
});
