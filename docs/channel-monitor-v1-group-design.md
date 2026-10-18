# 二改开关与 V1 分组账号监控

用户已于 2026-10-18 确认实施。功能分支：feature/custom-controls-active-monitor。

## 范围与验收

1. 系统设置新增二改功能页，复用现有开关；新增礼遇真正总开关、监控用户展示及鹈鹕开关；自定义菜单逐项启停，站内直链同样受控。局部保存不覆盖其他设置。
2. V1 渠道监控支持批量选择真实分组、模型、探活/鹈鹕频率、提示词和每日请求上限。新计划默认暂停；旧外部监控配置保持原状。
3. 每组每轮由现有调度选一个适用账号，固定该账号依次执行所选模型；失败不切换账号。复用 AccountTestService 的账号协议、凭证、代理与模型映射，无需手动创建站内 Key。界面准确标记为账号抽测，不声称覆盖用户鉴权、计费的端到端链路。直接消耗上游额度，Token 单独记录。
4. 鹈鹕位于渠道卡片弹窗，提供历史/模型切换、沙箱预览、人工评估。管理员查看账号快照、消息/响应 ID、上游请求 ID、站内追踪 ID 和可复制反馈摘要；未返回的上游 ID 不伪造。
5. 用户列表/详情/历史/预览均按真实分组权限过滤；DTO 不含账号及内部标识，手动检测仅管理员。
6. 持久化租约控制手动与定时任务重入，多实例安全；独立并发限制、超时、过期恢复及历史清理；关闭开关停止领取新任务，进行中收尾。

## 实施与验证顺序

- [x] 配置与总开关：SettingService/SettingHandler、DTO/自定义页面、SettingsView、菜单。21 个独立开关，已有配置复用旧 key；菜单只修改 enabled，保留未知字段；礼遇关闭时状态返回 visible=false，业务接口不进入 handler。
- [x] 追踪：HTTPUpstream 在账号测试上下文中观测响应头、JSON/SSE 消息 ID、首内容时间；ScheduledTestResult 增量字段。其他请求无观测副作用。
- [x] 数据：migration 246 新增 channel_group_monitors、channel_group_monitor_runs；独立 repository/service/handler，复用现有调度与账号测试。迁移已在隔离 PostgreSQL 18.1 测试容器执行。
- [x] 任务：数据库原子领取、租约 token 校验、请求预算、错峰、超时与有界历史；真实数据库并发、预算与恢复已验证。
- [x] 前端：V1 管理页增加分组卡片/批量配置；用户渠道页整合分组卡片；共享鹈鹕历史弹窗。
- [x] 本地验证：相关 Go 单元、HTTP 权限回归、前端组件/配置回归、typecheck/lint/build；真实应用路由和组件配合本地模拟 API 完成浏览器交互验证。
- [x] PostgreSQL 集成验证：启动现有本地 Docker 后，2 项集成测试通过；初次因 Docker 不可用而跳过的结果不计为通过。测试容器已自动清理。
- [ ] 真实上游联调：本轮不发付费请求、不部署，留待测试环境执行。

## 实现边界与运行参数

- 仅普通分组；复合路由、自托管池不能独立配置。选号固定为该分组的账号来源，不沿 Claude Code 限制回退到其他分组，也不通过普通 OpenAI 分组的池回退路由取号。运行前重检分组类型，防止保存计划后改为复合组。
- 一轮固定一个账号，依次测试最多 5 个文本模型；不支持的模型、媒体模型映射、空内容和无完整 HTML 均记录失败，不换号掩盖失败。鹈鹕生成成功与人工质量评估分别保存。
- 批量最多 50 组；探活 60–86400 秒；鹈鹕 600–604800 秒；每日请求上限 1–10000 且不少于模型数量。手动和定时共用 UTC 日预算，领取时按模型数预留，未真正发送的请求也不退还预算。
- 调度每 15 秒运行；单实例同时最多 2 轮；每组数据库租约 20 分钟、整轮超时 17 分钟；每模型探活 45 秒、鹈鹕 180 秒。新任务错峰最多 60 秒，后续加最多 15 秒抖动。关闭总开关/改为 V2 后不再领取新任务，已领取的本轮收尾。
- 监控输出捕获最多 4 MiB；HTML 复用既有鹈鹕输出上限与凭证脱敏。保留 10–500 条历史及每模型最近 3 个鹈鹕结果，均受 30 天清理限制。列表和历史只返回摘要，单条详情才加载 HTML。
- 消息/响应 ID 从已知 SSE/JSON 协议获取；请求 ID 优先读取账号 extra.upstream_request_id_header，其次常见请求头。上游不提供时显示“上游未返回”。gm-* 是站内服务日志追踪 ID，不是伪造的供应商消息 ID。
- 管理员可查看账号、模型映射、Token 与三类 ID。普通用户需同时满足启用计划、当前分组/订阅权限及用户展示开关；使用白名单 DTO，不返回账号、内部 ID、映射和上游错误。HTML 预览使用不带 same-origin 的沙箱及既有 CSP。
- 本方案为账号直接抽测，不写普通用户用量/计费记录，也不覆盖用户 Key 鉴权、复合路由、站内扣费等完整网关链路。开启检测会消耗上游额度。

