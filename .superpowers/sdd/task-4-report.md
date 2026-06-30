# Task 4 Report

## Summary

已完成 OpenAI 自动最优惠分组在 OpenAI gateway 侧的最小接入：

- 新增 effective OpenAI API Key 解析与跨分组逐组选号。
- 自动模式按 `CandidateGroups` 顺序尝试分组，并复用现有组内选号链路。
- 成功选中账号后，计费、usage log、sticky session、cyber usage 归属和 channel usage fields 使用实际 `effectiveAPIKey`。
- 成功命中自动分组后 best-effort 更新 `last_effective_group_id`。
- 固定分组模式继续走原有组内选择逻辑。

## Files Changed

- `backend/internal/service/openai_auto_cheapest_group_service.go`
- `backend/internal/service/openai_auto_cheapest_group_service_test.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/cmd/server/wire_gen.go`

## Notes

- 没有改 `GatewayHandler` 的 Anthropic/Gemini 兼容路径，避免把 OpenAI 自动分组带入非 OpenAI 平台。
- 自动模式下，OpenAI Responses / Messages / WebSocket 会在实际分组选出后再做 group 相关的 image permission、billing eligibility 和 channel mapping。
- 组内账号健康、runtime block、并发、scheduler open/half_open/running/observing 等仍全部由现有 selection 函数判断。

## Verification

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestOpenAIAutoCheapestSelection|TestOpenAIGatewayService_SelectAccount' -count=1
```

结果：通过。

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler -run 'TestOpenAI.*Gateway|Test.*Responses|Test.*ChatCompletions' -count=1
```

结果：通过。

## Concerns

- 当前 handler 级测试主要验证编译和既有路径回归；尚未新增完整 HTTP 级自动分组 E2E 用例。
- `wire_gen.go` 是生成文件，本次按现有项目结构直接补了 resolver 注入；如果项目后续重新跑 Wire，需要同步维护 provider 图。
