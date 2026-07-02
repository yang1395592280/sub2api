# Task 2 Report

时间：2026-07-02

## 结论

已完成 Task 2 的后端实现：

- 新增管理端手动签到接口 `POST /api/v1/admin/accounts/:id/upstream-checkin/test`
- 将 `Sub2APICheckinService` 注入 `AccountHandler`
- 将 `Sub2APICheckinService` 接入 Wire provider 与 server cleanup 生命周期
- 补充路由、handler、provider/cleanup 相关测试

## 关键改动

### 1. 管理端手动签到接口

文件：

- `backend/internal/handler/admin/account_handler.go`
- `backend/internal/server/routes/admin.go`

改动：

- 在 `AccountHandler` 中新增 `sub2APICheckinService` 依赖
- 新增 `TestUpstreamCheckin` handler
- 在 admin accounts 路由下暴露 `POST /:id/upstream-checkin/test`
- 复用 `buildAccountResponseWithRuntime` 返回更新后的账号响应

### 2. Wire 与生命周期接线

文件：

- `backend/internal/service/wire.go`
- `backend/cmd/server/wire.go`
- `backend/cmd/server/wire_gen.go`

改动：

- 新增 `ProvideSub2APICheckinService`
- provider 内使用现有 `OpenAIUpstreamBalanceService` 复用 HTTP client/鉴权逻辑
- provider 创建后立即 `Start()`
- 在 `provideCleanup` 中新增 `Sub2APICheckinService.Stop()`

### 3. 测试

新增：

- `backend/internal/server/routes/admin_checkin_test.go`
- `backend/internal/handler/admin/account_handler_checkin_test.go`

补充：

- `backend/internal/service/wire_test.go`
- `backend/cmd/server/wire_gen_test.go`

兼容性调整：

- 若干 `NewAccountHandler(...)` 测试调用补齐新参数

## 风险与约束核对

- 第一版仅走现有 sub2api 上游管理凭据链路，未扩展其他 provider
- 未新增数据库表
- handler 未主动记录敏感凭据
- 返回仍复用既有 account DTO/response 逻辑

## 实际执行的验证

1. 任务说明要求测试

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/server/routes ./internal/handler/admin -run 'TestRegisterAdminRoutes_ExposesUpstreamCheckinTest|TestAccountHandler_TestUpstreamCheckinReturnsUpdatedAccount' -v
```

结果：通过

2. 生命周期相关补充测试

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./cmd/server ./internal/service ./internal/server/routes ./internal/handler/admin -run 'TestProvideCleanup_WithMinimalDependencies_NoPanic|TestProvideSub2APICheckinService_StartsWorker|TestRegisterAdminRoutes_ExposesUpstreamCheckinTest|TestAccountHandler_TestUpstreamCheckinReturnsUpdatedAccount' -v
```

结果：通过

## 提交建议

按任务说明使用：

```bash
git commit -m "feat: expose sub2api checkin endpoint"
```
