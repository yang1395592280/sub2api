# v0.2.18 合并后 Bug 排查与修复

日期：2026-10-25。用户要求继续检查并修复其他 Bug；基于 custom-main 的 85231d2ce，在 feature/fix-upstream-018-regressions 开发。

## 已确认问题

OpenAI 容量降载重试的数值解析先转换为 time.Duration，再限制到 8 秒，存在两种异常行为：

- amd64：Retry-After 或 Retry-After-Ms 的超大有限数值（例如 9223372036854775807、1e300）转换成负时长，调用方忽略该等待指令。
- arm64：+Inf 被接受并转换成大时长，最终等待 8 秒；同一响应提供了有效的毫秒值时，仍无法回退到该值。

旧代码在 arm64 的回归测试中有 2 项失败；通过 Go overlay 加载原代码，在 amd64 上再次确认回归测试失败。未改动生产代码来制造测试失败，未放宽已有断言。

## 实现与决策

- backend/internal/service/openai_gateway_upstream_errors.go：拒绝无穷大，在浮点数转换为 Duration 之前应用既有 8 秒上限；继续按原优先级查找后续有效值。
- backend/internal/service/openai_capacity_shed_test.go：增加 12 个边界用例，覆盖超大值、小数、Inf、NaN、负数及 HTTP 日期；原生和透传流式路径断言无效头回退到有效的 1500ms。
- 采用局部校验，复用现有上限；未增加依赖、未调整限流策略或重试次数。保留 HTTP 日期的原有解析和最终上限。
- 撤销条件：未来若调整容量降载的等待上限，应同步更新数值解析与回归预期；无需数据库或数据回滚。

## 已排除的疑点

- scheduler_cache.GetAccount 逐次 JSON 解码，选中账号后 hydrate 得到独立对象；Fable 候选阶段写入通过 setAccountModelRateLimitSnapshot 写时复制，因此本次未把共享 map 疑点判为缺陷。
- UpdateWithAccountBillingSettings 的底层 updateAccount 复用 context 中的事务，受限管理员路径未发现该处绕开事务的证据。
- 账号配额优化保留了跨越额度上限时的 scheduler outbox 更新，并在日/周配置与提示不一致时回退到完整 SQL；没有为此修改数据库代码。

## 验证

- 前端 pnpm --dir frontend run test:run：410 个文件、3147 项测试通过。
- arm64 和 amd64 定向回归：17 个数值用例及 2 个流式路径全部通过。
- go vet ./...：通过。
- CGO_ENABLED=0 go build -tags=embed -trimpath -o /tmp/sub2api-postmerge-server ./cmd/server：通过。
- go test -tags=unit ./internal/service ./internal/handler ./internal/repository：通过，分别用时 217.921s、44.859s、12.103s。
- git diff --check：通过。

日志：/tmp/sub2api-postmerge-retry-red.log、/tmp/sub2api-postmerge-retry-amd64-red.log、/tmp/sub2api-postmerge-retry-arm64-tests.log、/tmp/sub2api-postmerge-retry-amd64-tests.log、/tmp/sub2api-postmerge-backend-tests.log、/tmp/sub2api-postmerge-vet.log、/tmp/sub2api-postmerge-build.log、/tmp/sub2api-postmerge-20261025-frontend-tests.log。

## 边界

本轮没有执行真实 PostgreSQL/Redis 集成测试（Docker daemon 未启动，SCOPED_ADMIN_TEST_DSN 未设置），没有调用真实第三方上游；不能据此宣称全仓库不存在其他 Bug。未 push、未部署、未执行数据库迁移。

## Session Summary

- 会话目标：审查上游同步后的兼容问题，修复可复现缺陷。
- 已完成：两种架构复现、局部根因修复、补充回归、后端整组测试、前端全量测试、vet 与构建；安全及质量自审未发现新增数据库、权限、日志或并发风险。
- 关键决策：在数值转换前校验和封顶，保留既有 8 秒策略与后备来源；仅复用已存在的标准库，不引入新抽象或依赖。
- 遗留问题：无已确认而未修复的缺陷；真实数据库及第三方上游验证未执行，原因见上文。
- 交付方式：修复先在 feature 分支完成，再提交并快进合回本地 custom-main；最终提交点以 Git 历史为准。
- 后续建议：上线前在测试环境验证新增上游迁移及受限管理密钥；本轮不执行部署或迁移。
- 规范沉淀：Go 的超范围浮点数到整数转换在不同架构表现不同；来自外部的时长应在转换前校验和限制范围，不能依赖转换后的负数检测。
