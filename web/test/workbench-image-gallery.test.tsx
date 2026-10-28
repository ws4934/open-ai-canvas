import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { Window } from "happy-dom";

import { WorkbenchImageGallery } from "@/pages/workbench/workbench-image-gallery";
import { generationTaskImageUrls as workbenchTaskImages, generationTaskPreviewUrl as workbenchTaskPreview } from "@/lib/generation-task-media";

const images = ["/api/resources/first/file", "/api/resources/second/file", "/api/resources/third/file", "/api/resources/fourth/file"];

describe("工作台一次生成多张图片", () => {
    test("列表摘要返回全部图片，而不是只使用首张封面", () => {
        const task = { type: "canvas_image", previewUrl: images[0], previewImages: images };
        expect(workbenchTaskImages(task)).toEqual(images);
        expect(workbenchTaskPreview(task)).toBe(images[0]);
    });

    test("详情保留实际返回的所有图片，url 和 dataUrl 不算两张", () => {
        const task = { type: "canvas_image", previewUrl: images[0], resultJson: JSON.stringify({ images: images.map((url) => ({ url, dataUrl: url })) }) };
        expect(workbenchTaskImages(task)).toEqual(images);
        expect(workbenchTaskImages({ ...task, resultJson: JSON.stringify({ images: [{ url: images[0] }, { dataUrl: images[1] }] }) })).toEqual(images.slice(0, 2));
        expect(workbenchTaskImages({ ...task, resultJson: JSON.stringify({ images: [{ url: images[0] }, { url: images[0] }] }) })).toHaveLength(2);
    });

    test("以实际结果计数，不按请求张数补图或截断结果", () => {
        const all = Array.from({ length: 32 }, (_, index) => `/api/resources/image-${index}/file`);
        expect(workbenchTaskImages({ type: "canvas_image", previewImages: all })).toHaveLength(32);
        expect(workbenchTaskImages({ type: "canvas_image", resultJson: "{}", previewImages: images })).toEqual(images);
        expect(workbenchTaskImages({ type: "canvas_image", resultJson: "{", previewUrl: images[0] })).toEqual([images[0]]);
    });

    test("视频仍播放视频，不把海报当多图结果", () => {
        const task = { type: "canvas_video", previewUrl: "/api/resources/video/file", resultJson: JSON.stringify({ video: { dataUrl: "/api/resources/video/file", posterUrl: images[0] } }) };
        expect(workbenchTaskImages(task)).toEqual([]);
        expect(workbenchTaskPreview(task)).toBe(task.previewUrl);
    });

    test("宫格同时显示全部图片，保留原详情入口和单张放大", () => {
        const window = new Window();
        window.document.body.innerHTML = renderToStaticMarkup(<WorkbenchImageGallery images={images} caption="图片模型" onOpen={() => {}} />);
        expect(window.document.querySelectorAll(".workbench-image-cell")).toHaveLength(4);
        expect(Array.from(window.document.querySelectorAll(".workbench-image-current")).map((image) => image.getAttribute("src"))).toEqual(images);
        expect(window.document.querySelector('button[aria-label="放大第 2 张图片"]')).not.toBeNull();
        expect(window.document.querySelector(".workbench-image-gallery.is-multi")).not.toBeNull();
        expect(window.document.body.textContent).toContain("查看详情");
        expect(window.document.querySelector(".workbench-image-controls")).toBeNull();
    });

    test("单图保持原展示，空结果不伪造图片，不增加分页或缩略图条", () => {
        const single = renderToStaticMarkup(<WorkbenchImageGallery images={images.slice(0, 1)} />);
        expect(single).not.toContain("is-multi");
        expect(single).not.toContain("workbench-image-number");
        expect(single).not.toContain("workbench-image-thumbnails");
        expect(renderToStaticMarkup(<WorkbenchImageGallery images={[]} />)).toBe("");
    });
});