## API 与主要文件

- 集中开关：GET/PATCH /api/v1/admin/settings/custom-features，flags 和 menus 均只发送改动项。
- 分组监控：GET/PUT /api/v1/admin/channel-group-monitors；DELETE /:id；POST /:id/run；GET /:id/history；GET/PATCH /:id/runs/:run_id。
- 用户只读：GET /api/v1/channel-group-monitors、/:id/history、/:id/runs/:run_id；无手动触发或评估入口。
- 前端：CustomFeaturesSettings.vue、GroupMonitorPanel.vue、api/groupMonitor.ts；原管理员渠道监控及用户 V1 渠道状态页接入共享组件。
- 后端：service/channel_group_monitor.go、repository/channel_group_monitor_repo.go、handler/channel_group_monitor_handler.go；Wire 生成与服务启动/退出清理已接入。

## 本地验证记录（2026-10-18）

```sh
# backend/
go generate ./cmd/server
go test -tags=unit ./internal/service ./internal/handler/... ./internal/server/routes -run 'Test(AccountTestTrace|GroupMonitor|CustomFeatures|PublicGroupMonitor|.*ChannelMonitor|.*UserVisibleMenu|.*PageContent)' -count=1
go test -tags=unit ./internal/service -run 'Test(Pelican|AccountTestMonitorCapture|RunPrompt|AccountTest.*Prompt)' -count=1
go test -tags=unit ./internal/server/routes -run '^TestCustomFeaturesZenxiangGuard$' -count=1
go test -tags=unit ./internal/service -run '^TestGroupMonitor' -count=1
go build ./cmd/server
# 隔离 PostgreSQL 18.1 / Redis 容器，2 项实际通过：
go test -tags=integration ./internal/repository -run '^TestGroupMonitor(LeaseBudgetAndStaleWrites|HistoryReviewRetentionAndRecovery)$' -count=1 -v

# frontend/
pnpm exec vitest run src/components/admin/__tests__/CustomFeaturesSettings.spec.ts src/components/monitor/__tests__/GroupMonitorPanel.spec.ts src/views/admin/__tests__/SettingsView.spec.ts
pnpm typecheck
pnpm exec eslint <本轮改动的 Vue/TS 文件>
pnpm build

# 仓库根目录
git diff --check
```

相关 Go 测试与服务端构建通过；鹈鹕/输出上限相关 14 项通过。前端本轮相关回归 55 项通过，最后修正后的 3 个组件/页面测试文件 52 项再次通过；中英文完整性、类型、改动文件 ESLint、构建通过。已有大包体积提示，不阻止构建。

数据库集成 2 项通过：新迁移应用、并发领取只成功一次、手动与定时共用租约及预算、UTC 跨日预算归零、旧 token 不得写新任务、过期 running 恢复失败；历史摘要不含 HTML、单条返回 HTML、人工评估写入、探活历史条数上限及鹈鹕预览保留均通过。生产迁移、长期调度及供应商兼容性仍未实测。

选号边界先复现失败：Anthropic/Gemini 实际查询了 fallback #10，而计划目标为 #9；修复后 Anthropic/Gemini/OpenAI 均只查询 #9。权限 HTTP 测试验证无权历史/单条详情 404 且不读取历史，关闭展示不可访问；菜单停用直链 404。

浏览器使用真实前端、完整应用路由与本地模拟 API：批量选 2 组/2 模型并以暂停保存，鹈鹕历史展示账号 #42 与模拟 ID、沙箱预览、人工评估保存通过；二改开关只修改菜单时 PATCH 仅含该菜单，成功提示保留。所有截图为模拟数据，不能作为真实供应商或数据库联调证据。

## 兼容与回退

新增表不改现有数据；新开关缺省保持旧功能行为，自动鹈鹕默认关闭。原设置页不继续保存集中管理的字段，避免旧表单回写。迁移随部署执行，本轮不连接生产、不发真实模型请求、不推送。

回退先暂停新计划，再回退二进制；新表与历史可保留，旧版本不读取。确需删除新数据时在备份后执行 `DROP TABLE channel_group_monitor_runs; DROP TABLE channel_group_monitors;`。不在自动迁移中执行破坏性回滚。

## 规划检查

A1–A10：范围、出口、数据依赖、权限、预算、超时、回退、干净基线及中断恢复均已定义。B1/B3/B5/B6/B9/B10/B11 适用；B2 不新增依赖，B4 不改网关请求性能路径，B7/B8 不进行生产批量或不可逆操作。C1/C2：每个模块完成追加检查点。无图谱则以函数引用检索核对上下游。

已处理验证失败：集中设置保存后卸载组件导致成功反馈消失，现改为局部更新父表单；选号误走回退分组，现固定管理员抽测组。均有回归证据。Docker 原始阻塞已解除；剩余未验证项为真实上游及生产/长时间运行。本轮未 commit/merge/push/部署或执行生产迁移。
