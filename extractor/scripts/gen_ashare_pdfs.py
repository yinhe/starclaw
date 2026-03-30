"""Generate Chinese + English PDFs for A-Share Quant Guide."""
import markdown2
import os
import subprocess

base = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
edge = r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe"
if not os.path.exists(edge):
    edge = r"C:\Program Files\Microsoft\Edge\Application\msedge.exe"

css_cn = """
@page { size: A4; margin: 2cm; }
body { font-family: "Microsoft YaHei", "PingFang SC", Arial, sans-serif; line-height: 1.8; color: #1a1a1a; max-width: 800px; margin: 0 auto; padding: 40px 20px; }
h1 { color: #c41e3a; border-bottom: 3px solid #c41e3a; padding-bottom: 12px; font-size: 26px; }
h2 { color: #1a1a2e; margin-top: 32px; font-size: 20px; border-bottom: 1px solid #e0e0e0; padding-bottom: 8px; }
h3 { color: #32325d; font-size: 17px; }
table { border-collapse: collapse; width: 100%%; margin: 14px 0; font-size: 13.5px; }
th { background: #c41e3a; color: white; padding: 9px 12px; text-align: left; }
td { padding: 7px 12px; border-bottom: 1px solid #e6e6e6; }
tr:nth-child(even) { background: #fef7f7; }
blockquote { background: #fff5f5; border-left: 4px solid #c41e3a; padding: 12px 16px; margin: 14px 0; border-radius: 4px; font-style: italic; }
code { background: #f4f4f8; padding: 2px 5px; border-radius: 3px; font-size: 13px; }
pre { background: #1a1a2e; color: #e0e0e0; padding: 14px; border-radius: 8px; font-size: 12px; overflow-x: auto; }
hr { border: none; border-top: 1px solid #e0e0e0; margin: 28px 0; }
a { color: #c41e3a; text-decoration: none; }
"""

css_en = css_cn.replace("#c41e3a", "#0a2540").replace("#fff5f5", "#f0f4ff").replace("#fef7f7", "#f7f9fc").replace(
    '"Microsoft YaHei", "PingFang SC",', '"Segoe UI",')

guides = [
    {
        "md": "CHINA_A_SHARE_QUANT_GUIDE_CN.md",
        "html": "China_A_Share_Quant_Guide_CN.html",
        "pdf": "China_A_Share_Quant_Guide_CN.pdf",
        "title": "A股量化交易投资指南",
        "subtitle": "写给投资人伙伴 — StarClaw 量化交易团队",
        "footer": "由 StarClaw 量化交易团队准备 · 本文档仅供参考，不构成投资建议",
        "css": css_cn,
        "hdr_bg": "linear-gradient(135deg, #c41e3a 0%, #ff6b6b 100%)",
        "lang": "zh",
    },
    {
        "md": "CHINA_A_SHARE_QUANT_GUIDE_EN.md",
        "html": "China_A_Share_Quant_Guide_EN.html",
        "pdf": "China_A_Share_Quant_Guide_EN.pdf",
        "title": "China A-Share Quantitative Trading",
        "subtitle": "Investor Guide — StarClaw Quant Team",
        "footer": "Prepared by the StarClaw Quantitative Trading Team · For informational purposes only, not investment advice",
        "css": css_en,
        "hdr_bg": "linear-gradient(135deg, #0a2540 0%, #635bff 100%)",
        "lang": "en",
    },
]

for g in guides:
    md_path = os.path.join(base, "docs", g["md"])
    html_path = os.path.join(base, "docs", g["html"])
    pdf_path = os.path.join(base, "docs", g["pdf"])

    with open(md_path, "r", encoding="utf-8") as f:
        md_text = f.read()

    html_body = markdown2.markdown(md_text, extras=["tables", "fenced-code-blocks"])

    html_full = """<!DOCTYPE html>
<html lang="%s">
<head>
<meta charset="utf-8">
<title>%s</title>
<style>
%s
.hdr { background: %s; color: white; padding: 28px; border-radius: 12px; margin-bottom: 28px; }
.hdr h1 { color: white; border: none; margin: 0; font-size: 24px; }
.hdr p { color: rgba(255,255,255,0.8); margin: 8px 0 0; }
.ft { text-align: center; margin-top: 36px; padding-top: 18px; border-top: 1px solid #e0e0e0; color: #666; font-size: 12.5px; }
</style>
</head>
<body>
<div class="hdr">
  <h1>%s</h1>
  <p>%s</p>
</div>
%s
<div class="ft"><p>%s</p></div>
</body>
</html>""" % (g["lang"], g["title"], g["css"], g["hdr_bg"], g["title"], g["subtitle"], html_body, g["footer"])

    with open(html_path, "w", encoding="utf-8") as f:
        f.write(html_full)
    print(f"HTML: {html_path}")

    # Convert to PDF via Edge headless
    subprocess.run([
        edge, "--headless", "--disable-gpu",
        f"--print-to-pdf={pdf_path}",
        f"file:///{html_path.replace(os.sep, '/')}"
    ], capture_output=True)

    if os.path.exists(pdf_path):
        size_kb = os.path.getsize(pdf_path) / 1024
        print(f"PDF: {pdf_path} ({size_kb:.0f} KB)")
    else:
        print(f"PDF FAILED: {pdf_path}")

print("\nDone!")
