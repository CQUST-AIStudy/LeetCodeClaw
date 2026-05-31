from __future__ import annotations

import math
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont


ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "artifacts" / "teacher-dashboard-ui.png"


W, H = 1440, 900
SCALE = 2


def font(size: int, bold: bool = False) -> ImageFont.FreeTypeFont:
    candidates = [
        r"C:\Windows\Fonts\msyhbd.ttc" if bold else r"C:\Windows\Fonts\msyh.ttc",
        r"C:\Windows\Fonts\simhei.ttf",
        r"C:\Windows\Fonts\arial.ttf",
    ]
    for candidate in candidates:
        try:
            return ImageFont.truetype(candidate, size * SCALE)
        except OSError:
            continue
    return ImageFont.load_default()


def xy(v: int | float) -> int:
    return int(round(v * SCALE))


def box(x1: int, y1: int, x2: int, y2: int) -> tuple[int, int, int, int]:
    return (xy(x1), xy(y1), xy(x2), xy(y2))


def draw_text(
    d: ImageDraw.ImageDraw,
    pos: tuple[int, int],
    text: str,
    size: int,
    color: str,
    bold: bool = False,
    anchor: str | None = None,
) -> None:
    kwargs = {"font": font(size, bold), "fill": color}
    if anchor:
        kwargs["anchor"] = anchor
    d.text((xy(pos[0]), xy(pos[1])), text, **kwargs)


def rounded(
    d: ImageDraw.ImageDraw,
    rect: tuple[int, int, int, int],
    radius: int,
    fill: str,
    outline: str | None = None,
    width: int = 1,
) -> None:
    d.rounded_rectangle(box(*rect), radius=xy(radius), fill=fill, outline=outline, width=xy(width))


def line(d: ImageDraw.ImageDraw, points: list[tuple[int, int]], fill: str, width: int = 2) -> None:
    d.line([(xy(x), xy(y)) for x, y in points], fill=fill, width=xy(width), joint="curve")


def pill(d: ImageDraw.ImageDraw, x: int, y: int, text: str, fill: str, fg: str) -> int:
    f = font(12, True)
    bbox = d.textbbox((0, 0), text, font=f)
    tw = bbox[2] - bbox[0]
    w = int(tw / SCALE) + 24
    rounded(d, (x, y, x + w, y + 26), 13, fill)
    d.text((xy(x + 12), xy(y + 4)), text, font=f, fill=fg)
    return w


def card(d: ImageDraw.ImageDraw, x: int, y: int, w: int, h: int, title: str) -> None:
    rounded(d, (x, y, x + w, y + h), 8, "#ffffff", "#e8edf4")
    draw_text(d, (x + 20, y + 18), title, 16, "#172033", True)


def metric_card(
    d: ImageDraw.ImageDraw,
    x: int,
    y: int,
    w: int,
    h: int,
    title: str,
    value: str,
    delta: str,
    accent: str,
    mini: list[int],
) -> None:
    rounded(d, (x, y, x + w, y + h), 8, "#ffffff", "#e7edf5")
    rounded(d, (x + 18, y + 18, x + 54, y + 54), 8, accent + "24")
    d.ellipse(box(x + 28, y + 28, x + 44, y + 44), fill=accent)
    draw_text(d, (x + 68, y + 18), title, 13, "#697386")
    draw_text(d, (x + 18, y + 68), value, 28, "#172033", True)
    draw_text(d, (x + 18, y + 106), delta, 12, "#16855f", True)

    px0, py0 = x + w - 112, y + 96
    pts: list[tuple[int, int]] = []
    for i, v in enumerate(mini):
        pts.append((px0 + i * 18, py0 - v))
    line(d, pts, accent, 3)
    for px, py in pts:
        d.ellipse(box(px - 3, py - 3, px + 3, py + 3), fill="#ffffff", outline=accent, width=xy(2))


