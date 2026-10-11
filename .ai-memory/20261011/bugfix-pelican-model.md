# WebSocket 调价与鹈鹕自填模型修复

时间：2026-10-11 13:56 CST。基线：custom-main=9915395c8，功能分支 feature/ws-pricing-pelican-model。

## 已完成

1. 自动低价 WebSocket Key 的认证快照没有绑定组，导致后续 turn 无法刷新有效分组定价。按连接选定的分组 ID 读取 Lite 快照，使利润门、预留和计费使用同一 turn 的价格；重取失败或 Key 模式、平台、订阅类型改变时保留连接快照。
2. 鹈鹕测试的旧平台白名单遗漏 Command Code、Cline。改用账号现有协议路由能力判断，保留每账号一次生成请求的约束，仍拒绝多协议探测的 CN adaptive 路由。
3. 自适应账号测试的 Anthropic、Responses 分支漏传自定义 prompt。三个协议现在使用相同提示词。
4. 鹈鹕弹窗增加可选自填模型 ID，trim 后优先于下拉选择；清空时回退下拉，模型列表加载中、为空或失败时均可手填提交。限制 255 字符，保留账号模型白名单和 mapping。

## 修改文件

- backend/internal/handler/openai_gateway_handler.go、openai_gateway_handler_test.go、openai_gateway_ws_group_pricing_test.go
- backend/internal/service/api_key_service.go、pelican_test_service.go、pelican_test_service_generation_test.go
- backend/internal/service/account_test_service_cn_adaptive.go、account_test_service_cn_adaptive_test.go
- frontend/src/components/admin/account/PelicanBatchTestDialog.vue、__tests__/PelicanBatchTestDialog.spec.ts
- frontend/src/i18n/locales/zh/admin/accounts.ts、frontend/src/i18n/locales/en/admin/accounts.ts

## 验证结果

以下命令均退出 0；Go 命令在 backend、pnpm 命令在 frontend 执行。

- go test -tags=unit ./internal/handler -run 'TestOpenAIResponsesWebSocket_.*Group|TestOpenAIResponsesWebSocket_FailedKeyRefresh|TestOpenAIResponsesWebSocket_KeyMoved|TestRefreshOpenAIWSTurnBillingAPIKey|TestOpenAIWSTurnBillingAPIKeys' -count=1
- go test -tags=unit ./internal/service -run 'TestAccountTestService_(Adaptive|CommandCode|.*CN|.*OpenCode)|TestPelican' -count=1
- go test -tags=unit ./...（后端全量 unit）
- go vet ./...
- go test -tags=embed ./internal/web
- pnpm exec vitest run src/components/admin/account/__tests__/PelicanBatchTestDialog.spec.ts src/i18n/__tests__/localeKeyCompleteness.spec.ts（2 文件 / 10 项通过）
- pnpm run lint:check
- pnpm run build（含 i18n 校验、vue-tsc、Vite）
- git diff --check

浏览器使用实际弹窗组件和本地 API 请求桩，确认手填优先、trim、列表失败后可提交；没有调用真实供应商。

## 决策、风险与撤销条件

按有效分组读取替代重新选号，保持连接调度稳定；代价是自动模式后续每轮增加一次 Lite 分组读取。固定分组保持原刷新路径。若后续实现统一的分组定价缓存，可重新评估读取方式。

手填复用现有 model_id 接口，避免扩展接口或模型白名单；没有数据库结构、权限、生产配置变更。未执行真实供应商或 PostgreSQL/Redis 集成验证；后续部署可验证一轮真实生成及连接内调价。

交付范围为本地提交并合入 custom-main；未 push、部署或执行迁移。
