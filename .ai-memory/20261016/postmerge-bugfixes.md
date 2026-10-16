# 上游合并后 Bug 排查与修复

用户请求检查其他 Bug 并直接修复。基线 custom-main=edaf58734，工作区干净；修复分支 feature/postmerge-bugfixes。无推送、发布、部署或版本修改授权。

## 已确认问题与影响面

分组用量优化把水位读取和金额汇总拆成两条 SQL，但没有统一数据库快照。并发日志清理/历史回填会后退水位，第二条 SQL 仍按旧水位组合新日志与日桶，统计金额可能既不是修改前也不是修改后。PostgreSQL 18 集成回归已复现：清理前总额1.75、清理后0，旧实现却返回1.25。

真实调用链已用 rg 复核：GroupHandler.GetUsageSummary → DashboardService.loadGroupUsageSummary/refreshGroupUsageSummaryAsync → UsageLogRepository.GetAllGroupUsageSummary → getAllGroupUsageSummaryFromRollups → readGroupUsageRollupSnapshot + 汇总SQL。修改集中于仓储实现、仓储单元测试、现有日桶集成测试。图谱查询返回Go符号索引缺失，以rg与实码为准。

## 实施与验证计划

1. 从支持 BeginTx 的 DB/Conn 开启 ReadOnly + RepeatableRead 事务，让两条SQL共用快照；调用方传入的事务保留其生命周期。保留参数化尾段索引条件、日桶与原有无效水位降级规则。
2. 保留真实PostgreSQL并发复现测试，通过测试schema视图与advisory锁精确安排清理提交时机；同时校验下一次查询能看到清理后的0，不以无限使用旧快照掩盖问题。
3. 扩展事务开始、读取水位、汇总、提交失败的单元测试，确认失败金额不返回给缓存；运行仓储unit/race、相关service/handler、真实数据库集成和服务端构建。
4. 检查diff、资源释放及影响面，追加日志和验证结论。前端未修改，复用本轮全量测试407文件/3114项通过的结果。

## 方案取舍与边界

选择短生命周期只读可重复读事务，比重新合成单条SQL更能保留已实现的索引查询优化；比锁水位行更少阻塞日志写入/清理。代价是每次刷新多一次事务开始与提交，受现有30秒缓存、singleflight和30秒查询超时约束。无数据库迁移、数据修复、扣费、权限、生产配置或新依赖变更；数据库行为仅在隔离的testcontainers PostgreSQL/Redis中验证。若未来改成单条同快照且索引性能经验证的SQL，可撤销事务包装。

## 检查点

- 前端全量407文件3114项通过；后端repository基线unit通过。
- 新并发集成回归Red已确认，错误金额1.25（期望1.75）。
- 最小仓储修复与事务失败回归已实现，Green、race及收尾验证已完成。

## 验证结果

- `pnpm exec vitest run`：前端全量407文件、3114项通过；测试夹具仍输出既有缺少mock方法的日志，但无失败。前端源码本轮未修改。
- `go test -tags=unit ./internal/repository -count=1`：仓储整包通过（10.017s）。
- `go test -tags=integration ./internal/repository -run '^TestGroupUsage' -count=1 -v`：9个顶层测试及子用例通过（4.265s），包含真实日桶、水位失效、清理并发、时区/DST；使用隔离的PostgreSQL 18.1与Redis容器。
- `go test -race -tags=unit ./internal/repository ./internal/service ./internal/handler/admin -run 'GroupUsageSummary|GroupHandlerUsageSummary|GroupUsageRollup' -count=1`：3包通过，无race报告。
- `go test -race -tags=integration ./internal/repository -run '^TestGroupUsageSummaryKeepsSnapshotDuringConcurrentCleanup$' -count=3 -v`：并发清理集成回归连续3次通过，无race报告；下一次查询返回0的断言也通过。
- `go build -o /tmp/sub2api-postmerge-bugfix-server ./cmd/server`：通过。
- `git diff --check`与改动Go文件gofmt检查：通过。

## 自审与交付状态

真实Red失败与Green通过使用同一测试，错误金额1.25已被消除；未放宽断言。事务begin、水位读取、金额汇总、commit失败均返回nil金额，错误由既有service失败回退处理；defer rollback覆盖失败，成功commit释放快照，不接管调用方事务。原参数化SQL完整保留，未增加锁水位行或循环查询。图谱临时产物已清理。

本轮只确认并修复上述统计读取问题，排查范围内未确认其他新增缺陷；不声称全仓无Bug。当前feature/postmerge-bugfixes保留可审查工作区diff，未提交/合并/推送/部署；未连接生产数据库或验证生产负载/真实支付与供应商。事务增加begin/commit开销，生产延迟仍需上线后观察。

## 本地提交与合并授权

用户随后明确要求“合并分支”，授权提交本轮修复并合入本地custom-main。合并前feature/postmerge-bugfixes与custom-main均为edaf58734，目标分支未被其他worktree占用；工作区仅包含上述3份Go实现/测试文件与4份本轮记录。采用git merge --ff-only，失败则保留分支与改动，不覆盖用户内容。复用本轮已通过的unit/integration/race/前端全量与构建验证，不追加业务代码，不推送、打tag、发布或部署；完成结果以Git分支及工作区核验为准。
