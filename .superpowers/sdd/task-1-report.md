# Task 1 Report: Persist Group-Level Refresh And Price Guard Config

## 任务结论

已完成 Task 1 要求范围：为 group 增加上游余额刷新与上游价格倍率上限的持久化字段、迁移、service 校验、repository 映射、DTO 输出、admin handler 入参透传。未实现 runner，也未实现价格拦截策略本身。

## 实际修改文件

- 新增：`backend/migrations/160_group_upstream_price_guard.sql`
- 修改：`backend/ent/schema/group.go`
- 修改：`backend/ent/group.go`
- 修改：`backend/ent/group/group.go`
- 修改：`backend/ent/group/where.go`
- 修改：`backend/ent/group_create.go`
- 修改：`backend/ent/group_update.go`
- 修改：`backend/ent/migrate/schema.go`
- 修改：`backend/ent/mutation.go`
- 修改：`backend/ent/runtime/runtime.go`
- 修改：`backend/internal/service/group.go`
- 修改：`backend/internal/service/admin_service.go`
- 修改：`backend/internal/repository/group_repo.go`
- 修改：`backend/internal/repository/api_key_repo.go`
- 修改：`backend/internal/handler/dto/types.go`
- 修改：`backend/internal/handler/dto/mappers.go`
- 修改：`backend/internal/handler/admin/group_handler.go`
- 修改：`backend/ent/schema/openai_auto_scheduler_schema_test.go`
- 修改：`backend/internal/repository/migrations_schema_integration_test.go`
- 修改：`backend/internal/service/admin_service_group_test.go`
- 修改：`backend/internal/service/payment_config_plans_validation_test.go`

## 核心实现说明

### 1. 数据库迁移

新增迁移 `160_group_upstream_price_guard.sql`，给 `groups` 表增加：

- `upstream_balance_refresh_enabled BOOLEAN NOT NULL DEFAULT FALSE`
- `upstream_balance_refresh_interval_seconds INTEGER NOT NULL DEFAULT 600`
- `upstream_price_max_multiplier DECIMAL(10,4) NOT NULL DEFAULT 0`

并补充中文注释。默认值满足“默认关闭 / 默认不限价”，保证历史分组行为不变。

### 2. Ent Schema 与生成代码

在 `backend/ent/schema/group.go` 为 `Group` 增加三个字段：

- `upstream_balance_refresh_enabled`
- `upstream_balance_refresh_interval_seconds`
- `upstream_price_max_multiplier`

随后执行 `go generate ./ent` 同步生成代码，确保 repository 可以直接使用新的 setter / getter。

### 3. Service 层模型与校验

在 `backend/internal/service/group.go`：

- `Group` 增加：
  - `UpstreamBalanceRefreshEnabled`
  - `UpstreamBalanceRefreshIntervalSeconds`
  - `UpstreamPriceMaxMultiplier`
- 增加常量：
  - `DefaultUpstreamBalanceRefreshIntervalSeconds = 600`
  - `MinUpstreamBalanceRefreshIntervalSeconds = 60`
- 增加统一校验函数：
  - `ValidateGroupUpstreamPriceGuardConfig(enabled, intervalSeconds, maxMultiplier)`

校验规则按 brief 落地：

- `upstream_price_max_multiplier < 0` 拒绝
- `upstream_price_max_multiplier = 0` 表示不限价
- 仅当 `enabled=true` 时要求 `intervalSeconds >= 60`

### 4. AdminService 写路径接入

在 `backend/internal/service/admin_service.go`：

- `CreateGroupInput` / `UpdateGroupInput` 增加三个字段
- `CreateGroup` 中接入统一校验
- `UpdateGroup` 中对合并后的最终值做统一校验
- `CreateGroup` 额外按 brief 处理：
  - 当 `enabled=false` 且 `interval=0` 时，自动回填 `600`

这样可以保证 create/update 入口都在 service 层统一兜住约束。

### 5. Repository 持久化与实体映射

在 `backend/internal/repository/group_repo.go`：

- `Create` / `Update` builder 增加三个字段的 `Set...`

在 `backend/internal/repository/api_key_repo.go` 的 `groupEntityToService`：

- 将 Ent entity 上的新字段映射回 `service.Group`

### 6. DTO 与 Handler 入参

在 `backend/internal/handler/dto/types.go` 与 `mappers.go`：

- group DTO 增加三个 JSON 字段并输出

在 `backend/internal/handler/admin/group_handler.go`：

- `CreateGroupRequest` 增加：
  - `upstream_balance_refresh_enabled`
  - `upstream_balance_refresh_interval_seconds`
  - `upstream_price_max_multiplier`
