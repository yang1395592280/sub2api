# OpenAI 调度与健康看板回退设计

日期：2026-06-25

## 背景

当前 `custom-main` 中存在一套二改 OpenAI 调度增强：健康分层调度、OpenAI 调度管理页、OpenAI 健康看板、调度每日统计，以及账号管理里的“调度状态”列。用户希望删除这些二改能力，恢复原项目自带调度行为，并且账号管理中整列删除“调度状态”。

## 目标

- 删除 OpenAI 调度管理页和 OpenAI 健康看板的前端入口、路由、API、页面和文案。
- 删除对应后端 admin handler、路由注入、健康看板聚合 service、每日调度统计 repository 和统计写入。
- 恢复 OpenAI 账号选择为原项目基础调度：候选账号过滤、粘性会话、previous_response_id 续接、负载/优先级/价格等既有基础权重仍保留；不再使用主力、备用、观察、隔离这类健康分层决定调度。
- 账号管理列表整列删除“调度状态”，后端响应也不再计算或返回 `stability` 字段。

## 非目标

- 不删除原项目已有临时不可调度能力，包括 `temp_unschedulable_until`、错误重试后临时摘除账号、限流退出调度等基础保护逻辑。
- 不删除 OpenAI OAuth、API Key、模型映射、计费、粘性会话、分组隔离、连接池、失败重试等原有网关能力。
- 不做数据库破坏性清理。已有 `openai_scheduler_daily_stats` 表可以保留为空闲历史表，本次只停止新写入和移除代码依赖。
- 不执行 `git push`、`rebase`、`reset --hard` 等高风险 Git 操作。

## 后端设计

1. 调度核心
   - 从 `backend/internal/service/openai_account_scheduler.go` 移除健康分层设置、健康快照、手动动作、健康分层过滤和基于健康 tier 的选择逻辑。
   - `Select` 保留 previous response、session sticky、load balance 三层选择。
   - `buildOpenAIAccountLoadPlan` 不再生成 `OpenAIAccountHealthSnapshot`，不再排除 `degraded` tier 账号；是否可调度仍由账号状态、临时不可调度、限流、能力匹配、分组隔离等基础过滤决定。

2. 管理接口
   - 删除 `/api/v1/admin/openai-scheduler/*` 和 `/api/v1/admin/openai-health/*` handler、wire 注入和路由注册。
   - 删除 `OpenAISchedulerStatsRepository` 的依赖注入和 `recordOpenAISchedulerDailySelection` 写入链路。
   - 删除 `ChannelMonitorService.GetOpenAIHealthOverview` 等仅服务 OpenAI 健康看板的聚合代码。

3. 账号管理响应
   - 从 admin account list 响应类型中移除 `stability` 字段。
   - 删除账号列表批量查询稳定性统计和 OpenAI 健康快照转调度状态的逻辑。
   - 保留账号当前并发、Anthropic 窗口费用、活跃会话数、RPM 等与调度状态列无关的运行时字段。

## 前端设计

1. 页面与导航
   - 删除 `OpenAISchedulerView.vue`、`OpenAIHealthView.vue`、对应 API 文件、路由和侧边栏入口。
   - 删除相关 i18n 文案和路由/侧边栏测试。

2. 账号管理
   - 从 `AccountsView.vue` 删除 `stability` 表格列、header tooltip、cell slot、颜色类、tooltip 文案、局部更新时保留 stability 的逻辑。
   - 从前端类型定义中移除 `AccountStability` 和账号上的 `stability` 字段。
   - 更新账号页相关测试，避免继续断言该列或字段。

## 数据库与迁移

不新增删除表迁移。`backend/migrations/151_openai_scheduler_daily_stats.sql` 可删除或保留需要在实现时根据迁移加载机制确认：

- 如果迁移文件删除会影响已部署环境的迁移序号或历史一致性，则保留文件，但删除代码引用。
- 如果项目允许删除尚未发布的自定义迁移，则删除该迁移文件和 repository。

默认选择保留已存在迁移历史，不再写入该表，降低部署风险。

## 测试与验证

- 后端：运行 OpenAI 调度相关最小测试集，重点覆盖 sticky session、load balance、分组隔离、临时不可调度、OpenAI gateway 选择失败回退。
- 后端：运行 admin account handler 相关测试，确认账号列表响应不再包含 `stability` 且其他运行时字段正常。
- 前端：运行账号页、路由、侧边栏相关测试，确认无已删除路由和列的残留断言。
- 全局搜索 `openaiScheduler`、`openaiHealth`、`OpenAIScheduler`、`stability`，确认仅保留与非本次删除目标相关的内容。

## 风险

- OpenAI 调度核心历史改动较多，直接删除健康分层时要避免误删基础调度、分组隔离、能力匹配和临时不可调度逻辑。
- 前后端类型同时变化，账号列表局部刷新和批量编辑测试需要同步调整。
- 如果某些后续代码依赖 OpenAI 调度统计 repository 的构造参数，删除注入时需要同步修正 wire 构造链。
