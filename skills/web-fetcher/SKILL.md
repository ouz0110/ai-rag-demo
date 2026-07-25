---
name: web-fetcher
description: 抓取并提取指定网页 URL 的 HTML 内容或文本信息。当用户要求读取网页、抓取网页链接、分析网络 URL 或获取网络页面内容时使用。
---

# Web Fetcher Skill指南

本 Skill 用于使用专用的 Node.js 抓取脚本提取与分析指定网页的内容。

### 🛠️ 操作步骤指南

1. **执行 JS 抓取脚本**：
   ```bash
   node skills/web-fetcher/scripts/fetch.js "<URL>"
   ```

2. **内容提取与分析**：
   从 JS 脚本返回的 HTML 或文本中提取用户关注的核心标题、正文、数据列表或关键链接信息。

3. **总结输出**：
   将提取结果整理为结构清晰的 Markdown 内容呈现在回答中。
