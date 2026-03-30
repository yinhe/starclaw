import markdown
import os

src = os.path.join(os.path.dirname(__file__), "AI_TRADING_ARCHITECTURE.md")
out_html = os.path.join(os.path.dirname(__file__), "AI_TRADING_ARCHITECTURE.html")

with open(src, "r", encoding="utf-8") as f:
    md_text = f.read()

html_body = markdown.markdown(md_text, extensions=["tables", "fenced_code"])

style = """
body { font-family: 'Microsoft YaHei', 'Segoe UI', sans-serif; max-width: 900px; margin: 0 auto; padding: 40px 50px; color: #1a1a1a; font-size: 14px; line-height: 1.8; }
h1 { font-size: 26px; border-bottom: 3px solid #c0392b; padding-bottom: 10px; color: #c0392b; }
h2 { font-size: 20px; border-bottom: 1px solid #ddd; padding-bottom: 6px; margin-top: 36px; color: #2c3e50; }
h3 { font-size: 16px; margin-top: 24px; color: #34495e; }
h4 { font-size: 14px; margin-top: 18px; color: #555; }
table { border-collapse: collapse; width: 100%; margin: 12px 0; font-size: 13px; }
th, td { border: 1px solid #ccc; padding: 8px 12px; text-align: left; }
th { background: #f5f5f5; font-weight: 600; }
tr:nth-child(even) { background: #fafafa; }
code { background: #f4f4f4; padding: 2px 5px; border-radius: 3px; font-size: 13px; }
pre { background: #1e1e1e; color: #d4d4d4; padding: 16px; border-radius: 8px; overflow-x: auto; font-size: 12px; line-height: 1.5; }
pre code { background: none; color: inherit; padding: 0; }
strong { color: #c0392b; }
ul, ol { padding-left: 24px; }
li { margin: 4px 0; }
p { margin: 8px 0; }
@page { size: A4; margin: 20mm 15mm; }
@media print { body { padding: 0; } pre { font-size: 10px; } }
"""

html = f"""<!DOCTYPE html>
<html><head><meta charset="utf-8"><style>{style}</style></head>
<body>{html_body}</body></html>"""

with open(out_html, "w", encoding="utf-8") as f:
    f.write(html)

print(f"HTML: {out_html}")
