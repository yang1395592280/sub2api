# 项目: sub2api

- 当前阶段: 鹈鹕动画预览 Bug 修复
- 活跃任务: 修复 CSP nonce 与入口 HTML 缓存重新验证冲突
- 上次会话: 入口 HTML 命中 ETag 时返回 304，浏览器复用旧 nonce HTML 导致 sandbox iframe 脚本被阻断
- 交接状态: 代码与目标测试已完成，部署后需在管理页面复验原先空白的 Canvas 动画
