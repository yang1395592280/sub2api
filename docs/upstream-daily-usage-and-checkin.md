# 顶栏签到与上游每日消费

## 使用

- 登录后在顶栏「每日签到」打开签到日历、领取奖励和翻倍；个人资料页不再内嵌签到卡片。
- 翻倍先展示洗牌，再按服务端结果翻牌：中奖为绿色光环和彩屑，未中奖为紫色涟漪。遵循系统减少动画偏好。失败仍执行原活动规则：扣除 1.5R，今日最终净收益 −0.5R。
- 账号管理的「刷新余额」同时更新余额、现有上游分组/渠道价格和今日上游消费，沿用手动与分组自动刷新入口。
- 「今日上游扣费」只统计当前账号维护的上游 API Key；「折算成本」为该扣费除以账号充值比例。不要再次乘渠道倍率，实际扣费已包含上游计价。
- 「上游账号今日合计」统计管理凭据对应用户的所有 API Key，可能包含本系统之外的调用；多个本地渠道使用同一个上游用户时不要重复求和。多个本地账号使用同一 API Key 时，其 Key 消费同样不能重复相加。
- 日期按 Asia/Shanghai。跨日旧快照隐藏金额并提示刷新；统计失败、Key 未唯一匹配、凭据不足均显示不可用，不当成零。余额刷新仍可以成功。

## 源码依据与匹配

核对基线：本仓库 99d8fb838 及本地 upstream/main；New API 官方仓库 `QuantumNous/new-api` 的 `0bda086fb4c2f83ac0397e4a60d90686578d67ad`。

| 平台 | 接口及字段 | 归属与口径 |
| --- | --- | --- |
| sub2api | `GET /v1/usage?days=1&timezone=Asia/Shanghai`，`daily_usage[].actual_cost` | 当前 API Key，当日实际扣费；不使用 `cost` 标准价或钱包余额差值 |
| sub2api 管理凭据 | `GET /api/v1/usage/stats?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD&timezone=Asia/Shanghai`，`total_actual_cost` | 不带 Key 为登录用户合计；带 `api_key_id` 为当前 Key；使用已有 Key 列表按完整密钥匹配 ID |
| New API | `GET /api/token/search?token=...` | 复用维护的用户 ID / Access Token，完整 Key 或掩码前后缀唯一匹配 Token ID；不再把任意单条结果当匹配 |
| New API | `GET /api/log/self?type=2&start_timestamp=...&end_timestamp=...&p=N&page_size=100`，`token_id`、`quota` | 分页读完整当日消费日志，按 Token ID 区分重名、改名令牌；所有日志合计为上游用户合计 |
| New API | `GET /api/status`，`quota_per_unit` | 使用站点额度单位换算；未返回有效配置时沿用既有默认 500000 quota/USD |

参考源码：

- sub2api：`backend/internal/handler/gateway_handler.go` 的 `Usage`、`buildUsageData`、`buildAPIKeyDailyUsage`；`backend/internal/handler/usage_handler.go` 的 `Stats` 与 `parseUserUsageFilters`。
- [New API 日志控制器](https://github.com/QuantumNous/new-api/blob/0bda086fb4c2f83ac0397e4a60d90686578d67ad/controller/log.go)、[日志查询与汇总](https://github.com/QuantumNous/new-api/blob/0bda086fb4c2f83ac0397e4a60d90686578d67ad/model/log.go)、[额度单位配置](https://github.com/QuantumNous/new-api/blob/0bda086fb4c2f83ac0397e4a60d90686578d67ad/controller/misc.go)。

`/api/log/self/stat` 仅按 `token_name` 过滤，重名或改名会影响归属，故不用于 Key 级核算；`/api/log/token` 限制最近条数，不能作为完整日消费。

## 兼容性与边界

- 使用既有 `accounts.extra.upstream_balance_daily_usage` 缓存当前日期快照，无数据库迁移、额外后台任务或新依赖。该键沿用 `upstream_balance_` 观测字段前缀，不触发无关调度重建。
- 这是当日快照，不是历史账本，不补历史日期；精确性取决于上游已落库日志。充值、赠送和签到变化不混入消费。
- sub2api 管理凭据不足时，仍可通过支持 `daily_usage` 的 Key 接口获取本 Key 消费；账号总计不可用时不展示。旧上游可回退管理端统计。
- New API 每次最多读取 10000 条日志，总读取超时 10 秒；额外探测一页避免上游 total 截断。超过上限、页数据不完整、查询期间 total 改变或缺少必要字段时不提供部分合计。高请求量站点应提供按 Token ID 聚合的接口后再扩展。
- 本地渠道对应的是配置中的上游 Key；普通用户凭据不能读取上游管理员内部渠道成本。快照反映该 Key 所有调用的上游扣费。
- 回退：回退本次功能代码即可；新增 extra 字段可保留，旧版本忽略。没有生产数据清理需求。

## 验证

- Go：上游服务完整 short 测试、定向 provider / 每日统计测试、race、build、vet；具体执行结果在 `.ai-memory/20261113/checkin-upstream-daily-plan.md`。
- 前端：签到与余额组件、布局与个人资料回归、i18n、类型检查、ESLint、正式构建；在本地模拟 API 验证桌面/390px 弹窗、两种结果、账号余额刷新。
- 浏览器模拟接口只用于本地验收，没有操作真实用户钱包或上游账户。真实站点的凭据权限、验证码和定制接口仍需上线后核对。
- 账号页验收额外发现并修复质量监控开启时的初始化顺序错误：立即执行的 watcher 移至账号列表创建之后，新增开启质量监控的回归用例。
- 最终相关前端测试 38 项通过，翻牌最后调整后签到 6 项通过；Go provider / 每日消费定向 48 项及 race 通过。构建、类型检查和修改文件 ESLint 通过；完整验证记录见上述计划。
