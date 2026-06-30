## Task 3 Report

### 变更结论

已按 brief 完成 OpenAI 自动候选分组 resolver 与 helper 的最小实现，未接入 OpenAI gateway，未修改 handler，未触碰前端文件。

### 实际修改文件

- `backend/internal/service/openai_auto_cheapest_group_service.go`
- `backend/internal/service/openai_auto_cheapest_group_service_test.go`

### 实现内容

1. 新增 `OpenAIAutoCheapestGroupResolver`
   - 暴露 `CandidateGroups(ctx, userID)`。
   - 通过 `GetAvailableGroups` + `GetUserGroupRates` 构建候选分组。
   - 仅返回 `StatusActive` 且 `PlatformOpenAI` 的分组。
   - 排序规则：
     - 用户专属倍率优先覆盖分组倍率；
     - 按有效倍率升序；
     - 再按 `SortOrder` 升序；
     - 最后按 `ID` 升序稳定排序。

2. 新增 `EffectiveOpenAIGroupRate`
   - 优先读取用户专属倍率；
   - 无覆盖时回退到 `Group.RateMultiplier`。

3. 新增 `CloneAPIKeyForEffectiveGroup`
   - 保留原 API Key 身份字段与选择模式；
   - 将 `GroupID` / `Group` 切换为实际生效分组；
   - 返回克隆副本，不原地修改入参。

4. 字段映射核查
   - `service.Group` 已存在 `SortOrder`；
   - `backend/internal/repository/api_key_repo.go` 中 `groupEntityToService` 已映射 `SortOrder`；
   - 因此本任务无需额外修改 repository / service 字段映射。

### TDD 过程

1. 先新增失败测试：
   - `TestEffectiveOpenAIGroupRate_UsesUserRateOverride`
   - `TestCloneAPIKeyForEffectiveGroup_UsesActualGroup`
   - `TestCandidateGroups_FiltersOpenAIAndSortsByEffectiveRate`
   - `TestCandidateGroups_TieBreaksBySortOrderThenID`

2. 首次运行结果：
   - 因 `EffectiveOpenAIGroupRate`、`CloneAPIKeyForEffectiveGroup`、`NewOpenAIAutoCheapestGroupResolver` 不存在而失败，符合预期 RED。

3. 最小实现通过后再次运行 targeted tests，结果通过。

### 验证命令

1. RED：

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestEffectiveOpenAIGroupRate|TestCloneAPIKeyForEffectiveGroup|TestCandidateGroups' -count=1
```

结果：失败，报未定义符号，证明测试先行生效。

2. GREEN：

```bash
cd backend
gofmt -w internal/service/openai_auto_cheapest_group_service.go internal/service/openai_auto_cheapest_group_service_test.go
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestEffectiveOpenAIGroupRate|TestCloneAPIKeyForEffectiveGroup|TestCandidateGroups' -count=1
```

结果：通过。

### 风险与注意事项

- 本任务仅提供候选分组解析与 API Key 生效分组克隆能力，尚未接入 gateway 选择链路。
- 当前未实现候选顺序缓存，符合 brief 的“非强制实现缓存”要求。
- `CandidateGroups` 依赖 provider 上游已正确过滤“用户可用且 active 的 groups”；当前实现仍再次限定 `PlatformOpenAI` 和 `StatusActive`，避免混入非目标分组。