def draw_axes_chart(d: ImageDraw.ImageDraw, x: int, y: int, w: int, h: int) -> None:
    card(d, x, y, w, h, "实验完成趋势")
    plot = (x + 44, y + 62, x + w - 28, y + h - 36)
    for i in range(5):
        yy = plot[1] + i * (plot[3] - plot[1]) / 4
        line(d, [(plot[0], yy), (plot[2], yy)], "#eef3f8", 1)
    labels = ["周一", "周二", "周三", "周四", "周五", "周六"]
    values = [48, 56, 67, 63, 76, 84]
    values2 = [32, 38, 44, 52, 57, 69]
    maxv = 100
    pts: list[tuple[int, int]] = []
    pts2: list[tuple[int, int]] = []
    for i, v in enumerate(values):
        px = plot[0] + i * (plot[2] - plot[0]) / (len(values) - 1)
        py = plot[3] - (v / maxv) * (plot[3] - plot[1])
        pts.append((int(px), int(py)))
        py2 = plot[3] - (values2[i] / maxv) * (plot[3] - plot[1])
        pts2.append((int(px), int(py2)))
        draw_text(d, (int(px) - 12, plot[3] + 12), labels[i], 11, "#7a8494")
    line(d, pts2, "#f59e0b", 3)
    line(d, pts, "#2563eb", 3)
    for p in pts:
        d.ellipse(box(p[0] - 4, p[1] - 4, p[0] + 4, p[1] + 4), fill="#ffffff", outline="#2563eb", width=xy(2))
    for p in pts2:
        d.ellipse(box(p[0] - 4, p[1] - 4, p[0] + 4, p[1] + 4), fill="#ffffff", outline="#f59e0b", width=xy(2))
    pill(d, x + w - 184, y + 18, "完成率", "#dbeafe", "#1d4ed8")
    pill(d, x + w - 104, y + 18, "正确率", "#fef3c7", "#b45309")


def draw_bar_chart(d: ImageDraw.ImageDraw, x: int, y: int, w: int, h: int) -> None:
    card(d, x, y, w, h, "高频错误类型")
    labels = ["数组越界", "空指针", "递归错误", "超时", "编译错误"]
    values = [78, 64, 52, 45, 31]
    colors = ["#2563eb", "#14b8a6", "#f59e0b", "#ef4444", "#8b5cf6"]
    top = y + 66
    maxv = max(values)
    for i, (label, v) in enumerate(zip(labels, values)):
        yy = top + i * 38
        draw_text(d, (x + 24, yy + 2), label, 12, "#5c6678")
        rounded(d, (x + 104, yy + 4, x + w - 64, yy + 18), 7, "#eef3f8")
        bw = int((w - 184) * v / maxv)
        rounded(d, (x + 104, yy + 4, x + 104 + bw, yy + 18), 7, colors[i])
        draw_text(d, (x + w - 48, yy), f"{v}", 12, "#172033", True)


def draw_radar(d: ImageDraw.ImageDraw, x: int, y: int, w: int, h: int) -> None:
    card(d, x, y, w, h, "班级能力画像")
    cx, cy = x + w // 2, y + h // 2 + 18
    r = min(w, h) // 2 - 54
    axes = ["语法", "算法", "数据结构", "调试", "规范", "效率"]
    vals = [0.82, 0.68, 0.73, 0.58, 0.76, 0.64]
    for level in range(1, 5):
        rr = r * level / 4
        pts = []
        for i in range(6):
            a = -math.pi / 2 + i * math.tau / 6
            pts.append((xy(cx + math.cos(a) * rr), xy(cy + math.sin(a) * rr)))
        d.polygon(pts, outline="#dde6f0")
    value_pts = []
    for i, val in enumerate(vals):
        a = -math.pi / 2 + i * math.tau / 6
        ax = cx + math.cos(a) * r
        ay = cy + math.sin(a) * r
        line(d, [(cx, cy), (ax, ay)], "#e7edf5", 1)
        lx = cx + math.cos(a) * (r + 28)
        ly = cy + math.sin(a) * (r + 28)
        draw_text(d, (int(lx), int(ly) - 8), axes[i], 11, "#697386", anchor="mm")
        value_pts.append((xy(cx + math.cos(a) * r * val), xy(cy + math.sin(a) * r * val)))
    d.polygon(value_pts, fill="#14b8a64a", outline="#0f766e")


