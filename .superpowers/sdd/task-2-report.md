# Task 2 Report

## 时间
- 2026-07-06

## 任务结论
- 已完成 Task 2：新增上游价格守卫策略 helper 与聚焦单测。
- 未实现 group refresh runner、repository candidate query、wire startup、frontend，保持在任务边界内。

## 修改文件
- `backend/internal/service/openai_upstream_price_guard.go`
- `backend/internal/service/openai_upstream_price_guard_test.go`

## 核心变更
- 新增 `ApplyGroupUpstreamPriceGuard(ctx, repo, account, group, now)`：
  - 当 `upstream_price_max_multiplier <= 0` 时，仅回写 `upstream_price_guard_status=ok`，表示不做价格拦截。
  - 当账号缺失 `ChannelPrice` 或有效价格不可用时，回写 `unsupported` 与错误信息，不做 temp unschedulable 封禁。
  - 当实际上游倍率高于分组阈值时：
    - 回写 `upstream_price_guard_*` 观测字段；
    - 以 `upstream_price_guard:` 前缀写入 `temp_unschedulable_reason`；
    - 设置 24 小时临时不可调度。
  - 当价格恢复正常时：
    - 回写 `upstream_price_guard_status=ok`；
    - 仅当现有 `temp_unschedulable_reason` 以 `upstream_price_guard:` 开头时才清理临时不可调度状态；
    - 其他原因（如 token refresh retry exhausted）保持不动。

## TDD / 验证过程
1. 先新增 `openai_upstream_price_guard_test.go`。
2. 运行：
   - `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestApplyGroupUpstreamPriceGuard'`
   - 结果：失败，报 `ApplyGroupUpstreamPriceGuard` 与 `UpstreamPriceGuardReasonPrefix` 未定义，符合预期红灯。
3. 新增 `openai_upstream_price_guard.go` 最小实现。
4. 运行：
   - `gofmt -w internal/service/openai_upstream_price_guard.go internal/service/openai_upstream_price_guard_test.go`
   - `GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestApplyGroupUpstreamPriceGuard'`
   - 结果：通过。

## 边界与说明
- 本次未修改 `backend/internal/service/account.go`：该文件已存在 `EffectiveChannelPrice()`，且其语义已满足 helper 使用，无需额外改动。
- 未改动用户计费倍率、API Key 配额、usage billing 相关逻辑。
- 未触碰其他开发者已有变更；工作区中原有 `.superpowers/sdd/progress.md` 改动保持原样。

## 风险点
- 当前 helper 仅完成策略判断与 repo 写入，尚未接入实际 group refresh 调度流程；必须等待后续任务把调用链串起来，策略才会真正生效。

## 提交信息
- `feat: add upstream price guard policy`
