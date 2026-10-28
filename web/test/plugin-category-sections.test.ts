import { readFileSync } from "node:fs";
import { describe, expect, test } from "bun:test";

const source = readFileSync(new URL("../src/pages/plugins/index.tsx", import.meta.url), "utf8");

describe("plugin category sections (#428)", () => {
    test("limits rendered sections to the selected category, not just matching plugins", () => {
        const sections = source.slice(source.indexOf("const pluginSections ="), source.indexOf("const selectCategory ="));
        expect(sections).toContain('categoryFilter === "all" || section.key === categoryFilter');
        expect(sections).toContain("[categoryFilter, filteredPlugins]");
    });

    test("omits empty sections and category navigation entries", () => {
        expect(source).toContain("section.plugins.length > 0");
        expect(source).toContain("pluginSectionMeta.filter((section) => categoryCounts[section.key] > 0)");
        expect(source).not.toContain("暂无该类型的插件");
    });

    test("uses the same visibility list for cards and counts and resets a hidden selection", () => {
        expect(source).toContain("return visiblePlugins.filter((plugin) =>");
        const counts = source.slice(source.indexOf("const categoryCounts ="), source.indexOf("const settingsPlugin ="));
        expect(counts).toContain("for (const plugin of visiblePlugins)");
        expect(counts).toContain("}, [visiblePlugins]);");
        expect(counts).toContain('if (categoryFilter !== "all" && !categoryCounts[categoryFilter]) setCategoryFilter("all")');
    });
});
