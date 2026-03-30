"""Generate Chinese PDF from US Stock Quant Guide."""
import markdown2
import os

base = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
md_path = os.path.join(base, "docs", "US_STOCK_QUANT_GUIDE_CN.md")
html_path = os.path.join(base, "docs", "US_Stock_Quant_Guide_CN.html")

with open(md_path, "r", encoding="utf-8") as f:
    md_text = f.read()

html_body = markdown2.markdown(md_text, extras=["tables", "fenced-code-blocks"])

html_full = """<!DOCTYPE html>
<html lang="zh">
<head>
<meta charset="utf-8">
<title>美股量化交易入门指南</title>
<style>
  @page { size: A4; margin: 2cm; }
  body { font-family: "Microsoft YaHei", "PingFang SC", "Segoe UI", Arial, sans-serif; line-height: 1.8; color: #1a1a1a; max-width: 800px; margin: 0 auto; padding: 40px 20px; }
  h1 { color: #0a2540; border-bottom: 3px solid #635bff; padding-bottom: 12px; font-size: 26px; }
  h2 { color: #0a2540; margin-top: 32px; font-size: 20px; border-bottom: 1px solid #e0e0e0; padding-bottom: 8px; }
  h3 { color: #32325d; font-size: 17px; }
  table { border-collapse: collapse; width: 100%%; margin: 14px 0; font-size: 13.5px; }
  th { background: #0a2540; color: white; padding: 9px 12px; text-align: left; }
  td { padding: 7px 12px; border-bottom: 1px solid #e6e6e6; }
  tr:nth-child(even) { background: #f7f9fc; }
  blockquote { background: #f0f4ff; border-left: 4px solid #635bff; padding: 12px 16px; margin: 14px 0; border-radius: 4px; font-style: italic; }
  code { background: #f4f4f8; padding: 2px 5px; border-radius: 3px; font-size: 13px; }
  pre { background: #1a1a2e; color: #e0e0e0; padding: 14px; border-radius: 8px; font-size: 12.5px; overflow-x: auto; }
  hr { border: none; border-top: 1px solid #e0e0e0; margin: 28px 0; }
  a { color: #635bff; text-decoration: none; }
  .hdr { background: linear-gradient(135deg, #0a2540, #635bff); color: white; padding: 28px; border-radius: 12px; margin-bottom: 28px; }
  .hdr h1 { color: white; border: none; margin: 0; }
  .hdr p { color: #c4c8ff; margin: 8px 0 0; }
  .ft { text-align: center; margin-top: 36px; padding-top: 18px; border-top: 1px solid #e0e0e0; color: #666; font-size: 12.5px; }
</style>
</head>
<body>

<div class="hdr">
  <h1>美股量化交易入门指南</h1>
  <p>写给投资人伙伴 &mdash; StarClaw 量化交易团队</p>
</div>

%s

<div class="ft">
  <p>由 StarClaw 量化交易团队准备</p>
  <p>本文档仅供参考，不构成投资建议。</p>
</div>

</body>
</html>""" % html_body

with open(html_path, "w", encoding="utf-8") as f:
    f.write(html_full)

print(f"HTML: {html_path}")
print(f"Now converting to PDF via Edge...")
""" % html_body is already written above """
