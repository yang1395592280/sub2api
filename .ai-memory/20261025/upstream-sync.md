# 2026-10-25 上游同步检查点

- 任务：同步最新 upstream/main，并以 merge 合入 custom-main。
- 同步前：main=b0a02a330（v0.2.17），custom-main=74b973836（v0.2.17.6）；工作区干净。
- 上游目标：3f1c8f36c（v0.2.18），新增 48 个提交、212 个文件变更。
- 操作：git fetch upstream；切换 main 并快进到 upstream/main；切回 custom-main 合并 main。保留两侧历史，不 rebase，不修改远程地址。
- 版本采用上游 0.2.18，本次不创建发布标签。

## 冲突与整合

9 个冲突文件已手工整合：

- backend/cmd/server/VERSION：采用 0.2.18。
- backend/internal/handler/admin/user_handler.go：同时保留二改分页与上游 adminaccess 依赖。
- backend/internal/repository/account_repo.go：BindGroups 先执行上游事务切换与受限管理员增量关联，再保留普通管理员的渠道价格分组锁定逻辑。
- backend/internal/service/admin_group.go：采用上游导出的生图默认规则，保留上游余额刷新配置及价格保护校验。
- backend/internal/service/admin_service.go：保留渠道价、充值比例及自动分组字段，接入上游 ProxyMode。
- backend/internal/service/admin_service_group_test.go：整合 Gemini/Antigravity 生图默认启用测试，保留二改价格保护校验测试。
- backend/internal/service/setting_service.go：同时保留上游管理密钥仓库、二改 Codex ticket 与自动调度缓存字段。
- frontend/src/i18n/locales/en/index.ts、frontend/src/i18n/locales/zh/index.ts：同时导出 customFeatures 与 adminAccessKeys。

额外测试适配：

- backend/internal/server/routes/custom_admin_key_access_test.go：上游新增的 5 处构造调用补齐本仓库已有的上游余额与签到依赖参数；路由测试通过。
- backend/internal/server/api_contract_test.go：默认倍率预期由历史 0.3 修正为二改现有 0.5。合并前业务常量已经为 0.5，本次未改变业务默认值；契约测试通过。

## 验证结果

- pnpm --dir frontend install --frozen-lockfile：通过，未修改依赖清单或锁文件的合并结果。
- make test-frontend：lint、typecheck 及 30 个文件共 606 项关键测试通过。
- 前端定向 Vitest：AdminAccessKeysSettings、AccountUsageCell、groupsImagePricing，3 个文件共 89 项测试通过。
- pnpm --dir frontend run build：通过，含 3 项 i18n 测试；有非阻断的 chunk 大小提示。
- go test -tags=unit ./internal/server/routes：通过。
- go test -tags=unit ./internal/server：通过。
- go vet ./...：通过。
- CGO_ENABLED=0 go build -tags=embed -trimpath -o /tmp/sub2api-sync-20261025.NIZmxF/server ./cmd/server：通过。
- 构建二进制 -version：输出 Sub2API 0.2.18。
- go test -tags=embed ./internal/web：通过。
- 全量 make test-unit 首轮：发现上述两处测试适配问题，已分别复验通过；另有 TestInflightEstimate_AccountMappingNoDBAndBoundedMemory 的进程级 HeapAlloc 差值为 8630880 字节，略超 8 MiB 阈值。未修改测试阈值或业务逻辑，单独连续重跑 3 次通过。
- 全量 make test-unit 最终复验：全部通过；service 包 217.383s，日志 backend-unit-final.log。
- git diff --check、git diff --cached --check：通过，无未解决冲突；main 与 upstream/main 均为 3f1c8f36c。本记录随 merge commit 提交，最终提交点以 Git 历史为准。
- 浏览器静态冒烟：已实际加载构建后的首页，点击登录跳转并核验表单显示；截图 /tmp/sub2api-sync-20261025.NIZmxF/frontend-smoke.png。使用隔离静态服务，未连接真实后端。
- 原始验证日志目录：/tmp/sub2api-sync-20261025.NIZmxF/。

## 部署注意与边界

- 上游 Go 工具链升级到 1.27.3，已由 Go 自动下载并用于验证；未修改系统默认 Go。
- 新增 246_enable_gemini_image_generation_groups.sql，会将现有 Gemini/Antigravity 分组的 allow_image_generation 置为 true。
- 新增 247_scoped_admin_api_keys.sql，创建按操作及资源范围授权的管理员密钥表。
- 迁移 runner 按完整 filename 跟踪和排序；二改现有 246_channel_group_monitors.sql、247_redeem_campaign_limits.sql 与上述上游文件可共存，未改名或修改已发布迁移。
- 未 push、未创建 tag、未部署、未执行数据库迁移；未运行真实 PostgreSQL/Redis 集成测试或登录后浏览器交互。SCOPED_ADMIN_TEST_DSN 未配置时，相应测试由现有测试逻辑跳过。
- golangci-lint 未安装，未运行；已执行 go vet。
- 部署前建议备份数据库，在测试环境验证新增迁移、管理员密钥权限、Gemini 生图开关，以及自动分组锁定与价格保护。
- 撤销本次合并应另行制定 revert merge 方案，不能直接覆盖二改主线。
