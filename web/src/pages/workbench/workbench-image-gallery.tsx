// 保持原作品区外框，单图铺满、多图宫格；放大预览复用 AntD，不新增页内分页布局。
import { Button, Image as AntImage } from "antd";
import { Expand } from "lucide-react";
import { useState } from "react";

import { MediaPreview } from "@/components/media-preview";

export function WorkbenchImageGallery({ images, caption, onOpen }: { images: string[]; caption?: string; onOpen?: () => void }) {
    const [previewIndex, setPreviewIndex] = useState<number | null>(null);
    if (!images.length) return null;
    return (
        <div className={`workbench-image-gallery${images.length > 1 ? " is-multi" : ""}`} role="group" aria-label={`生成图片，共 ${images.length} 张`}>
            <div className="workbench-image-grid">
                {images.map((url, index) => (
                    <button key={`${index}:${url}`} type="button" className="workbench-image-cell" aria-label={`放大第 ${index + 1} 张图片`} onClick={() => setPreviewIndex(index)}>
                        <MediaPreview src={url} kind="image" alt={`生成结果，第 ${index + 1} 张，共 ${images.length} 张`} className="workbench-image-current" fallbackClassName="workbench-image-current" fallbackLabel={`第 ${index + 1} 张图片暂不可用`} />
                        {images.length > 1 ? (
                            <span className="workbench-image-number" aria-hidden="true">
                                {index + 1} / {images.length}
                            </span>
                        ) : null}
                    </button>
                ))}
            </div>
            {caption || onOpen ? (
                <div className="workbench-result-caption">
                    <span>{caption}</span>
                    {onOpen ? (
                        <Button size="small" icon={<Expand className="size-3.5" />} onClick={onOpen}>
                            查看详情
                        </Button>
                    ) : null}
                </div>
            ) : null}
            <AntImage.PreviewGroup items={images} preview={{ open: previewIndex !== null, current: previewIndex ?? 0, onChange: setPreviewIndex, onOpenChange: (open) => !open && setPreviewIndex(null) }} />
        </div>
    );
}
