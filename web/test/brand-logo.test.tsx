import { afterAll, afterEach, describe, expect, test } from "bun:test";
import type { ReactNode } from "react";
import { Window } from "happy-dom";

const dom = new Window({ url: "https://studio.example/" });
const originals = new Map<string, PropertyDescriptor | undefined>();
for (const [key, value] of Object.entries({ window: dom, document: dom.document, navigator: dom.navigator, HTMLElement: dom.HTMLElement, Element: dom.Element, Node: dom.Node, IS_REACT_ACT_ENVIRONMENT: true })) {
    originals.set(key, Object.getOwnPropertyDescriptor(globalThis, key));
    Object.defineProperty(globalThis, key, { configurable: true, writable: true, value });
}
const { act } = await import("react");
const { createRoot } = await import("react-dom/client");
const { BrandLogo, BrandLogoFrame } = await import("../src/components/brand/brand-logo");
const { LogoThemePreview } = await import("../src/pages/admin/settings/appearance-settings-parts");
const { applyAppearanceMetadata, brandStudioLabel, DEFAULT_PUBLIC_APPEARANCE, normalizePublicAppearance, useAppearanceStore } = await import("../src/stores/use-appearance-store");

const originalAppearance = useAppearanceStore.getState().appearance;
const originalResolved = useAppearanceStore.getState().resolved;

afterEach(() => {
    useAppearanceStore.setState({ appearance: originalAppearance, resolved: originalResolved });
});

afterAll(() => {
    dom.happyDOM.abort();
    for (const [key, value] of originals) {
        if (value) Object.defineProperty(globalThis, key, value);
        else Reflect.deleteProperty(globalThis, key);
    }
});

async function renderLogo(node: ReactNode, interact?: (host: HTMLDivElement) => void) {
    const host = document.createElement("div");
    document.body.append(host);
    const root = createRoot(host);
    try {
        await act(async () => root.render(node));
        if (interact) await act(async () => interact(host));
        return host.innerHTML;
    } finally {
        await act(async () => root.unmount());
        host.remove();
    }
}

