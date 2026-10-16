# 上游v0.2.17同步记录

日期：2026-10-16。用户明确要求同步最新迭代，并将最新上游合并回custom-main。

## Git范围

- 开始时工作区干净，custom-main为18a01cb9e，已包含分组用量刷新优化。
- 执行git fetch upstream；本地main由9c2cca9e6快进到upstream/main=b0a02a330（v0.2.17），纳入47个上游提交。
- 切回custom-main，执行git merge --no-commit main，完成冲突处理和验证后提交合并。合并提交的双亲及最终hash以Git历史为准。
- 不执行push、rebase、tag、发布或部署。

## 冲突和兼容处理

1. backend/cmd/server/VERSION：采用上游0.2.17，不另造二改版本或发布tag。
2. backend/internal/service/account_test_service.go：接入上游TestConnectionWithPrompt及图片事件，保留本地connect_timing；新增真实请求路径联合回归，检查实际出站提示词及SSE内容/图片/耗时/完成事件。
3. frontend/src/components/admin/account/AccountTableFilters.vue：上游恢复内联筛选与本地已有功能一致，保留本地布局和aria-label；AccountsView保留lg:w-auto/lg:flex-1以匹配本地筛选控件宽度，桌面布局保持原有排列。
4. frontend/src/components/admin/account/__tests__/AccountTableFilters.spec.ts：处理上游删除、本地修改冲突，保留本地有效交互测试。
5. backend/internal/handler/admin/account_codex_import_test.go：上游新增两处构造调用补齐本地余额/签到服务依赖参数，保持测试可编译。
6. backend/internal/service/openai_codex_ticket_test.go：覆盖可用compact兜底模型和GPT-5.5退役兜底两种情况，门票判定始终跟随实际出站模型，不改生产门控逻辑。

## 已执行验证

后端范围回归命令（backend目录）：

```sh
go test -tags=unit ./internal/service ./internal/handler/... ./internal/server/routes ./internal/payment/... ./internal/pkg/... ./internal/config ./internal/domain -count=1
```

初次36个包通过，admin新增测试构造参数不兼容，以及service旧compact断言、内存断言失败。构造参数与compact测试修正后执行以下命令，均通过；service整包用时211.670秒，最终38个有测试的后端包通过。内存断言单独连续3次和最终整包通过，保留原8MiB断言，没有修改该测试或计费实现。

```sh
go test -tags=unit ./internal/handler/admin -count=1
go test -tags=unit ./internal/service ./internal/handler/admin -run 'AccountTestService_AntigravityPromptImagesAndTiming|AntigravityConnectionTestModel|AntigravityBuildGeminiTestRequest|ExtractContentFromSSEResponse|ImportCodexSessionsSame|GroupUsageSummary|GroupHandlerUsageSummary' -count=1
go test -tags=unit ./internal/service -run '^(TestInflightEstimate_AccountMappingNoDBAndBoundedMemory|TestOpenAICodexTicketGate_CompactRequestUsesForwardOutboundModel)$' -count=3
go test -tags=unit ./internal/service -count=1
go build -o /tmp/sub2api-upstream-v0217-server ./cmd/server
```

前端命令（frontend目录，使用现有Node运行时）：

```sh
pnpm exec vitest run src/views/admin/__tests__/GroupsView src/views/admin/__tests__/AccountsView src/views/user/__tests__/KeysView src/views/user/__tests__/keyGroupOptions.spec.ts src/views/user/__tests__/StripePaymentView.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/components/admin/account/__tests__/AccountTableFilters.spec.ts src/components/admin/account/__tests__/AccountTestModal src/components/account/__tests__/AccountTestModal src/components/account/__tests__/EditAccountModal.spec.ts src/composables/__tests__/useModelWhitelist.spec.ts src/api/__tests__/admin.groups.usage-summary.spec.ts src/i18n/__tests__/localeKeyCompleteness.spec.ts
pnpm run typecheck
pnpm exec eslint src/components/account/AccountTestModal.vue src/components/account/EditAccountModal.vue src/components/admin/account/AccountTestModal.vue src/components/admin/account/AccountTableFilters.vue src/components/admin/account/__tests__/AccountTableFilters.spec.ts src/composables/__tests__/useModelWhitelist.spec.ts src/composables/useModelWhitelist.ts src/views/admin/AccountsView.vue src/views/user/PaymentView.vue src/views/user/StripePaymentView.vue src/views/user/__tests__/StripePaymentView.spec.ts
pnpm exec vitest run src/components/admin/account/__tests__/AccountTableFilters.spec.ts src/views/admin/__tests__/AccountsView
pnpm exec eslint src/views/admin/AccountsView.vue
pnpm run build
```

前端26文件294项通过，布局收尾后9文件56项复测通过，typecheck、ESLint与最终生产构建通过。测试夹具中部分既有mock缺项会打印日志，但没有失败测试；构建仍有已有Browserslist和分包体积提示。

真实生产构建配合本地API fixtures，执行node /tmp/sub2api-upstream-v0217-ui-smoke.cjs，13项交互检查通过，pageErrors为空。覆盖账号内联筛选及无障碍标签、平台与分组参数组合、分组用量普通/显式刷新/失败/恢复/轮询以及390px手机布局。缩放中的侧栏动画会影响截图，最终手机截图使用初始390px页面，不改变产品代码规避验证。截图和verification.json在本聊天visualizations目录的upstream-v0.2.17子目录。

git diff --check、git diff --cached --check、冲突标记扫描通过，git ls-files -u为空。分组用量GroupsView/API/DashboardService与同步前HEAD一致。

## 影响与限制

本次没有数据库迁移或依赖升级，没有改生产配置。上游版本包含支付退款、Gemini/Antigravity兼容、Responses created_at和GPT-5.5退役规则等变化。ChatGPT登录类账号不能路由GPT-5.5；API Key上游不受该退役规则影响，已有映射配置不会自动迁移。

没有连接生产数据库、真实支付或供应商，未执行生产部署。上线前核对GPT-5.5旧映射及真实充值退款流程；本地验证不能证明真实供应商可用性或生产负载表现。
