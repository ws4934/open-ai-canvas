import { expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { SegmentInputs } from "@/pages/projects/studio/segment-parts";
import { Storyboard } from "@/pages/projects/studio/storyboard";
import type { DramaSegment, DramaShot } from "@/services/api/drama";

const segment: DramaSegment = {
    id: "segment",
    revision: 1,
    index: 0,
    title: "模型不可用的剧情段",
    shotIds: ["shot"],
    durationMs: 5000,
    videoPrompt: "角色进入房间",
    generated: "角色进入房间",
    custom: false,
    promptStatus: "ready",
    videoStatus: "",
    videoModel: {},
    referenceMode: "auto",
    request: { operation: "", seconds: 0, maxVideoSeconds: 0, firstFramePending: false, references: [], referenceCapacity: 0, modelMaxImages: 0, header: "", warning: "请选择视频模型" },
};

test("模型不可用预演的空参考图数组可在分镜卡片和生成依据中正常渲染", () => {
    const shot = { id: "shot", episodeId: "episode", segmentIndex: 0, action: "角色进入房间" } as DramaShot;
    const html = renderToStaticMarkup(
        <Storyboard
            shots={[shot]}
            segments={[segment]}
            aspectRatio="9:16"
            castById={new Map()}
            defaultVideoModel={{}}
            busy={false}
            keyboardEnabled={false}
            onOpen={() => {}}
            onGenerate={() => {}}
            onReprocess={() => {}}
            onWritePrompts={() => {}}
            empty={{ title: "尚未改编", text: "等待改编", active: false }}
        />,
    );
    expect(html).toContain("模型不可用的剧情段");
    expect(html).toContain('aria-label="提示"');
    expect(html).toContain("角色进入房间");
    expect(html).toContain("待生成视频");
    for (const compact of [false, true]) {
        const inputs = renderToStaticMarkup(<SegmentInputs segment={segment} castById={new Map()} compact={compact} />);
        expect(inputs).toContain("首帧");
        expect(inputs).toContain("开场");
        expect(inputs).not.toContain("参考图未生成");
    }
});

test("正常预演的参考图编号与未就绪说明仍保留", () => {
    const referenced: DramaSegment = { ...segment, request: { ...segment.request, operation: "reference_to_video", references: [{ castId: "actor", label: "参考图1", kind: "character", name: "角色甲", sent: false }] } };
    const html = renderToStaticMarkup(<SegmentInputs segment={referenced} castById={new Map()} />);
    expect(html).toContain("图1");
    expect(html).toContain("角色甲");
    expect(html).toContain("参考图未生成");
});
