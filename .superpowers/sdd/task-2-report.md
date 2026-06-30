# Task 2 Report

## Summary

已完成 Task 2 指定范围内的 API Key `group_select_mode` 暴露与联动：

- service create/update request 增加 `group_select_mode`
- create/update 校验支持 `fixed` / `openai_auto_cheapest`
- handler request DTO 映射透传 `group_select_mode`
- handler response DTO 增加：
  - `group_select_mode`
  - `last_effective_group_id`
  - `last_effective_group_at`
  - `last_effective_group`
- auth cache snapshot 增加：
  - `group_select_mode`
  - `last_effective_group_id`
  - `last_effective_group_at`
- auth cache snapshot version 从 `12` 提升到 `13`

未实现内容保持不动：

- 未实现自动候选分组 resolver
- 未接入 OpenAI 网关自动选组
- 未修改前端

## TDD Notes

### Red

先新增并运行以下失败测试：

- `TestAPIKeyServiceCreate_OpenAIAutoCheapestAllowsNilGroup`
- `TestAPIKeyServiceUpdate_FixedRequiresGroupWhenSwitchingFromAuto`

首次运行结果为编译失败，缺失点与 brief 一致：

- `CreateAPIKeyRequest` 缺少 `GroupSelectMode`
- `UpdateAPIKeyRequest` 缺少 `GroupSelectMode`
- `ErrGroupRequired` 未定义

### Green

按最小实现补充：

- `backend/internal/service/api_key_service.go`
  - 新增 `ErrGroupRequired`
  - 新增 `normalizeAPIKeyGroupSelectMode`
  - create:
    - 默认归一化到 `fixed`
    - `fixed` 且 `group_id == nil` 返回 `ErrGroupRequired`
    - `openai_auto_cheapest` 时清空 `group_id`
  - update:
    - 支持更新 `group_select_mode`
    - 切回 `fixed` 且无分组时返回 `ErrGroupRequired`
    - auto mode 时清空 `GroupID` / `Group`
- `backend/internal/handler/api_key_handler.go`
  - create/update request 新增 `group_select_mode`
  - 映射到 service request
- `backend/internal/handler/dto/types.go`
  - APIKey DTO 新增 group mode / last effective group 字段
- `backend/internal/handler/dto/mappers.go`
  - 新增 DTO 字段映射
- `backend/internal/service/api_key_auth_cache.go`
  - snapshot 新增 group mode / last effective group 字段
- `backend/internal/service/api_key_auth_cache_impl.go`
  - snapshot version bump to `13`
  - snapshot round-trip 映射新字段

### Additional Test Coverage

补充了字段落地验证：

- `backend/internal/service/api_key_service_cache_test.go`
  - `TestAPIKeyAuthSnapshotRoundTrip_PreservesGroupSelectionMetadata`
- `backend/internal/handler/dto/api_key_mapper_last_used_test.go`
  - `TestAPIKeyFromService_MapsGroupSelectionFields`

## Verification

执行过的命令：

```bash
cd /Volumes/workspace/中转站/sub2api/backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestAPIKeyServiceCreate_OpenAIAutoCheapestAllowsNilGroup|TestAPIKeyServiceUpdate_FixedRequiresGroupWhenSwitchingFromAuto' -count=1
```

结果：

- 预期 RED，编译失败，提示缺少 `GroupSelectMode` 与 `ErrGroupRequired`

执行过的格式化与最终验证：

```bash
cd /Volumes/workspace/中转站/sub2api/backend
gofmt -w internal/service/api_key_service.go internal/service/api_key_auth_cache.go internal/service/api_key_auth_cache_impl.go internal/service/api_key_service_test.go internal/service/api_key_service_cache_test.go internal/handler/api_key_handler.go internal/handler/dto/types.go internal/handler/dto/mappers.go internal/handler/dto/api_key_mapper_last_used_test.go
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestAPIKeyServiceCreate_OpenAIAutoCheapestAllowsNilGroup|TestAPIKeyServiceUpdate_FixedRequiresGroupWhenSwitchingFromAuto|TestAPIKeyAuth' -count=1
GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/dto -count=1
```

结果：

- `ok   github.com/Wei-Shaw/sub2api/internal/service`
- `ok   github.com/Wei-Shaw/sub2api/internal/handler/dto`

## Files Changed

- `backend/internal/service/api_key_service.go`
- `backend/internal/service/api_key_auth_cache.go`
- `backend/internal/service/api_key_auth_cache_impl.go`
- `backend/internal/service/api_key_service_test.go`
- `backend/internal/service/api_key_service_cache_test.go`
- `backend/internal/handler/api_key_handler.go`
- `backend/internal/handler/dto/types.go`
- `backend/internal/handler/dto/mappers.go`
- `backend/internal/handler/dto/api_key_mapper_last_used_test.go`
- `.superpowers/sdd/task-2-report.md`

## Notes / Risks

- 当前阶段只做 API Key mode 字段与校验，不区分平台触发额外逻辑；OpenAI 自动选组解析仍留给 Task 3/4。
- 本次未触碰 `frontend/pnpm-lock.yaml`、`.pnpm-store/`、`frontend/pnpm-workspace.yaml`。

## Fix Review

### Reviewer Important 修复

- 修正 `backend/internal/service/api_key_service.go` 中 `Update` 的分组处理顺序：
  - 当最终 `group_select_mode` 为 `openai_auto_cheapest` 时，先清空 `req.GroupID`、`apiKey.GroupID`、`apiKey.Group`
  - 避免继续按请求中的固定 `group_id` 走 `GetByID` 和权限校验
- 新增回归测试：
  - `TestAPIKeyServiceUpdate_OpenAIAutoCheapestIgnoresRequestedInvalidGroup`
  - 覆盖 `GroupSelectMode=openai_auto_cheapest` 且请求携带无效 `GroupID` 时仍成功更新，并验证 repo capture 中 `GroupID == nil`、`Group == nil`、`GroupSelectMode == openai_auto_cheapest`

### TDD Evidence

先运行：

```bash
cd /Volumes/workspace/中转站/sub2api/backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestAPIKeyServiceUpdate_.*Auto|TestAPIKeyServiceCreate_OpenAIAutoCheapestAllowsNilGroup|TestAPIKeyServiceUpdate_FixedRequiresGroupWhenSwitchingFromAuto' -count=1
```

结果：

- `TestAPIKeyServiceUpdate_OpenAIAutoCheapestIgnoresRequestedInvalidGroup` 失败
- 失败原因为当前实现先执行 `get group`，返回 `GROUP_NOT_FOUND`

修复后再次运行：

```bash
cd /Volumes/workspace/中转站/sub2api/backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestAPIKeyServiceUpdate_.*Auto|TestAPIKeyServiceCreate_OpenAIAutoCheapestAllowsNilGroup|TestAPIKeyServiceUpdate_FixedRequiresGroupWhenSwitchingFromAuto|TestAPIKeyAuth' -count=1
GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/dto -count=1
```

结果：

- `ok   github.com/Wei-Shaw/sub2api/internal/service`
- `ok   github.com/Wei-Shaw/sub2api/internal/handler/dto`

### Commit

- `c09ef0e5` `fix: ignore fixed group when api key uses auto mode`
