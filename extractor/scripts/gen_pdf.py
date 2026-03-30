"""Generate PDF from US Stock Quant Guide markdown."""
import markdown2
import os

md_path = os.path.join(os.path.dirname(__file__), "..", "docs", "US_STOCK_QUANT_GUIDE.md")
pdf_path = os.path.join(os.path.dirname(__file__), "..", "docs", "US_Stock_Quant_Trading_Guide.html")

with open(md_path, "r", encoding="utf-8") as f:
    md_text = f.read()

html_body = markdown2.markdown(md_text, extras=["tables", "fenced-code-blocks", "header-ids"])

html_full = f"""<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Getting Started with Algorithmic Trading in US Stock Markets</title>
<style>
  @page {{ size: A4; margin: 2cm; }}
  body {{ font-family: 'Segoe UI', Arial, Helvetica, sans-serif; line-height: 1.7; color: #1a1a1a; max-width: 800px; margin: 0 auto; padding: 40px 20px; }}
  h1 {{ color: #0a2540; border-bottom: 3px solid #635bff; padding-bottom: 12px; font-size: 28px; }}
  h2 {{ color: #0a2540; margin-top: 36px; font-size: 22px; border-bottom: 1px solid #e0e0e0; padding-bottom: 8px; }}
  h3 {{ color: #32325d; font-size: 18px; }}
  table {{ border-collapse: collapse; width: 100%; margin: 16px 0; font-size: 14px; }}
  th {{ background: #0a2540; color: white; padding: 10px 14px; text-align: left; }}
  td {{ padding: 8px 14px; border-bottom: 1px solid #e6e6e6; }}
  tr:nth-child(even) {{ background: #f7f9fc; }}
  blockquote {{ background: #f0f4ff; border-left: 4px solid #635bff; padding: 14px 18px; margin: 16px 0; border-radius: 4px; font-style: italic; }}
  code {{ background: #f4f4f8; padding: 2px 6px; border-radius: 3px; font-size: 13px; }}
  pre {{ background: #1a1a2e; color: #e0e0e0; padding: 16px; border-radius: 8px; overflow-x: auto; font-size: 13px; }}
  hr {{ border: none; border-top: 1px solid #e0e0e0; margin: 30px 0; }}
  .header-bar {{ background: linear-gradient(135deg, #0a2540 0%, #635bff 100%); color: white; padding: 30px; border-radius: 12px; margin-bottom: 30px; }}
  .header-bar h1 {{ color: white; border: none; margin: 0; }}
  .header-bar p {{ color: #c4c8ff; margin: 8px 0 0 0; }}
  a {{ color: #635bff; text-decoration: none; }}
  a:hover {{ text-decoration: underline; }}
  .risk-box {{ background: #fff3f3; border: 1px solid #ffcccc; border-radius: 8px; padding: 16px; margin: 16px 0; }}
  .footer {{ text-align: center; margin-top: 40px; padding-top: 20px; border-top: 1px solid #e0e0e0; color: #666; font-size: 13px; }}
</style>
</head>
<body>

<div class="header-bar">
  <h1>Getting Started with Algorithmic Trading in US Stock Markets</h1>
  <p>A guide for investors partnering with a quantitative trading team &mdash; StarClaw Quant</p>
</div>

{html_body}

<div class="footer">
  <p>Prepared by the StarClaw Quantitative Trading Team &bull; {os.popen("date /t").read().strip()}</p>
  <p>This document is for informational purposes only and does not constitute investment advice.</p>
</div>

</body>
</html>"""

with open(pdf_path, "w", encoding="utf-8") as f:
    f.write(html_full)

print(f"HTML generated: {pdf_path}")
print(f"Open in browser and Print → Save as PDF (Ctrl+P)")
