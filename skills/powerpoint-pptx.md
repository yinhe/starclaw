---
description: 通过 python-pptx 或 PowerPoint COM 自动化，创建、编辑和美化本地 .pptx 演示文稿。内置政务风格与科技风格双设计系统。
category: productivity
tags: [ppt, pptx, powerpoint, python_pptx, presentation, com_automation]
---

# PowerPoint PPTX

## When to Use

需要程序化创建或修改本地 `.pptx` 文件时使用此技能。

典型场景：
- 从结构化内容（HTML/Markdown/JSON）生成演示文稿
- 用模板批量生成幻灯片
- 对已有 PPT 做版式美化、配色换肤
- 生成带环图、进度条、卡片的数据可视化幻灯片

## 两条技术路径

### 路径 A — python-pptx（跨平台）

```python
from pptx import Presentation
from pptx.util import Inches, Pt
from pptx.enum.text import PP_ALIGN
from pptx.dml.color import RGBColor
```

适合：Linux/macOS 环境、CI 流水线、简单版式。

### 路径 B — PowerPoint COM 自动化（Windows）

```python
import win32com.client as win32
app = win32.gencache.EnsureDispatch('PowerPoint.Application')
pres = app.Presentations.Add()
pres.PageSetup.SlideWidth = 960   # 16:9 pixel units
pres.PageSetup.SlideHeight = 540
slide = pres.Slides.Add(1, 12)    # ppLayoutBlank = 12
```

适合：需要高保真渲染、阴影/渐变/COM 原生图形、导出 PNG 预览。

**优先选择 COM 路径**（若运行在 Windows 且安装了 PowerPoint），可获得最佳视觉效果和 PNG 导出能力。

## 核心规则

1. **16:9 比例**：960×540 pt（COM）或 13.333×7.5 in（python-pptx）
2. **空白版式**：始终用 Blank 布局，手动放置所有元素
3. **中文字体**：显式设置 `Microsoft YaHei`，同时设 `.Font.Name`、`.Font.NameFarEast`、`.Font.NameComplexScript`
4. **本地文件**：仅创建/编辑本地 `.pptx`，不上传
5. **验证输出**：生成后检查页数、关键文字、图形渲染

## 设计系统

### 政务风格 (Gov)

| 元素 | 色值 | 说明 |
|------|------|------|
| 背景 | `#F5F7FA` | 浅灰白底 |
| 主色 | `#1A3A6B` | 藏蓝 — 标题、面板标识 |
| 强调 | `#C41E3A` | 党政红 — 标签、进度条、环图 |
| 点缀 | `#D4A843` | 金色 — 偶尔点缀 |
| 卡片 | `#FFFFFF` | 白卡 + `#D8DFE8` 边框 + 微阴影 |
| 正文 | `#444444` | 深灰 |

版式特征：
- 顶部**藏蓝条 + 红色细线**双层横幅
- 底部**藏蓝底边**
- 面板左侧**红/蓝竖条**标识层级
- 章节标题下**红色短横线**
- `pill` 标签：红色淡底 + 红色文字，或蓝色淡底 + 藏蓝文字

### 科技风格 (Tech Dark)

| 元素 | 色值 | 说明 |
|------|------|------|
| 背景 | `#0D1B2A` | 深藏青 |
| 面板 | `#162D45` | 半透面板 |
| 强调 | `#D4A843` | 金色 — 标签、进度条 |
| 光晕 | `#1E4D7B` / `#5C4A1E` | 蓝/金光晕装饰 |
| 正文 | `#E8EDF2` | 亮灰白 |
| 辅助 | `#7B8FA3` | 中灰 |

版式特征：
- 深色全屏背景 + 椭圆光晕
- 圆角面板 + 金色左侧竖条
- 导航圆点指示器
- 金色短横线装饰

## 通用排版规范

### 字号
- 页面大标题：30–44 pt，Bold
- 面板标题：17–18 pt，Bold
- 正文：9–12 pt
- 标注/标签：7.5–9 pt

### 版式节奏
- 左右双栏对称：左 440–450 pt / 右 420–440 pt
- 卡片圆角：0.04（政务）/ 0.12（科技）
- 卡片间距：16–20 pt
- 行距：1.05–1.15

## 构建脚本模式

```python
# 推荐结构
class PPTBuilder:
    def __init__(self):
        self.app = win32.gencache.EnsureDispatch('PowerPoint.Application')
        self.pres = self.app.Presentations.Add()
        # 设置尺寸...
    def add_slide(self):
        return self.pres.Slides.Add(self.pres.Slides.Count + 1, 12)

def build_cover(deck): ...
def build_slide1(deck): ...
# 每页一个函数，内容数据与构建逻辑分离

def main():
    deck = PPTBuilder()
    build_cover(deck)
    # ...
    deck.pres.SaveAs(output_path)
    deck.pres.SaveAs(export_dir, 18)  # 18 = ppSaveAsPNG
```

## 常见陷阱

- COM `Adjustments` 索引从 1 开始
- `ZOrder` 需用 `msoSendToBack=1` / `msoBringToFront=0` 常量
- `set_transparency` 要对 Fill 设 `.ForeColor.TintAndShade` 或用 XML
- 字体回退会显著改变版面
- python-pptx 的 chart category/series 长度必须精确匹配

## 范围

此技能：
- 创建和编辑本地 `.pptx` 文件
- 支持 python-pptx 和 PowerPoint COM 两条路径
- 内置政务风和科技风两套设计系统

此技能不：
- 上传文件到云端
- 自动调用外部 API
- 在未经许可的情况下访问工作目录外的文件
