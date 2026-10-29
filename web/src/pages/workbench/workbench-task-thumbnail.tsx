import { useEffect, useRef, useState } from "react";

import { MediaPreview } from "@/components/media-preview";

export function WorkbenchTaskThumbnail({ url, video }: { url: string; video: boolean }) {
    const element = useRef<HTMLVideoElement>(null);
    const [visible, setVisible] = useState(false);
    useEffect(() => {
        if (!video || !element.current) return;
        const observer = new IntersectionObserver(
            (entries) => {
                if (!entries.some((entry) => entry.isIntersecting)) return;
                setVisible(true);
                observer.disconnect();
            },
            { rootMargin: "200px" },
        );
        observer.observe(element.current);
        return () => observer.disconnect();
    }, [video]);
    return video ? <video ref={element} src={visible ? url : undefined} muted playsInline preload="metadata" /> : <MediaPreview src={url} kind="image" alt="" loading="lazy" className="size-full object-cover" />;
}
