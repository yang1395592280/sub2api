# 2026-10-11 上游同步检查点

- 目标：将 main 快进到 upstream/main，再以 merge 合入 custom-main；不 push、不部署、不执行数据库迁移。
- 同步前：main=3f1a2ea0a（v0.2.14），custom-main=d19cc71c6（v0.2.14.1），工作区干净。
- 上游目标：3a6fd1c9d（v0.2.15），146 个新增提交，278 个文件变更。
- 回溯点：原 custom-main 提交 d19cc71c6 保留在 Git 历史；不会重写历史。恢复代码应另行使用 revert，未授权任何破坏性回退。
- 影响面：13 个冲突文件，涉及版本、分组校验、WebSocket 计费、账号测试、限额平台清单、用量展示、Codex 配置测试、i18n、合规状态请求。
- 整合原则：保留自托管账号池、自动低价分组、批量测试提示词、Grok 上游余额、账号备注和首字延迟展示、远程模型发现及请求去重；接入上游新平台和竞态修复。
- 并发处理：合规状态以会话版本隔离旧请求，仍保留同会话去重；WebSocket 利润门、额度预留、计费共用每轮有效分组快照。
- 真实引用已核对：group_platform validator、testModelRoutedResponsesConnection、openAIWSTurnBillingAPIKeys、normalizePlatformQuotasMap、listPlatformIds、fetchStatus/reset。无现成项目图谱，按源码搜索复核调用点。
- 验证路径：冲突标记与 diff 检查；前端 lint/typecheck/build 和测试；后端 go test 与 unit 核心测试、go vet、构建；核对上游祖先关系及最终工作区。
- 部署注意：上游要求 Go 1.27.2，工具链自动下载已成功；迁移 242 移除两处平台 CHECK，改由应用校验，尚未在数据库执行或验证回滚。
- 检查点：13 个文本冲突已手工整合并暂存，代码整合与验证已完成。本记录随 merge commit 提交，最终提交点以 Git 历史为准；原 custom-main 和 main 历史均保留。

## 冲突文件

- backend/cmd/server/VERSION：使用上游版本 0.2.15，本次不创建发布标签。
- backend/internal/handler/admin/group_handler.go：保留 GroupRole/SelfHostedPoolGroupID，采用清单驱动的 group_platform 校验。
- backend/internal/handler/openai_gateway_handler.go：保留有效路由分组与额度预留，将上游每轮计费快照接入利润门、额度预留和用量记录。
- backend/internal/service/account_test_service.go：采用模型路由供应商统一测试入口，保留自定义 prompt 和调度结果上报。
- backend/internal/service/openai_gateway_response_handling.go：空完成事件故障切换支持 Grok，保留原有客户端输出判定。
- frontend/src/api/__tests__/settings.authSourceDefaults.spec.ts：限额归一化覆盖当前全部平台。
- frontend/src/api/admin/settings.ts：平台限额从平台清单生成。
- frontend/src/components/account/AccountUsageCell.vue：保留 Grok API Key 上游余额展示，支持新增多协议供应商。
- frontend/src/components/admin/usage/__tests__/UsageTable.spec.ts：保留账号备注、自动低价和首字延迟回归，整合 TPS 与本地化长上下文测试，删除重复 latency 测试 slot。
- frontend/src/components/keys/__tests__/UseKeyModal.spec.ts：按 features 分段断言，并保留 api_key_model_discovery 覆盖。
- frontend/src/i18n/locales/en/dashboard.ts：保留延迟阶段词条，增加输出 TPS 词条。
- frontend/src/i18n/locales/zh/dashboard.ts：保留延迟阶段词条，增加输出 TPS 词条。
- frontend/src/stores/adminCompliance.ts：保留同会话请求去重，用 stateVersion 阻止旧会话回写及清理新会话请求。

## 验证结果

- `git diff --check` / `git diff --cached --check`：通过；无未解决冲突标记。
- `pnpm --dir frontend run lint:check`：通过。
- `pnpm --dir frontend run typecheck`：通过。
- `pnpm --dir frontend run build`：通过，包括 3 项 i18n 测试；存在非阻断的 Browserslist 数据过期、动态导入和 chunk 大小提示。
- `pnpm --dir frontend run test:run`：396 个测试文件、3014 项测试通过；其中补充 3 项合规状态请求去重/跨会话回归。
- 在 backend 执行 `go test ./...`：全部通过，service 包耗时 145.274s，包含 WebSocket 每轮计费刷新回归。
- 在 backend 执行 `go test -tags=unit ./internal/service -run 'AccountTest|AccountConnection|OpenCode|CNProvider|EmptyCompleted|ValidateCreateParams'`：通过，service 包耗时 3.171s。
- 在 backend 执行 `go vet ./...`：通过。
- 在 backend 执行 `CGO_ENABLED=0 go build -trimpath -o /tmp/sub2api-upstream-sync-20261011-server ./cmd/server`：通过。
- 在 backend 执行 `go test -tags=embed ./internal/web`：通过，耗时 0.676s。
- 在 backend 执行 `CGO_ENABLED=0 go build -tags=embed -o /tmp/sub2api-upstream-sync-20261011-server-embed ./cmd/server`：通过。
- 嵌入测试初次发现 2 项用例仍请求已不存在的 logo.png；已按真实 logo.svg 更新 MIME 和路径，并增加响应字节内容断言，复验通过。未修改静态资源服务逻辑。
- 迁移按完整 filename 跟踪并排序，现有自定义 `242_account_pelican_test_usage.sql` 与上游 `242_drop_platform_check_constraints.sql` 可共存；未改已发布迁移。

## 边界与后续

- 未 push、未部署、未运行真实数据库迁移，未创建发布标签；未执行带 integration 标签的 PostgreSQL/Redis 集成测试及登录后浏览器交互。
- 本机 PATH 无 golangci-lint，未运行该检查；后端已执行 go vet 和相关测试。
- 部署前建议备份数据库，在测试环境验证迁移 242、新平台分组创建、自动低价/自托管分组、WebSocket 连续请求与管理员调价、账号批量测试和用量显示。
- 本次未新增架构或分支策略；若需要撤销，另行制定 revert merge 方案，禁止直接覆盖二改主线。
