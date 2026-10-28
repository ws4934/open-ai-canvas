import { describe, expect, test } from "bun:test";
import { Window } from "happy-dom";
import { renderToStaticMarkup } from "react-dom/server";

import { generationTaskMedia } from "@/lib/generation-task-media";
import { TaskResultMedia } from "@/pages/tasks/task-result-media";

describe("创作历史完整媒体结果", () => {
    test("无后缀的本地资源与签名地址按返回顺序全部显示，不截断为 12 张", () => {
        const urls = Array.from({ length: 32 }, (_, index) => `/api/resources/result-${index}/file${index === 31 ? "?signature=test" : ""}`);
        const task = { type: "canvas_image", resultJson: JSON.stringify({ mode: "image", images: urls.map((url) => ({ dataUrl: url, url })) }) };
        expect(generationTaskMedia(task).map((media) => media.url)).toEqual(urls);
        const window = new Window();
        window.document.body.innerHTML = renderToStaticMarkup(<TaskResultMedia task={task} />);
        expect(Array.from(window.document.querySelectorAll("img")).map((image) => image.getAttribute("src"))).toEqual(urls);
        expect(window.document.body.textContent).toContain("32 项");
        expect(window.document.querySelectorAll('img[loading="lazy"]')).toHaveLength(32);
    });

    test("外部无后缀图片不漏图，同址的不同产物仍按两张展示", () => {
        const url = "https://media.example.test/generated/opaque-id";
        const task = { type: "canvas_image", resultJson: JSON.stringify({ images: [{ dataUrl: url, url }, { url }] }) };
        expect(generationTaskMedia(task)).toEqual([
            { kind: "image", url },
            { kind: "image", url },
        ]);
    });

    test("列表摘要先显示全部预览，详情不混入封面、海报或文本中的链接", () => {
        const previews = ["/api/resources/a/file", "/api/resources/b/file"];
        expect(generationTaskMedia({ type: "canvas_image", previewImages: previews })).toHaveLength(2);
        const task = {
            type: "canvas_video",
            previewKind: "video" as const,
            previewUrl: "/old.mp4",
            previewImages: previews,
            resultJson: JSON.stringify({ mode: "video", video: { dataUrl: "/api/resources/video/file", posterUrl: previews[0] }, text: "https://media.example.test/ignore.png" }),
        };
        expect(generationTaskMedia(task)).toEqual([{ kind: "video", url: "/api/resources/video/file" }]);
        const html = renderToStaticMarkup(<TaskResultMedia task={task} />);
        expect(html).toContain("<video");
        expect(html).not.toContain("<img");
        expect(generationTaskMedia({ type: "canvas_text", resultJson: JSON.stringify({ text: "https://media.example.test/ignore.png" }) })).toEqual([]);
    });

    test("单图、音频、空结果和损坏结果保持明确的展示语义", () => {
        const image = { type: "canvas_image", resultJson: JSON.stringify({ images: [{ dataUrl: "data:image/png;base64,AAAA" }] }) };
        expect(generationTaskMedia(image)).toHaveLength(1);
        const audio = { type: "canvas_audio", resultJson: JSON.stringify({ mode: "audio", audio: { dataUrl: "/api/resources/audio/file" }, audios: [{ dataUrl: "/api/resources/audio/file" }] }) };
        expect(generationTaskMedia(audio)).toHaveLength(1);
        expect(renderToStaticMarkup(<TaskResultMedia task={audio} />)).toContain("<audio");
        expect(renderToStaticMarkup(<TaskResultMedia task={{ type: "canvas_image", resultJson: "{" }} />)).toBe("");
    });
});