def draw_warning_table(d: ImageDraw.ImageDraw, x: int, y: int, w: int, h: int) -> None:
    card(d, x, y, w, h, "学习预警")
    headers = ["学生", "问题", "状态"]
    xs = [x + 22, x + 118, x + w - 104]
    for i, htxt in enumerate(headers):
        draw_text(d, (xs[i], y + 54), htxt, 12, "#8993a5", True)
    rows = [
        ("张同学", "链表题连续 6 次错误", "需干预"),
        ("李同学", "实验 3 未完成", "提醒"),
        ("王同学", "正确率下降 18%", "观察"),
        ("陈同学", "超时错误集中", "建议"),
    ]
    for i, row in enumerate(rows):
        yy = y + 82 + i * 40
        if i > 0:
            line(d, [(x + 18, yy - 12), (x + w - 18, yy - 12)], "#edf2f7", 1)
        draw_text(d, (xs[0], yy), row[0], 12, "#273043")
        draw_text(d, (xs[1], yy), row[1], 12, "#5c6678")
        colors = {
            "需干预": ("#fee2e2", "#b91c1c"),
            "提醒": ("#ffedd5", "#c2410c"),
            "观察": ("#e0f2fe", "#0369a1"),
            "建议": ("#dcfce7", "#15803d"),
        }
        bg, fg = colors[row[2]]
        pill(d, xs[2], yy - 5, row[2], bg, fg)


def draw_ai_panel(d: ImageDraw.ImageDraw, x: int, y: int, w: int, h: int) -> None:
    card(d, x, y, w, h, "AI 错误分析")
    items = [
        ("链表反转", "空指针边界处理缺失，建议复习 dummy head 与尾节点判断。", "#ef4444"),
        ("最短路径", "优先队列使用正确，但距离更新条件存在重复入队。", "#f59e0b"),
        ("递归遍历", "终止条件遗漏空子树，建议补充基本情况测试。", "#2563eb"),
    ]
    for i, (title, body, c) in enumerate(items):
        yy = y + 62 + i * 78
        rounded(d, (x + 20, yy, x + w - 20, yy + 58), 8, "#f8fafc", "#edf2f7")
        d.ellipse(box(x + 34, yy + 18, x + 50, yy + 34), fill=c)
        draw_text(d, (x + 64, yy + 10), title, 13, "#172033", True)
        draw_text(d, (x + 64, yy + 32), body, 11, "#697386")


def draw_sync_panel(d: ImageDraw.ImageDraw, x: int, y: int, w: int, h: int) -> None:
    card(d, x, y, w, h, "PTA 同步状态")
    steps = [
        ("课程", "已同步 12 门", "#14b8a6"),
        ("题目", "新增 86 题", "#2563eb"),
        ("提交", "今日 2,418 条", "#f59e0b"),
        ("代码", "异常 37 条", "#ef4444"),
    ]
    for i, (name, desc, c) in enumerate(steps):
        yy = y + 64 + i * 48
        d.ellipse(box(x + 28, yy, x + 46, yy + 18), fill=c)
        if i < len(steps) - 1:
            line(d, [(x + 37, yy + 20), (x + 37, yy + 44)], "#dbe4ee", 2)
        draw_text(d, (x + 62, yy - 2), name, 13, "#172033", True)
        draw_text(d, (x + 62, yy + 20), desc, 11, "#697386")


