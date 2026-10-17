# 织影品牌与 Logo

## 使用

产品默认展示名称为「织影」，英文品牌眉题为 `ZHIYING STUDIO`。前后端默认站点名、SEO、版权、邮件品牌和画布助手名称保持一致；管理员保存的自定义站点身份和独立助手名称继续生效。

邮件配置中原内置发件人名称仍按「继承站点品牌」解释，避免改名后变成固定旧名称；明确填写的其他发件人名称不变。

Logo 位于 `web/public/logo-zhiying.png`，由内置生图工具生成。紫蓝光带交织成抽象的 Z，呼应「织」与影像创作；透明背景供浅色、深色主题共用。登录页、品牌首页、工作区侧栏、管理端和加载态通过 `BrandLogo` 使用同一资源，favicon 通过外观 metadata 投影设置。未解析外观时保持中性占位；自定义品牌未上传 Logo 时，页面组件仍使用中性图标。

README 保留原名称与原 Logo 引用，因此保留 `web/public/logo.svg`。仓库地址、Go module、容器名、协议、存储键及对象存储前缀不随展示品牌改名。

## 生图提示词

使用内置 `image_gen`，透明背景。先生成图形，再简化胶片齿孔和细节。最终资源仅做等比缩小，保留生成图像的透明通道。

初始提示词：

```text
Use case: logo-brand
Asset type: production logo symbol for a Chinese AI film and short-drama creation workbench named 织影 (weaving images/stories).
Primary request: Create one distinctive, beautifully simple logo mark: two broad cinematic ribbons weave over and under each other to form a compact, slightly angular open film-frame / abstract Z. The single clear crossing expresses weaving; the negative space subtly suggests a beam of projection. Strong recognisable silhouette at 20–32 px, substantial ribbon widths, clean negative space.
Style/medium: precise flat vector-like graphic rendered as a high-resolution PNG, restrained contemporary creative-software identity.
Color palette: use the existing app's violet-to-blue family (#6554df and #386fbc), with a lighter periwinkle ribbon for contrast on dark and light interfaces. At most two ribbons, gentle controlled color transitions, crisp clean edges.
Composition/framing: square 1:1 transparent canvas, single centered symbol filling roughly 86 percent of the canvas, balanced optical weight.
Text: none. The Chinese name will be typeset separately in the interface.
Constraints: genuinely transparent background; all interior cutouts transparent; no tile or bounding box, no outer frame, no mockup, no lettering, no checkerboard drawn into image, no shadow, no 3D, no texture, no sparkles, no watermark. Deliver only the final single logo symbol.
```

以初稿为编辑目标的最终提示词：

```text
Use case: logo-brand
Image 1 is the logo edit target. Refine this 织影 app logo into a much simpler flat graphic for a 20px navigation icon. Keep its compact abstract Z silhouette, weaving-film-ribbon concept, and violet / blue palette.
Change: remove ALL film sprocket holes / perforations and ALL tiny details. Use only two broad clean flat-color interwoven ribbon shapes: violet #6554df and a mid-blue #568bed. Show over-under weaving through one crisp negative-space break, NOT through shaded volume.
Flat 2D vector-logo aesthetic, bold geometric silhouette, optically balanced. Smooth and immaculate antialiased edges. No 3D shading, no highlights, no gradients, no texture, no shadow, no background color, no outer tile, no text. Genuinely transparent square canvas, centered mark with 8 percent padding. All negative space truly transparent. Deliver one final icon only.
```
