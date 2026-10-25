# 织影品牌与 Logo

## 使用

产品默认展示名称为「织影」，英文品牌眉题为 `ZHIYING STUDIO`。前后端默认站点名、SEO、版权、邮件品牌和画布助手名称保持一致；管理员保存的自定义站点身份和独立助手名称继续生效。

邮件配置中原内置发件人名称仍按「继承站点品牌」解释，避免改名后变成固定旧名称；明确填写的其他发件人名称不变。

默认 Logo 使用黑白 Z 形标志：宽斜带呼应胶片与「织影」首字母，上下横笔画形成画幅感。右上和左下保留贯通外部的断口，只对端部做小圆角；三个笔画不连接，不使用渐变、立体折面或阴影。

- 设计原稿：`assets/brand/zhiying-logo-source.png`，保留生图定稿供后续编辑。
- 浅色界面：`web/public/logo-zhiying.png`，黑色标志，512 × 512 透明 PNG。
- 深色界面：`web/public/logo-zhiying-dark.png`，白色标志，512 × 512 透明 PNG。

登录页、品牌首页、工作区侧栏、管理端和加载态通过 `BrandLogo` 选择对应主题资源。favicon 由外观 metadata 写入，按浏览器的 `prefers-color-scheme` 选择黑白版本，因为标签栏底色跟随浏览器而不是应用主题。工作区侧栏与后台导航都使用实心反色外框，`BrandLogoFrame inverted` 在外框启用时对调内置黑白版本；禁用外框后恢复普通主题版本。外观设置的预览遵循相同规则。登录页品牌名压在影片上，使用与右侧影片控制相同的半透明胶囊底，保证任意画面下的对比度。

未解析外观时保持中性占位；自定义品牌未上传 Logo 时，页面组件仍使用中性图标。上传的浅色、深色或单一 Logo 保持原有选择规则，不因内置标志的反色适配而替换或改色。

README 保留原名称与原 Logo 引用，因此保留 `web/public/logo.svg`。仓库地址、Go module、容器名、协议、存储键及对象存储前缀不随展示品牌改名。

## 生图方式与提示词

使用内置 `image_gen`。先基于用户选定的黑白 Z 稿微调端部，再分别导出黑、白透明版；生产 PNG 仅等比缩小到 512 × 512，保留生成图像的透明通道。

### 断口小圆角定稿

```text
Use case: precise-object-edit / logo-brand.
Edit Image 1, the black-and-white abstract Z logo.

用户要求：不要连接！保留右上和左下的断口与白色缝隙，只把断口两侧的尖角轻轻磨圆。几条黑色笔画必须仍然相互分离，不接触、不搭桥、不连成一整条。

Keep the existing disconnected topology EXACTLY. Both diagonal white slits must remain OPEN ALL THE WAY TO THE OUTSIDE of the silhouette at the upper-right and lower-left. There must be visible white daylight separating the two black ends at each opening. DO NOT close the gaps. DO NOT add any black connecting bridge. DO NOT turn the white channels into enclosed round-ended holes.

The ONLY geometry change: replace the needle-point tips facing those two open gaps with restrained small-radius rounded tips / fillets. Smooth the adjacent turns to be tangent-continuous. The two facing ends should look softly finished and intentionally separated, like two non-touching rounded ribbon ends, rather than razor points. Preserve a clear visible gap between them. Use a gentle corner radius, not big pill caps.
Keep the recognizable Z, the broad straight central diagonal band, the top and bottom arms, the near-square silhouette, the existing proportions, weight and layout. Do not otherwise redesign.
Strict flat solid black on pure white. No gray shading, no gradient, no 3D fold, no texture, no shadow, no text, no added symbols. One centered symbol only, same scale and margins as Image 1.
```

### 黑色透明版

```text
Use case: background-extraction.
Image 1 is the exact final logo. Make a transparent production asset from it. DO NOT redesign or alter any contours.
CRITICAL: KEEP BOTH WHITE SLITS OPEN TO THE OUTSIDE at the upper-right and lower-left. Keep all three black forms DISCONNECTED, with the exact little rounded ends and clear gaps in Image 1. No bridges, no joining, no closing the negative spaces.
Remove the white background and white slits to genuine transparency. Center the unchanged whole symbol on a square transparent canvas, scale the entire symbol uniformly to occupy about 84% of the canvas. Preserve its aspect ratio. Clean vector-like antialiased edges, no stray pixels, no texture, no shading.
The entire visible symbol is pure BLACK #000000, fully opaque. Outside and both open gaps are transparent. No white pixels, no tile, no text.
```

### 白色透明版

```text
Use case: background-extraction.
Image 1 is the exact final logo. Make a transparent production asset from it. DO NOT redesign or alter any contours.
CRITICAL: KEEP BOTH WHITE SLITS OPEN TO THE OUTSIDE at the upper-right and lower-left. Keep all three black forms DISCONNECTED, with the exact little rounded ends and clear gaps in Image 1. No bridges, no joining, no closing the negative spaces.
Remove the white background and white slits to genuine transparency. Center the unchanged whole symbol on a square transparent canvas, scale the entire symbol uniformly to occupy about 84% of the canvas. Preserve its aspect ratio. Clean vector-like antialiased edges, no stray pixels, no texture, no shading.
Recolor only the visible symbol to pure WHITE #FFFFFF, fully opaque. Outside and both open gaps are transparent. No black pixels, no tile, no text. Its disconnected silhouette must be identical to the input.
```
