"""
生成 typoratool 的程序图标（.ico）。

思路：沿用 Typora 原来的「白卡片 + 黑 T」形象，但按尺寸重绘，解决原图在
小尺寸下糊成一团的问题：

  · 16/20/24 px  —— 去掉背景灰线、T 放大到 62%、不加描边（保证辨得清）
  · 32/40/48 px  —— T 占 57%、灰线极淡
  · 64 px 以上   —— T 占 55%、还原原图的 5 条灰线 + 细描边

源图：Typora 安装目录里的 icon_512x512@2x.png（1024×1024，RGBA）。
用法：python make_icon.py <源图路径> <输出 ico 路径>
"""
import sys
from PIL import Image, ImageDraw

SRC_DEFAULT = r"D:\Apps\Typora\resources\assets\icon\icon_512x512@2x.png"
OUT_DEFAULT = r"..\assets\typoratool.ico"

# 各尺寸的绘制参数：(T 高度占比, 灰线数, 描边 alpha)
SIZES = [16, 20, 24, 32, 40, 48, 64, 128, 256]


def params_for(size: int):
    if size <= 24:
        return 0.62, 0, 0
    if size <= 48:
        return 0.57, 5, 30
    return 0.55, 5, 40


def extract_t(src: Image.Image) -> Image.Image:
    """从源图里抠出黑色 T 字形（去掉卡片背景、灰线与叠影）。

    注意：源图右下角有一层「叠影卡片」，它半透明偏灰，用 alpha>250 把它排除，
    否则抠出来的 bbox 会大出一圈。再限制在画面中央区域，双保险。
    """
    rgba = src.convert("RGBA")
    px = rgba.load()
    w, h = rgba.size
    out = Image.new("RGBA", (w, h), (0, 0, 0, 0))
    outpx = out.load()
    minx, miny, maxx, maxy = w, h, -1, -1
    x0, x1 = int(w * 0.2), int(w * 0.8)
    y0, y1 = int(h * 0.2), int(h * 0.8)
    for y in range(y0, y1):
        for x in range(x0, x1):
            r, g, b, a = px[x, y]
            if a > 250 and (r + g + b) / 3 < 120:  # 完全不透明 + 近黑 = T
                outpx[x, y] = (17, 17, 17, 255)     # 统一成 #111
                if x < minx:
                    minx = x
                if y < miny:
                    miny = y
                if x > maxx:
                    maxx = x
                if y > maxy:
                    maxy = y
    if maxx < 0:
        raise SystemExit("源图里没找到深色 T 字形")
    print(f"T bbox: ({minx},{miny})-({maxx},{maxy}) = {maxx - minx + 1}x{maxy - miny + 1}")
    return out.crop((minx, miny, maxx + 1, maxy + 1))


def rounded_card(size: int, radius_ratio: float, border_alpha: int) -> Image.Image:
    """画一张白色圆角卡片（透明背景）。"""
    ss = 4  # 超采样，让圆角边缘平滑
    big = size * ss
    img = Image.new("RGBA", (big, big), (0, 0, 0, 0))
    d = ImageDraw.Draw(img)
    r = int(big * radius_ratio)
    d.rounded_rectangle([0, 0, big - 1, big - 1], radius=r, fill=(255, 255, 255, 255))
    if border_alpha > 0:
        d.rounded_rectangle(
            [0, 0, big - 1, big - 1], radius=r,
            outline=(140, 140, 140, border_alpha), width=max(1, ss),
        )
    return img.resize((size, size), Image.LANCZOS)


