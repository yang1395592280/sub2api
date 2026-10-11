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

## 2026-10-11 Bug 修复与自填模型检查点

已完成：3 个 Bug 修复及鹈鹕自填模型 ID；后端全量 unit、go vet、embed 测试，前端 10 项测试、lint、build 通过；实际弹窗的手填优先及模型列表失败路径经浏览器本地请求桩验证。
相关文件：backend/internal/handler/openai_gateway_handler.go、backend/internal/service/api_key_service.go、pelican_test_service.go、account_test_service_cn_adaptive.go、frontend/src/components/admin/account/PelicanBatchTestDialog.vue 及回归测试/中英文文案；完整清单见 .ai-memory/20261011/bugfix-pelican-model.md。
未完成：本轮需求无遗留实现项；真实供应商与 PostgreSQL/Redis 集成验证未执行。未 push、部署、执行迁移。
阻塞：本地交付无已知阻塞；自动模式后续每轮多一次 Lite 分组读取，自填模型仍受账号模型配置约束。
