# 修复交接

已完成：4 个 Bug 根因修复；57 个测试包、12105 项顶层测试通过；专项 race、go build、git diff --check 通过。
相关文件：backend/internal/handler/gateway_inflight_reservation.go、openai_gateway_handler.go、openai_chat_completions.go；repository/billing_inflight_cache.go、billing_cache.go、api_key_repo.go；service/api_key_service.go、billing_cache_service.go、billing_inflight_reservation.go 及回归测试。
发布准备：用户已授权提交、打标签和推送 v0.2.11.1；VERSION 为 0.2.11.1，版本解析与二进制验证通过。Git 发布范围为 origin/custom-main 和 annotated tag v0.2.11.1；最终结果以远端分支与 peeled tag commit 核验为准。
未完成：Docker 可用后运行 TestAPIKeyCreateLock_ConcurrentRepositoriesRespectLimit 的 PostgreSQL 集成测试；上线时统一重启后端，避免新旧余额缓存 key 混用。发布工具 9 项测试通过，2 项镜像脚本测试受本机 Bash 3.2 限制，须在 Ubuntu/Bash 4+ 环境验证；远端构建结果需另行核验。
阻塞：Git 发布操作无已知阻塞；真实 PostgreSQL 集成验证与镜像脚本本地验证受上述环境限制。

## 2026-10-11 上游同步检查点

已完成：main 快进到 upstream/main=3a6fd1c9d（v0.2.15），custom-main 的 13 个文本冲突已整合；前端 3014 项测试和质量/构建检查、后端默认测试与相关 unit/go vet/普通及嵌入构建通过。
相关文件：13 个冲突文件、合规状态回归、嵌入静态资源测试；完整清单与命令见 .ai-memory/20261011/upstream-sync.md。
未完成：本次未授权 push 或部署；后续部署前需验证真实数据库迁移 242 与登录后核心功能。
阻塞：本地同步无已知阻塞。golangci-lint 未安装，未执行该项检查；真实 PostgreSQL/Redis 集成和浏览器验收尚未执行。
