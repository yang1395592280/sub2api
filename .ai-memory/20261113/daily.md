# 工作日志 - 2026-11-13
session-id: 20261113-1023

## [10:23] 功能实现：顶栏签到与上游每日消费
- 分支 feature/checkin-dialog-upstream-daily-spend；基线99d8fb838；无提交、合并、推送或部署。
- 变更：顶栏入口与签到弹窗、独立中奖/未中奖翻牌、按上游Key归属的当日消费与充值折算成本、账号用户合计；初始化 watch 错误回归修复。文件及业务口径见 checkin-upstream-daily-plan.md 与 ../../docs/upstream-daily-usage-and-checkin.md（仓库根 docs）。
- 决策：按北京时间实际扣费，保留原始值与折算值；New API 按 token_id 完整分页，缺失/超限不伪零；签到活动规则与后端钱包不变。
- 验证：Go service short/定向48/race/build/vet；前端相关38项/最终签到6项、ESLint、正式build；本地模拟API浏览器桌面/390px和两套结果通过。
- 限制：当前只保存当日快照，真实凭据权限和定制上游未联调。临时服务器缓存与列表fixture问题已定位修正，产品未为适配fixture修改。

## [11:06] Git 集成授权：提交并合入 custom-main
- 用户明确要求“合并分支”，授权提交本次已验证修改并合入 custom-main。
- 合并前两个分支均为99d8fb838，无分叉，工作区内容与本次已验证交付一致；提交后采用 fast-forward 合并，不 rebase、不推送、不打 tag、不部署。
- 复用前端38项/最后签到6项、Go定向48项/race/build/vet与桌面/390px模拟API验收；git diff --check通过。实际提交点与合并结果以 Git 历史及本次执行输出为准。

## [11:14] 发布授权：v0.2.21.4
- 用户明确要求打 tag、提交并推送；远端无同名 tag，origin/custom-main仍为99d8fb838。
- 版本文件更新为0.2.21.4，发布记录写入 release-v0.2.21.4.md；拟用 annotated tag 与 git push --atomic 推送 custom-main 和 tag，不 force、不部署。

## [11:43] 使用记录详细扣款精度
- 分支 feature/usage-wallet-precision：共用 UsageTable 的充值／福利扣款和来源提示复用 formatCostFixed 显示8位，金额概览仍2位。同步 docs/welfare-balance-checkin.md。
- 已验证前端相关64项、组件ESLint、typecheck、正式build；本地模拟API浏览器管理端／用户端核对微额、零金额、三种福利来源和未分类记录，用户端7项显示断言通过。测试fixture的groups/available返回形态修正后刷新无新增console error，产品未适配错误fixture。
- 比例以实码wallet_policy.go和wallet.go为准，默认2:1；线上可覆盖，未查询或更改实际策略。收益分析仅作假设估算，未推送／部署。
- 后端执行 go test -tags=unit ./internal/service -run '^(TestWalletConfiguredRatioAndDebtLimits|TestWalletSplitInvariants|TestWalletUsagePolicyValidationAndDefaults)$' -count=1 通过（含子用例共12项）；最终git diff --check通过。已完成2条主线，业务文件为UsageTable.vue和说明文档；无未完成实现项，线上比例与实际财务数据未验证。

## Git 集成授权：使用记录详细扣款精度
- 用户明确要求“合并分支”，授权提交本轮已验证修改并合入 custom-main。两个分支当前同为 fd78c3971，未分叉；工作区仅包含本轮代码、文档与会话记录。
- 复用前端64项、ESLint/typecheck/build、后端比例相关测试与本地模拟API浏览器验收；本轮git diff --check通过。提交后采用fast-forward合入，最终提交点与结果以Git核验为准；不推送或部署。
