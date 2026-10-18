# 撤回本次监控/鹈鹕增强，仅保留集中设置

时间：2026-10-18 22:43 CST。用户明确要求先撤回本次渠道监控和鹈鹕修改，保留「系统设置 → 二改功能」集中管理，再提供名单供确认。基于 custom-main=c0d5729ef，新建 feature/retain-custom-feature-settings；不修改远端和发布 tag。

## 修改

- 撤回 6cc187d5b 中的 GroupMonitor service/repository/handler/API/前端组件/相关测试及 Wire 生命周期；账号测试、原鹈鹕和定时结果结构、原监控前后端页面及相关 Wire 文件等 14 个监控专属文件逐字恢复 e4fe28310。
- 混合文件仅去掉新增监控路由、用户展示限制和公开设置字段。集中设置接口、菜单启停/直链、礼遇总开关及默认菜单图标仍保留。
- 集中设置的 frontend API 类型拆到 api/customFeatures.ts；移除新监控三个 key 和对应双语文案、监控计划提示。旧数据库中这些 key 若存在，不再读写。新增回归验证旧值不影响原有渠道监控总开关、集中接口拒绝过期开关、前端不渲染或回写过期开关。
- 原设计文档替换为 docs/custom-feature-settings-inventory.md，包含 18 项开关、来源和分组/账号级配置边界。比对已合入 upstream v0.2.17=b0a02a330：5 个集中页开关键不在上游，13 个上游已有；这不是全项目二改功能总数。菜单「GPT 一键配置」来自配置数据。
- migration 246 已随旧版本发布，按迁移不可变规则保留原 SQL/checksum，避免更改历史；两个表不再由业务代码使用。不执行 DROP、不改生产设置；新安装仍可能创建兼容空表。

## 已执行验证

```sh
# backend/
go test -tags=unit ./internal/service ./internal/handler/... ./internal/server/routes -run 'Test(CustomFeatures|.*ChannelMonitor|Pelican|RunPrompt|AccountTest.*Prompt|.*PublicSettings|GetPageContent)' -count=1
go build -o /tmp/sub2api-retain-custom-feature-settings ./cmd/server

# frontend/（12 个相关文件共 103 项）
pnpm exec vitest run src/components/admin/__tests__/CustomFeaturesSettings.spec.ts src/views/admin/__tests__/SettingsView.spec.ts src/views/admin/__tests__/ChannelMonitorView.grok.spec.ts src/views/admin/__tests__/ChannelMonitorView.duplicate.spec.ts src/views/admin/__tests__/ChannelMonitorView.checkModeBadge.spec.ts src/views/user/__tests__/ChannelStatusV1View.refresh.spec.ts src/views/user/__tests__/ChannelStatusV1View.details.spec.ts src/views/user/__tests__/ChannelStatusView.mode.spec.ts src/views/user/__tests__/CustomPageView.spec.ts src/views/user/__tests__/CustomPageView.race.spec.ts src/utils/__tests__/featureFlags.spec.ts src/components/layout/__tests__/AppSidebar.spec.ts
pnpm run typecheck
pnpm exec eslint src/api/customFeatures.ts src/components/admin/CustomFeaturesSettings.vue src/components/admin/__tests__/CustomFeaturesSettings.spec.ts src/components/layout/AppSidebar.vue src/views/admin/SettingsView.vue src/i18n/locales/en/customFeatures.ts src/i18n/locales/zh/customFeatures.ts src/types/index.ts
pnpm run build

# root/
git diff --check
```

Go 相关 118 项通过（6 个包），前端 103 项通过，类型/ESLint/前端含 3 项双语检查的构建/Go 构建通过；diff 检查通过。静态核验：14 个文件等于回退基线、迁移 246 未变、18 个前端 key 与中英文文案集合一致，生产代码无 GroupMonitor 或被撤回 key 引用。

真实 CustomFeaturesSettings 组件配本地模拟 API：18 项功能开关和 1 项菜单；不再显示撤回的三项；只切换 Codex 打票后 PATCH 精确为 `{"flags":{"openai_codex_ticket_enabled":true},"menus":{}}`，成功提示和刷新正常。截图 custom-settings-after-monitor-rollback.jpg 位于本轮 visualizations 目录。所有值为模拟数据，不是线上设置；临时预览文件、进程和标签页收尾清理。

## 待确认

名单 1–5（臻享礼遇、兑换码购买、加群、邀请打票、Codex 292 打票）与自定义菜单建议保留在集中页。其余 13 项是否继续收录，等用户确认后再调整。本轮不擅自把上游已有能力删除，也不添加账号/分组配置的全局替代开关。

未提交、合并、推送、打新 tag 或部署。未连接生产数据库、未执行迁移/删表，也未发送真实供应商请求。旧发布 v0.2.17.3 不变，后续需新的代码部署才生效。