def draw_lines(card: Image.Image, count: int) -> None:
    """在卡片上画淡灰横线（模仿原图的文字行）。"""
    if count <= 0:
        return
    ss = 4
    size = card.size[0]
    big = size * ss
    layer = Image.new("RGBA", (big, big), (0, 0, 0, 0))
    d = ImageDraw.Draw(layer)
    band = card.resize((big, big), Image.LANCZOS)
    # 线的纵向分布：0.26 ~ 0.76，等距
    top, bottom = 0.27, 0.75
    lh = max(1, int(big * 0.034))
    radius = lh // 2
    for i in range(count):
        cy = top + (bottom - top) * (i / max(1, count - 1))
        y = int(big * cy)
        # 长度交替（原图就是长短不一），靠近中心的行略长
        frac = 0.58 + 0.14 * abs(1 - 2 * i / max(1, count - 1))
        half = int(big * frac / 2)
        cx = big // 2
        d.rounded_rectangle([cx - half, y - lh // 2, cx + half, y + lh // 2],
                            radius=radius, fill=(0, 0, 0, 20))
    out = Image.new("RGBA", (big, big), (0, 0, 0, 0))
    out.alpha_composite(band)
    out.alpha_composite(layer)
    card.paste(out.resize((size, size), Image.LANCZOS), (0, 0))


def draw_badge(card: Image.Image) -> None:
    """右下角画「已激活」徽章：绿圆 + 白对勾 + 白色描边圈。

    这是和原版 Typora 区分开的关键标记 —— 看到绿勾就知道是打过补丁的工具。
    """
    ss = 4
    size = card.size[0]
    big = size * ss
    layer = Image.new("RGBA", (big, big), (0, 0, 0, 0))
    d = ImageDraw.Draw(layer)
    cx = cy = int(big * 0.775)
    r = int(big * 0.175)
    # 白色外圈：把绿圆从白卡片上衬出来
    ring = max(2 * ss, int(big * 0.018))
    d.ellipse([cx - r - ring, cy - r - ring, cx + r + ring, cy + r + ring],
              fill=(255, 255, 255, 255))
    d.ellipse([cx - r, cy - r, cx + r, cy + r], fill=(22, 163, 74, 255))  # #16A34A
    # 白色对勾（三段折线）
    lw = max(2 * ss, int(big * 0.052))
    p1 = (cx - int(r * 0.46), cy + int(r * 0.06))
    p2 = (cx - int(r * 0.08), cy + int(r * 0.42))
    p3 = (cx + int(r * 0.52), cy - int(r * 0.38))
    d.line([p1, p2, p3], fill=(255, 255, 255, 255), width=lw, joint="curve")
    # 线帽圆滑
    for p in (p1, p3):
        d.ellipse([p[0] - lw // 2, p[1] - lw // 2, p[0] + lw // 2, p[1] + lw // 2],
                  fill=(255, 255, 255, 255))
    card.alpha_composite(layer.resize((size, size), Image.LANCZOS))


def compose(t_glyph: Image.Image, size: int) -> Image.Image:
    t_ratio, line_count, border_alpha = params_for(size)
    card = rounded_card(size, 0.20, border_alpha)
    draw_lines(card, line_count)

    # T 按高度占比缩放，宽度按原比例
    th = max(1, int(size * t_ratio))
    tw = max(1, int(th * t_glyph.width / t_glyph.height))
    t_img = t_glyph.resize((tw, th), Image.LANCZOS)
    x = (size - tw) // 2
    y = (size - th) // 2
    card.alpha_composite(t_img, (x, y))
    draw_badge(card)
    return card


def main() -> None:
    src_path = sys.argv[1] if len(sys.argv) > 1 else SRC_DEFAULT
    out_path = sys.argv[2] if len(sys.argv) > 2 else OUT_DEFAULT
    src = Image.open(src_path)
    t_glyph = extract_t(src)
    print(f"T 字形尺寸: {t_glyph.size}")

    frames = [compose(t_glyph, s) for s in SIZES]
    frames[-1].save(out_path, format="ICO",
                    sizes=[(s, s) for s in SIZES])
    print(f"已生成 {out_path}（{len(SIZES)} 个尺寸: {SIZES}）")

    # 独立 logo：单独一张 256px PNG（README / 发布页用），不是多尺寸拼图
    logo = out_path.replace(".ico", "-logo.png")
    frames[-1].save(logo)
    print(f"已生成独立 logo {logo}（256px 单图）")

    # 预览：白底一行 + 黑底一行，按 1:1 与放大各看一遍
    pad = 10
    w = sum(s + pad for s in SIZES) + pad
    h = max(SIZES) + pad * 2
    prev = Image.new("RGBA", (w, h), (255, 255, 255, 255))
    x = pad
    for f in frames:
        prev.alpha_composite(f, (x, pad + (max(SIZES) - f.size[0]) // 2))
        x += f.size[0] + pad
    prev.save(out_path.replace(".ico", "-preview-light.png"))
    dark = Image.new("RGBA", (w, h), (24, 24, 24, 255))
    x = pad
    for f in frames:
        dark.alpha_composite(f, (x, pad + (max(SIZES) - f.size[0]) // 2))
        x += f.size[0] + pad
    dark.save(out_path.replace(".ico", "-preview-dark.png"))
    print("预览已生成")


if __name__ == "__main__":
    main()
