# OpenAI Observability Task 1 Report

日期：2026-07-13

## 实现内容

- 在 `backend/internal/service/openai_auto_scheduler_probe_runner.go` 增加共享分类器 `classifyOpenAIAutoSchedulerProbeEvent`，并提供仅转发的导出包装 `ClassifyOpenAIAutoSchedulerProbeEvent`。
- 分类器优先返回 `probe_error`（probe 失败或带错误）；成功 probe 以正 TTFB 为优先观测值，否则回退到正 Latency；按有效 settings 的 `SlowThresholdMS` 和 `SevereSlowThresholdMS` 返回 `slow`、`severe_slow` 或 `probe_success`。
- 自动 probe 将同一轮读取并归一化的有效 settings 传入分类器；管理员手动 probe 从 settings service 读取有效 settings，并调用同一导出包装。
- 未修改账号选择、调度策略或评分规则。

## TDD 证据

### RED

1. `GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run TestClassifyOpenAIAutoSchedulerProbeEvent -count=1`
   - 失败：`undefined: classifyOpenAIAutoSchedulerProbeEvent`。
2. `GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin -run TestOpenAIAutoSchedulerHandler_ProbeClassifiesSlowSuccessUsingEffectiveSettings -count=1`
   - 失败：7s 和 16s 成功 probe 均被错误记录为 `probe_success`，期望分别为 `slow` 和 `severe_slow`。

### GREEN

- `GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run TestClassifyOpenAIAutoSchedulerProbeEvent -count=1`：通过。
- `GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin -run 'TestOpenAIAutoSchedulerHandler_Probe' -count=1`：通过。
- `GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/handler/admin -run 'TestClassifyOpenAIAutoSchedulerProbeEvent|TestOpenAIAutoSchedulerProbe' -count=1`：service 通过；admin 因正则未匹配测试显示 `[no tests to run]`，已用上面的 handler probe 命令补证。
- `GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -count=1`：通过。
- `GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin -count=1`：通过。
- `git diff --check`：通过。

## 改动文件

- `backend/internal/service/openai_auto_scheduler_probe_runner.go`
- `backend/internal/service/openai_auto_scheduler_probe_classifier_test.go`
- `backend/internal/handler/admin/openai_auto_scheduler_handler.go`
- `backend/internal/handler/admin/openai_auto_scheduler_handler_test.go`

## 自审与风险

- 分类优先级明确：错误 > 正 TTFB > 正 Latency；nil 或非正延迟不会误判为慢。
- 自动与手动入口均进入同一核心分类器；手动入口额外校验 settings service 已配置。
- 现有 `//go:build unit` runner 测试在 `-tags=unit` 下会被分支中既有的旧 mock 接口缺失阻断（缺少 `ListUpstreamBalanceRefreshCandidatesByGroupID` 等），未修复无关问题；无 tag 的完整 service/admin 测试已通过。
- 未覆盖真实 HTTP probe 的慢响应计时，仅覆盖 checker 结果到事件分类的边界；该任务范围不涉及网络计时实现。

## Reviewer Fix: 手动 probe success 响应语义

### RED

- 强化 `TestOpenAIAutoSchedulerHandler_ProbeClassifiesSlowSuccessUsingEffectiveSettings`，要求 slow/severe 成功 probe 的响应包含 `"success":true`。
- `GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin -run 'TestOpenAIAutoSchedulerHandler_Probe' -count=1`：失败；slow 和 severe_slow 的响应均为 `"success":false`，虽然 checker 成功且事件分类正确。

### GREEN

- 将响应 `success` 字段改为 `result.Success && result.Err == nil`，与真实 checker 成功语义一致；事件类型仍由共享分类器决定。
- `GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin -run 'TestOpenAIAutoSchedulerHandler_Probe' -count=1`：通过（`ok github.com/Wei-Shaw/sub2api/internal/handler/admin`）。
