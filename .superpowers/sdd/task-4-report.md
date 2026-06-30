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

## Third Review Fix

修复 Task 4 第三轮 review 中的入口与兼容问题：

- 自动分组 API Key 会绕过未分组 Key 拦截，并在路由平台判断中被识别为 OpenAI，从而进入 OpenAI handler。
- 固定 nil-group API Key 在 effective wrapper 中直接回落原有选择函数，保留 legacy nil group 行为。
- `/v1/messages` 自动模式 failover 后每轮都从原始 `reqModel` 重新按候选组解析 dispatch model，不复用上一轮 effective mapped model 作为下一轮输入。
- 补齐 middleware 测试 fake repo 的 `UpdateLastEffectiveGroup` 方法，保持 server 包测试可编译。

复跑验证：

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestOpenAIAutoCheapestSelection|TestOpenAIGatewayService_SelectAccount' -count=1
```

结果：通过。

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler -run 'TestOpenAI|Test.*Responses|Test.*ChatCompletions|Test.*Embeddings|Test.*Images' -count=1
```

结果：通过。

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/server/middleware ./internal/server/routes -run 'Test.*Group|Test.*Gateway|Test.*Route|Test.*APIKey' -count=1
```

结果：通过。

## Second Review Fix

修复 Task 4 第二轮 review 中的阻塞问题：

- 固定 API Key 的 WaitPlan 行为恢复：只有自动模式才要求 `selection.Acquired == true` 才算当前组成功；固定模式仍保留原有等待逻辑。
- OpenAI `/v1/chat/completions` 接入 effective group，自动模式成功后使用实际分组做 channel mapping、billing、account slot、cyber usage 和 RecordUsage。
- OpenAI `/v1/embeddings` 接入 effective group，自动模式成功后使用实际分组做 channel mapping、billing、account slot 和 RecordUsage。
- OpenAI Images 接入 effective group，并通过 images 专用 wrapper 保留 native/basic capability fallback。

复跑验证：

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestOpenAIAutoCheapestSelection|TestOpenAIGatewayService_SelectAccount' -count=1
```

结果：通过。

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler -run 'TestOpenAI|Test.*Responses|Test.*ChatCompletions|Test.*Embeddings|Test.*Images' -count=1
```

结果：通过。

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler -run 'TestOpenAI.*Gateway|Test.*Responses|Test.*ChatCompletions' -count=1
```

结果：通过。

## Concerns

- 当前 handler 级测试主要验证编译和既有路径回归；尚未新增完整 HTTP 级自动分组 E2E 用例。

## Review Fix

修复 Task 4 review 中的阻塞问题：

- `/v1/messages` 自动模式现在按每个候选 effective group 解析 `ResolveMessagesDispatchModel` 后再进入组内 scheduler，避免 Claude family 请求在选组前以原始模型被误判不可用。
- 自动跨组 wrapper 只把已获得并发槽位的 `selection.Acquired` 视为成功；如果最低价组只返回 `WaitPlan`，会继续尝试下一个候选组。
- 在 `backend/internal/service/wire.go` 增加 `ProvideOpenAIGatewayService`，把 resolver/updater 注入放回 Wire source-of-truth；`wire_gen.go` 保持对应生成结果。

复跑验证：

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
