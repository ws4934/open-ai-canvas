import { expect, test } from "bun:test";
import { isAgentAvatarBundleManifest } from "../src/lib/canvas/agent-appearance";
import { agentAvatarBundleFileURL } from "../src/services/api/appearance";

const manifest = (file: unknown) => ({ version: 1, width: 480, height: 432, clips: { idle: file } });

test("avatar manifests accept the ZIP API's safe root filenames", () => {
    for (const file of ["idle.mp4", "待命.mp4", "idle clip.mp4", "Idle.MP4", "角色 (1).mp4", ".mp4"]) {
        expect(isAgentAvatarBundleManifest(manifest(file))).toBe(true);
    }
});

test("avatar filenames reject paths, URL delimiters, and control characters", () => {
    for (const file of ["../idle.mp4", "./idle.mp4", "dir/idle.mp4", "/idle.mp4", "dir\\idle.mp4", "x:idle.mp4", "idle%20clip.mp4", "idle?.mp4", "idle#.mp4", "idle\u0000.mp4", "idle\n.mp4", "idle\u007f.mp4", "idle\u0085.mp4", "idle.webm", "", null]) {
        expect(isAgentAvatarBundleManifest(manifest(file))).toBe(false);
    }
});

test("avatar filenames use the same 128-byte UTF-8 limit as the ZIP API", () => {
    expect(isAgentAvatarBundleManifest(manifest(`${"a".repeat(124)}.mp4`))).toBe(true);
    expect(isAgentAvatarBundleManifest(manifest(`${"a".repeat(125)}.mp4`))).toBe(false);
    expect(isAgentAvatarBundleManifest(manifest(`${"待".repeat(41)}a.mp4`))).toBe(true);
    expect(isAgentAvatarBundleManifest(manifest(`${"待".repeat(42)}.mp4`))).toBe(false);
});

test("avatar manifests require a clip-name map rather than an array", () => {
    expect(isAgentAvatarBundleManifest({ ...manifest("idle.mp4"), clips: ["idle.mp4"] })).toBe(false);
    expect(isAgentAvatarBundleManifest({ ...manifest("idle.mp4"), clips: { 待命: "idle.mp4" } })).toBe(false);
});

test("avatar URLs encode Unicode and spaces without changing their filename", () => {
    const file = "待命 clip.mp4";
    for (const preview of [false, true]) {
        const url = agentAvatarBundleFileURL("bundle-id", file, preview, "revision-1");
        const parsed = new URL(url, "https://studio.example");
        expect(parsed.pathname).toBe(`/api/${preview ? "admin/settings" : "public"}/appearance/avatar/bundle-id/${encodeURIComponent(file)}`);
        expect(decodeURIComponent(parsed.pathname.split("/").at(-1)!)).toBe(file);
        expect(parsed.search).toBe(preview ? "" : "?v=revision-1");
    }
});

test("the built-in manifest references existing public MP4 files", async () => {
    const bundled = await Bun.file(new URL("../public/doll/avatar.json", import.meta.url)).json();
    expect(isAgentAvatarBundleManifest(bundled)).toBe(true);
    for (const file of Object.values(bundled.clips) as string[]) {
        expect(await Bun.file(new URL(`../public/doll/${file}`, import.meta.url)).exists()).toBe(true);
    }
});