describe("Zhiying built-in brand", () => {
    test("defaults share the new name, generated logo, and independent Agent name", async () => {
        const appearance = normalizePublicAppearance();
        expect(appearance.brandName).toBe("织影");
        expect(appearance.canvas.agentName).toBe("织影");
        expect(appearance.seoTitle).toBe("织影");
        expect(appearance.seoDescription).toContain("织影");
        expect(appearance.footerCopyright).toContain("织影");
        expect(brandStudioLabel(appearance)).toBe("ZHIYING STUDIO");
        expect(appearance.logoUrl).toBe("/logo-zhiying.png");
        expect(appearance.darkLogoUrl).toBe("/logo-zhiying-dark.png");
        for (const url of [appearance.logoUrl, appearance.darkLogoUrl]) {
            expect(await Bun.file(new URL(`../public${url}`, import.meta.url)).exists()).toBe(true);
        }
    });

    for (const theme of ["light", "dark"] as const) {
        const logoURL = theme === "dark" ? DEFAULT_PUBLIC_APPEARANCE.darkLogoUrl : DEFAULT_PUBLIC_APPEARANCE.logoUrl;
        const invertedLogoURL = theme === "dark" ? DEFAULT_PUBLIC_APPEARANCE.logoUrl : DEFAULT_PUBLIC_APPEARANCE.darkLogoUrl;

        test(`${theme} renders the built-in image without an uploaded logo`, async () => {
            useAppearanceStore.getState().setAppearance(DEFAULT_PUBLIC_APPEARANCE);
            const html = await renderLogo(<BrandLogo theme={theme} alt="织影" fallback={<span>neutral fallback</span>} />);
            expect(html).toContain(`src="${logoURL}"`);
            expect(html).toContain('alt="织影"');
            expect(html).not.toContain("neutral fallback");
        });

        test(`${theme} browser scheme picks the favicon regardless of the app theme`, () => {
            const window = new Window({ url: "https://studio.example/login", settings: { device: { prefersColorScheme: theme } } });
            // 应用主题与浏览器相反：标签栏背景由浏览器决定，favicon 必须跟随浏览器。
            window.document.documentElement.classList.toggle("dark", theme !== "dark");
            applyAppearanceMetadata(DEFAULT_PUBLIC_APPEARANCE, window.document as unknown as Document);
            expect(window.document.title).toBe("织影");
            expect(window.document.querySelector('meta[property="og:site_name"]')?.getAttribute("content")).toBe("织影");
            expect(window.document.querySelector('link[rel="icon"]')?.getAttribute("href")).toBe(logoURL);
            window.close();
        });

        test(`${theme} inverse frames select the contrasting built-in mark`, async () => {
            useAppearanceStore.getState().setAppearance(DEFAULT_PUBLIC_APPEARANCE);
            const html = await renderLogo(<BrandLogoFrame theme={theme} inverted fallback={null} />);
            expect(html).toContain(`src="${invertedLogoURL}"`);
        });

        test(`${theme} unframed marks stop inverting with the frame disabled`, async () => {
            useAppearanceStore.getState().setAppearance(normalizePublicAppearance({ logoFrameEnabled: false }));
            const html = await renderLogo(<BrandLogoFrame theme={theme} inverted fallback={null} />);
            expect(html).toContain(`src="${logoURL}"`);
        });

        for (const frameEnabled of [true, false]) {
            test(`${theme} appearance preview matches the ${frameEnabled ? "inverse frame" : "plain surface"}`, async () => {
                const html = await renderLogo(<LogoThemePreview label={theme} icon={null} src={logoURL} dark={theme === "dark"} frameEnabled={frameEnabled} />);
                expect(html).toContain(`src="${frameEnabled ? invertedLogoURL : logoURL}"`);
            });
        }
    }

    test("unresolved appearance stays neutral while the configured brand loads", async () => {
        useAppearanceStore.setState({ appearance: DEFAULT_PUBLIC_APPEARANCE, resolved: false });
        expect(await renderLogo(<BrandLogo fallback={<span>loading</span>} />)).toBe("<span>loading</span>");
    });

    test("custom brands without uploads keep their neutral fallback", async () => {
        const appearance = normalizePublicAppearance({ brandName: "本地工作室", brandSlug: "local-studio" });
        useAppearanceStore.getState().setAppearance(appearance);
        const html = await renderLogo(<BrandLogo fallback={<span>neutral fallback</span>} />);
        expect(html).toBe("<span>neutral fallback</span>");
        expect(brandStudioLabel(appearance)).toBe("LOCAL STUDIO");
    });

    test("uploaded theme logos still override the built-in image", async () => {
        useAppearanceStore.getState().setAppearance(
            normalizePublicAppearance({
                brandName: "本地工作室",
                logoConfigured: true,
                darkLogoConfigured: true,
                logoUrl: "/api/public/appearance/assets/logo?v=custom",
                darkLogoUrl: "/api/public/appearance/assets/logo-dark?v=custom",
            }),
        );
        for (const theme of ["light", "dark"] as const) {
            const html = await renderLogo(<BrandLogo theme={theme} inverted fallback={null} />);
            expect(html).toContain(theme === "dark" ? "/assets/logo-dark?v=custom" : "/assets/logo?v=custom");
            expect(html).not.toContain("logo-zhiying.png");
        }
    });

    test("appearance previews never recolor or replace custom uploads", async () => {
        const html = await renderLogo(<LogoThemePreview label="custom" icon={null} src="blob:custom-logo" dark={false} frameEnabled />);
        expect(html).toContain('src="blob:custom-logo"');
    });

    test("the built-in image respects the existing unframed option", async () => {
        useAppearanceStore.getState().setAppearance(normalizePublicAppearance({ logoFrameEnabled: false }));
        const html = await renderLogo(<BrandLogoFrame fallback={null} />);
        expect(html).toContain('data-logo-frame-enabled="false"');
        expect(html).toMatch(/background:\s*transparent/);
        expect(html).toContain('src="/logo-zhiying.png"');
    });

    test("configured images keep their rendering contract without the new built-in gate", async () => {
        useAppearanceStore.setState({ appearance: normalizePublicAppearance({ logoConfigured: true, logoUrl: "/custom-logo.png" }), resolved: false });
        expect(await renderLogo(<BrandLogo fallback={null} />)).toContain('src="/custom-logo.png"');
    });

    test("a failed custom image never reveals the built-in logo", async () => {
        useAppearanceStore.getState().setAppearance(normalizePublicAppearance({ brandName: "本地工作室", logoConfigured: true, logoUrl: "/custom-logo.png" }));
        const html = await renderLogo(<BrandLogo fallback={null} />, (host) => host.querySelector("img")!.dispatchEvent(new dom.Event("error")));
        expect(html).toContain('aria-hidden="true"');
        expect(html).not.toContain("<img");
        expect(html).not.toContain("logo-zhiying.png");
    });
});
