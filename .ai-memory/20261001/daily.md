# 工作日志 - 2026-10-01
session-id: 20261001-0012

## [00:12] Bug 修复：上游同步后的并发与余额预留边界
- 范围：余额 Lua / 冷缓存建立、OpenAI HTTP 自动分组预留、WebSocket 每轮预留、API Key 创建事务。
- 决策：保持现有价格和单请求准入策略；余额与预留使用同一 Redis hash tag；使用 PostgreSQL 用户行锁和事务 context 绑定数量检查与写入。
- 验证：4 个原始问题有复现日志；专项回归通过，全量测试、构建和 race 检测运行中。
- 风险：余额缓存 key 从 billing:balance:uid 变为 billing:balance:{uid}；部署须避免旧新版本混跑造成余额缓存分裂，无数据库迁移。

## 收尾：验证完成
- 根因修复：余额读取与预留在同一 Lua 内完成；自动选组的三个 HTTP 入口与 WS 使用实际分组；WS 每轮持有预留并交给计费引用；Key 计数与创建绑定用户行锁事务。
- 补充回归发现 WithInflightReservation(ctx, nil) 原来不遮蔽旧 context，已修正并验证切组/订阅切换。
- 验证：go test -json -tags=unit ./...：57 包、12105 项顶层测试通过，17 项跳过；包含子测试 22641 项通过。go build ./... 与专项 go test -race 通过。git diff --check 通过。
- 实际入口：Responses/Messages/Chat 自动选组的 fail-open/fail-closed 分支、WS 空闲释放、WS 后续轮重新估价、WS 自动选组首轮拒绝。
- 未验证：Docker daemon 未运行，PostgreSQL 并发仓库集成用例由 harness 跳过。后续命令：cd backend && go test -v -tags=integration ./internal/repository -run '^TestAPIKeyCreateLock_ConcurrentRepositoriesRespectLimit$' -count=1。
- 部署注意：余额 Redis key 增加用户 hash tag。避免旧新后端同时运行导致两套余额缓存；推荐统一重启。无 DDL 或用户数据迁移。
