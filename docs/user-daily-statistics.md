# 用户每日统计

管理后台 →「用户每日统计」，路径 `/admin/user-daily-stats`。默认查看今天，可选择历史日期，按邮箱、用户名或用户 ID 搜索，并筛选签到、礼遇、调用或任意活跃用户。筛选后顶部显示所有匹配用户的汇总，分页不改变汇总；CSV 明确只导出当前页。

## 指标口径

| 指标 | 来源与含义 |
| --- | --- |
| 签到净收益 | 当日签到基础奖励 + 当日实际执行的签到翻倍增减，分别展示基础与输赢 |
| 礼遇净收益 | 当日抽奖中奖奖励 + 当日幸运翻倍增减 + 当日猜大小增减，保留负数 |
| 券面额 | 当日抽奖记录的券名义金额，不是本功能认定的现金支出或利润 |
| 当前充值余额 | 查询时的 users.balance；另显示冻结余额、累计充值 |
| 当前福利余额 | 查询时的签到 + 礼遇 + 赠送余额，分项展示，保留负余额 |
| 当日／前日消费合计 | 两个北京时间自然日内的 usage_logs.actual_cost，包含余额和订阅计费 |
| 充值扣款／福利抵扣 | 按用户、API Key、request ID 匹配已提交的 usage 钱包流水；福利进一步拆成签到、礼遇、赠送 |
| 订阅计费 | billing_type=1 的实际费用，不冒充充值余额扣款 |
| 未分账 | 余额计费但缺少对应扣款流水的费用，包括旧记录和未完成结算的记录；不按现比例推算 |
| 全站名次 | 当日总消费降序、Token 降序、用户 ID 升序；仅正消费用户排名，搜索与参与筛选保留全站名次 |
| 请求／Token | 每日调用数及输入、输出、缓存创建、缓存读取 Token 合计 |
| 管理员赠送／其他福利调整 | 当日管理员赠送、福利退款、签到清退及恢复流水净额，活动流水可逐笔核对 |

- 时间范围统一 Asia/Shanghai 的 `[00:00, 次日00:00)`。签到基础按 checkin_date、礼遇基础按 play_date 归日；后续游戏按实际执行时间归日，避免跨日重算。
- 用户范围包含在所选日期结束前注册的用户与软删除用户；软删除／禁用状态明确标注。历史日期旁的余额仍然是**查询时当前余额**，没有历史日末快照。
- 历史活动奖励可能曾入旧充值余额。活动净收益反映奖励及游戏输赢，不能直接等同当日福利入账或福利核销；也不能拿券面额或原活动名义利润当现金利润。
- 详细金额显示 8 位小数，单位沿用站内 USD。计费费用和钱包存储精度不同，合计可能有微小舍入差异。页面提供数据生成时间，刷新会重新查询。

## 接口与边界

- `GET /api/v1/admin/user-daily-stats`：date（YYYY-MM-DD）、search（最多200字）、activity（all/active/checkin/liyu/usage）、sort_by（today_cost/previous_cost/checkin_net/liyu_net/balance/welfare_balance/requests）、sort_order（asc/desc）、page、page_size（最多100）。
- `GET /api/v1/admin/user-daily-stats/:id/activities`：date、page、page_size，按执行时间倒序，返回来源、原记录 ID、奖励或输赢、券面额、结果和说明。
- 两条接口沿用管理员认证。受限管理密钥即使持有现有全套独立权限，也默认无法读取新汇总；普通管理员和全权管理密钥遵循现有全权访问规则。
- SQL 参数化、排序白名单、15秒查询超时。各来源先聚合再关联，一条查询同时计算列表、排名和筛选汇总，避免活动多对多放大消费及分页汇总不一致。
- 功能只读，不新增数据库迁移，不修改活动、计费、余额或生产策略。回退对应前后端改动即可，不需回滚数据。

## 本地验证（2026-11-13）

- Go 日期／参数／分页／错误单元测试、Gin HTTP 参数及错误响应、管理密钥默认拒绝测试；相关签到、钱包、原消费排行榜回归通过。
- `go test -tags integration ./internal/repository -run '^Test(UserDailyStatsIntegration|WalletIntegration)' -count=1 -v`：隔离 PostgreSQL 18.1 与 Redis 8.4，实际执行16项并通过，未跳过。覆盖微额分账、订阅、未分账、跨日输赢、负福利、多抽奖不重复消费、全站排名、软删除、筛选、排序、分页及通配字符搜索。
- 前端组件测试验证精度、筛选、分页、北京跨日、明细、过期响应、错误重试及 CSV 公式转义；中英文键完整性、TypeScript、ESLint、正式构建通过。
- 浏览器验证使用正式构建配本地模拟 API，测试数据为虚构账号。真实生产数据联调与生产规模的查询耗时尚未验证；本轮不读取或操作生产账目。
- 正式构建浏览器实测：1440px桌面与390px手机卡片／明细；搜索保留名次、历史日切换、微额和负福利显示，console error为空。实际下载当前页CSV，核对表头和充值0.00000002／福利0.00000001精度；手机页面宽382px小于视口390px。
- 其他执行命令：`go generate ./cmd/server`；`go test -tags unit ./internal/service ./internal/repository ./internal/handler/admin ./internal/pkg/adminaccess ./internal/server/routes -run 'Test(UserDailyStats|NormalizeUserDailyStatsQuery|ScopedAdmin|Checkin|.*Wallet|UsageLogRepositoryGetUserSpendingRanking)' -count=1`；新增统计与权限定向三包 `go test -race -tags unit`；`go build ./cmd/server`；`go vet ./internal/service ./internal/repository ./internal/handler/admin ./internal/server/routes`。
- 前端执行 `pnpm test:run`（UserDailyStatsView、UserSpendingRankingView、localeKeyCompleteness，12项），`pnpm typecheck`、`pnpm lint:check`、`pnpm build` 均通过。Vite仍提示现有大chunk及Browserslist数据陈旧，未升级依赖或调整无关构建策略。