def draw_sidebar(d: ImageDraw.ImageDraw) -> None:
    rounded(d, (0, 0, 232, H), 0, "#102033")
    draw_text(d, (28, 28), "智能实验辅助系统", 18, "#ffffff", True)
    draw_text(d, (30, 58), "AI Teaching Lab", 11, "#9fb0c7")
    nav = [
        ("数据看板", True),
        ("课程管理", False),
        ("PTA 同步", False),
        ("AI 分析", False),
        ("能力画像", False),
        ("推荐系统", False),
        ("实验报告", False),
        ("系统设置", False),
    ]
    y = 112
    for text, active in nav:
        if active:
            rounded(d, (18, y - 8, 214, y + 34), 8, "#2563eb")
            color = "#ffffff"
        else:
            color = "#b8c4d6"
        d.ellipse(box(34, y + 4, 46, y + 16), fill="#ffffff" if active else "#5e718a")
        draw_text(d, (60, y - 1), text, 14, color, active)
        y += 48
    rounded(d, (24, 782, 208, 858), 8, "#1d3047")
    draw_text(d, (42, 800), "当前课程", 11, "#9fb0c7")
    draw_text(d, (42, 823), "数据结构实验", 15, "#ffffff", True)


def main() -> None:
    img = Image.new("RGB", (W * SCALE, H * SCALE), "#f4f7fb")
    d = ImageDraw.Draw(img)

    draw_sidebar(d)

    rounded(d, (232, 0, W, 78), 0, "#ffffff", "#e7edf5")
    draw_text(d, (264, 22), "教师端数据看板", 24, "#172033", True)
    draw_text(d, (264, 52), "实验进度、AI 分析、学习预警与推荐效果总览", 12, "#697386")
    rounded(d, (1048, 20, 1214, 52), 16, "#f1f5f9", "#e2e8f0")
    draw_text(d, (1070, 28), "搜索学生 / 课程", 12, "#8993a5")
    pill(d, 1234, 22, "DeepSeek 已连接", "#dcfce7", "#15803d")
    d.ellipse(box(1376, 20, 1412, 56), fill="#2563eb")
    draw_text(d, (1394, 37), "师", 14, "#ffffff", True, "mm")

    metric_card(d, 264, 106, 250, 142, "班级完成率", "86.4%", "较上周 +7.2%", "#2563eb", [8, 16, 14, 25, 21, 32])
    metric_card(d, 532, 106, 250, 142, "今日提交", "2,418", "新增 384 条", "#14b8a6", [13, 11, 22, 17, 30, 27])
    metric_card(d, 800, 106, 250, 142, "AI 分析次数", "316", "待处理 23 条", "#f59e0b", [6, 18, 10, 23, 16, 29])
    metric_card(d, 1068, 106, 320, 142, "学习预警", "18 人", "高风险 4 人", "#ef4444", [21, 18, 26, 14, 19, 24])

    draw_axes_chart(d, 264, 272, 520, 282)
    draw_bar_chart(d, 804, 272, 300, 282)
    draw_radar(d, 1124, 272, 264, 282)

    draw_warning_table(d, 264, 578, 520, 260)
    draw_ai_panel(d, 804, 578, 364, 260)
    draw_sync_panel(d, 1188, 578, 200, 260)

    # subtle bottom highlight
    rounded(d, (264, 858, 1388, 884), 8, "#ffffff", "#e7edf5")
    draw_text(d, (284, 864), "推荐策略：优先补弱知识点，结合 PTA 历史题库与 LeetCode 难度标签生成个性化训练。", 12, "#697386")
    pill(d, 1260, 858, "RAG 知识库正常", "#e0f2fe", "#0369a1")

    img = img.resize((W, H), Image.Resampling.LANCZOS)
    OUT.parent.mkdir(parents=True, exist_ok=True)
    img.save(OUT)
    print(OUT)


if __name__ == "__main__":
    main()
