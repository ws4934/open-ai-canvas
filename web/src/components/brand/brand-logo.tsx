import { useState, type CSSProperties, type ReactNode } from "react";

import { cn } from "@/lib/utils";
import { appearanceLogoURL, isBuiltinBrand, useAppearanceStore } from "@/stores/use-appearance-store";
import type { ThemeName } from "@/stores/use-theme-store";
import { useActiveTheme } from "@/stores/canvas/use-canvas-theme-store";

type BrandLogoProps = {
    className?: string;
    fallback: ReactNode;
    alt?: string;
    theme?: ThemeName | "auto";
    inverted?: boolean;
};

export function BrandLogo({ className, fallback, alt = "", theme = "auto", inverted = false }: BrandLogoProps) {
    const appearance = useAppearanceStore((state) => state.appearance);
    const resolved = useAppearanceStore((state) => state.resolved);
    const currentTheme = useActiveTheme();
    const resolvedTheme = theme === "auto" ? currentTheme : theme;
    // Inverse surfaces swap only the built-in monochrome mark, never uploaded artwork.
    const logoTheme = inverted && !appearance.logoConfigured && isBuiltinBrand(appearance) ? (resolvedTheme === "dark" ? "light" : "dark") : resolvedTheme;
    const source = appearanceLogoURL(appearance, logoTheme);
    const [failedSource, setFailedSource] = useState<string | null>(null);
    if (!appearance.logoConfigured && (!resolved || !isBuiltinBrand(appearance))) return <>{fallback}</>;
    // A configured custom logo must never fall through to the built-in brand
    // when its file becomes unavailable. Keep its footprint neutral instead.
    if (failedSource === source) return <span className={cn("block", className)} aria-hidden="true" />;
    return (
        <img
            src={source}
            alt={alt}
            className={cn("block object-contain", className)}
            draggable={false}
            onError={(event) => {
                event.currentTarget.style.visibility = "hidden";
                setFailedSource(source);
            }}
        />
    );
}

export function BrandLogoFrame({ className, logoClassName, fallback, alt = "", theme = "auto", inverted = false }: BrandLogoProps & { logoClassName?: string }) {
    const frameEnabled = useAppearanceStore((state) => state.appearance.logoFrameEnabled);
    const unframedStyle: CSSProperties | undefined = frameEnabled
        ? undefined
        : {
              background: "transparent",
              borderColor: "transparent",
              borderRadius: 0,
              boxShadow: "none",
              color: "inherit",
          };
    return (
        <span className={cn("brand-logo-frame", className)} data-logo-frame-enabled={frameEnabled} style={unframedStyle}>
            <BrandLogo className={logoClassName} fallback={fallback} alt={alt} theme={theme} inverted={frameEnabled && inverted} />
        </span>
    );
}
