# 礼遇余额缓存修复

## 目标、范围与验收

用户已授权刷新绿云用户 1287 的余额缓存，并在当前仓库修复礼遇到账后余额缓存漏更新。
线上已执行仅针对 `billing:balance:{1287}` 的 DEL，返回 0（执行时已不存在），EXISTS=0；当时数据库余额 14.03984688。无余额修改、补发、清除在途预留或重启。
源码从干净 custom-main 创建 feature/fix-liyu-balance-cache；保留现有数据库事务、计费/预留与 API 契约。

1. 注入现有 APIKeyAuthCacheInvalidator 和 BillingCacheService，经生产 provider/Wire 传入礼遇服务。
2. 先通过真实 HTTP handler→礼遇服务→Redis 计费缓存/预留 Lua 建立回归，验证修复前失败。
3. 抽奖、幸运币、猜大小成功事务后同步失效余额与鉴权缓存；不写历史余额快照，不累加奖励；事务失败不处理，幂等重放可再次失效恢复缓存，skip 无余额变化不处理。
4. 失效使用独立的有超时 context，防止事务提交后客户端取消导致缓存永远不失效；失效失败记含 user_id 的日志，不把已提交奖励返回成失败。
5. 运行相关 service/handler/repository 回归、race 和后端 build，核对生成代码及 diff。临时撤回业务修复重新验证回归失败，再恢复并验证通过。

## 实码影响面

- service/zenxiang_liyu_service.go：Play→repo.Play；PlayLuckyCoin→repo.PlayLuckyCoin；PlayGuessSize→guessRepo.PlayGuessSize。
- repository/zenxiang_liyu_repo.go：用户 USD 余额变动仅在上述三条事务；GiftTickets/ResetUserDailyPlays 修改的是抽奖券钱包，不扩散处理。
- service/wire.go:ProvideZenxiangLiyuService→cmd/server/wire_gen.go→用户/admin礼遇 handler。
- repository/billing_cache.go：billing:balance:{uid}，计费服务已有 InvalidateUserBalance；余额缓存不得写成某次礼遇结果里的旧快照，以免覆盖并发消费。
- service/redeem_service.go、affiliate_service.go：沿用已有用户鉴权和余额缓存失效能力。
- 图谱未配置，以 rg 的调用引用和实际文件复核上下游。

## 风险、回退与门禁

A1-A10 已检查：两条用户需求分别对应线上单键失效与本地代码修复；每阶段保留命令/退出码证据；工作区修改前干净；预期3个实现文件+HTTP回归文件，另追加本地记忆；无用户输入、权限或日志敏感信息变动。验证失败回到对应实现，不改断言掩盖失败。回退限本分支新增代码，线上缓存由应用正常回源，不写旧值。
B1/B2/B4/B7/B8=N/A：无DDL、数据库修复、新依赖、性能算法、批量数据或不可逆操作。
B3 已检查：事务后失效、独立有界context、重试幂等、保持在途预留；不重写缓存金额。
B5 已检查：Redis使用已有接口；生产provider签名调整须同步Wire；HTTP和数据库契约不变。
B6 已检查：真实Redis(miniredis)和HTTP入口回归，正负奖励、失败、重放、取消、skip、缓存故障；相关测试与race/build。
B9/B10/B11 已检查：引用已实码核对、生成使用现有Wire、计划已回读；不新增服务器脚本。
C1/C2：实现与验证分阶段检查点；5文件或跨阶段时追加handoff。无代理委派。

## 当前进度

- 已完成：根因证据和线上单键失效，干净分支检查，影响面确认。
- 实现中：生产依赖注入、回归复现；尚未完成修复验证。
- 授权边界：本地修复及线上该用户缓存失效；未授权提交/推送/合并/部署。

## 验证完成

新增13场景、相关三包单元回归、race、go build ./...、diff检查均通过；线上只读核验缓存已回源一致。旧逻辑通过Go overlay重现失败，正式源码保持修复。交付限本地修复；未生产部署。

## 2026-10-20 17:10 合并授权更新

用户随后明确要求“合并分支”，现已授权本地提交和合并至 custom-main。合并前两分支均指向 42f9e7ed4，工作区仅包含本次修复和记录，diff 检查通过；按提交修复、切换 custom-main、merge --ff-only、核对分支及工作区的顺序执行。不推送、不发布、不部署，不删除修复分支。