- `UpdateGroupRequest` 增加对应指针字段
- Create / Update 时透传给 service input

## 测试与验证

### TDD / RED 阶段

先补充了以下测试断言：

- `backend/ent/schema/openai_auto_scheduler_schema_test.go`
- `backend/internal/repository/migrations_schema_integration_test.go`
- `backend/internal/service/admin_service_group_test.go`

在尝试按 brief 原命令执行时发现两点仓库现实情况：

1. `internal/service` 与 `internal/repository` 的目标测试分别带有 `unit` / `integration` build tags，原命令不带 tags 时不会真正命中这些测试。
2. `internal/service` 现有测试中存在一个历史性的 helper 重名问题，导致 package 编译失败，阻塞本任务校验。

### 为验证做的最小额外修正

为不影响业务逻辑、又能完成本任务验证，我做了一个最小测试修正：

- `backend/internal/service/payment_config_plans_validation_test.go`
  - 将局部测试 helper `ptrInt64` 重命名为 `ptrPlanInt64`

这只是测试侧命名去冲突，不涉及生产代码行为。

### 最终实际执行并通过的命令

```bash
cd /Volumes/workspace/中转站/sub2api/backend
GOPATH=/tmp/sub2api-go GOCACHE=/tmp/sub2api-go-cache GOMODCACHE=/tmp/sub2api-go-modcache go generate ./ent
GOPATH=/tmp/sub2api-go GOCACHE=/tmp/sub2api-go-cache GOMODCACHE=/tmp/sub2api-go-modcache go test ./ent/schema -run '^TestGroupUpstreamPriceGuardSchemaFields$'
GOPATH=/tmp/sub2api-go GOCACHE=/tmp/sub2api-go-cache GOMODCACHE=/tmp/sub2api-go-modcache go test -tags integration ./internal/repository -run '^TestMigrationsRunner_IsIdempotent_AndSchemaIsUpToDate$'
GOPATH=/tmp/sub2api-go GOCACHE=/tmp/sub2api-go-cache GOMODCACHE=/tmp/sub2api-go-modcache go test -tags unit ./internal/service -run '^(TestValidateGroupUpstreamPriceGuardConfig|TestAdminService.*Group)$'
GOPATH=/tmp/sub2api-go GOCACHE=/tmp/sub2api-go-cache GOMODCACHE=/tmp/sub2api-go-modcache go test -tags 'unit integration' ./ent/schema ./internal/repository ./internal/service -run 'TestGroupUpstreamPriceGuardSchemaFields|TestMigrationsRunner_IsIdempotent_AndSchemaIsUpToDate|TestValidateGroupUpstreamPriceGuardConfig|TestAdminService.*Group'
```

结果：

- `github.com/Wei-Shaw/sub2api/ent/schema` 通过
- `github.com/Wei-Shaw/sub2api/internal/repository` 通过
- `github.com/Wei-Shaw/sub2api/internal/service` 通过

## 风险与注意事项

1. 本任务只完成“配置落库、校验、读写链路接通”，没有实现真正的刷新执行器和价格拦截策略。
2. `upstream_balance_refresh_enabled=false` 时，校验允许 interval 保留任意非负值；create 入口仅在 `enabled=false && interval=0` 时回填 600，符合 brief 要求。
3. 为完成 service 测试验证，额外修复了一个现有测试 helper 重名问题；这不是业务变更，但会体现在本次 commit 中。

## 提交信息

按 brief 要求使用：

```bash
feat: add group upstream price guard config
```

---

## Task 1 Review Fix Append

### 修改内容

- 在 `backend/internal/repository/api_key_repo.go` 的 `GetByKeyForAuth` 中补充了 group 的三个白名单字段：
  - `group.FieldUpstreamBalanceRefreshEnabled`
  - `group.FieldUpstreamBalanceRefreshIntervalSeconds`
  - `group.FieldUpstreamPriceMaxMultiplier`
- 在 `backend/internal/repository/api_key_repo_messages_dispatch_unit_test.go` 新增 SQLite 回归测试，验证 auth 查询能保留这三个 group 字段的非默认值。

### 验证命令

```bash
cd /Volumes/workspace/中转站/sub2api/backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run 'TestAPIKeyRepository_GetByKeyForAuth_(PreservesMessagesDispatchModelConfig|PreservesUpstreamPriceGuardConfig)_SQLite'
```

### 结果

- `ok   github.com/Wei-Shaw/sub2api/internal/repository 0.666s`
