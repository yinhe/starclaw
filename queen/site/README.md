# StarClaw 官网

静态营销页面，用于产品宣传和用户引导。

## 本地预览

```bash
# 任意静态服务器即可
npx serve .
# 或
python -m http.server 3000
```

## 部署

将整个 `website/` 目录部署到任意静态托管服务：
- Nginx / Caddy
- Vercel / Netlify / Cloudflare Pages
- GitHub Pages

## 结构

```
website/
  index.html    # 首页（单页，含所有内容）
  README.md     # 本文件
```
