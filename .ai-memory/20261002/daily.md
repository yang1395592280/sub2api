# 工作日志 - 2026-10-02
session-id: 20261002-1919

## [19:20] 动作: 修复鹈鹕预览在缓存重新验证后因 CSP nonce 不一致而空白
- 文件: backend/internal/web/embed_on.go, backend/internal/web/embed_test.go
- 决策: 入口 index.html 使用每请求 CSP nonce，命中 ETag 时仍返回 200 并替换当前 nonce；保留服务端渲染缓存，避免浏览器复用旧 nonce HTML。
- 验证: `go test -tags embed ./internal/web -run TestFrontendServer_ServeIndexHTML -count=1` 通过；鹈鹕相关前端 5 个测试通过；`pnpm run typecheck` 通过；完整 embed Web 测试存在仓库现有 dist/logo.png 基线失败。

## [20:15] 动作: 准备发布 v0.2.11.2
- 文件: backend/cmd/server/VERSION 及本次鹈鹕预览修复、测试和会话记录
- 决策: 从 custom-main 当前 v0.2.11.1 提升到 v0.2.11.2，提交后创建 annotated tag，并按用户授权原子推送分支与标签到 origin。
- 验证: 后端目标测试、鹈鹕前端 5 个测试、前端 typecheck、后端二进制 `-version` 均通过；release matrix 测试的 3 个镜像脚本子测试受 macOS Bash 3.2 的 `${var,,}` 语法限制，非镜像测试通过。
