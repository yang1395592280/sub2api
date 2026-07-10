# Task 4B Report: 臻享礼遇后台 HTTP API

## 状态

完成。后台 API、用户查询 API 以及所需的最小 service/repository 查询能力已落地。

## 改动文件

- `backend/internal/handler/admin/zenxiang_liyu_handler.go`
- `backend/internal/handler/admin/zenxiang_liyu_handler_test.go`
- `backend/internal/handler/zenxiang_liyu_handler.go`
- `backend/internal/handler/zenxiang_liyu_handler_test.go`
- `backend/internal/handler/handler.go`
- `backend/internal/handler/wire.go`
- `backend/internal/server/routes/user.go`
- `backend/internal/server/routes/admin.go`
- `backend/internal/service/zenxiang_liyu_service.go`
- `backend/internal/service/zenxiang_liyu_service_test.go`
- `backend/internal/repository/zenxiang_liyu_repo.go`
- `backend/cmd/server/wire_gen.go`

## 接口清单

用户接口：

- `GET /api/v1/zenxiang-liyu/status`
- `POST /api/v1/zenxiang-liyu/play`
- `GET /api/v1/zenxiang-liyu/records`
- `GET /api/v1/zenxiang-liyu/daily-summary`

后台接口：

- `GET|PUT /api/v1/admin/zenxiang-liyu/settings`
- `GET|POST /api/v1/admin/zenxiang-liyu/prizes`
- `PUT /api/v1/admin/zenxiang-liyu/prizes`：完整奖项配置替换，遗漏的旧奖项将由 `SavePrizes` 禁用。
- `PUT|DELETE /api/v1/admin/zenxiang-liyu/prizes/:id`
- `GET|POST /api/v1/admin/zenxiang-liyu/grants`
- `DELETE /api/v1/admin/zenxiang-liyu/grants/:user_id`
- `GET /api/v1/admin/zenxiang-liyu/stats/overview`
- `GET /api/v1/admin/zenxiang-liyu/stats/users`
- `GET /api/v1/admin/zenxiang-liyu/stats/prizes`
- `POST /api/v1/admin/zenxiang-liyu/simulate`
- `POST /api/v1/admin/zenxiang-liyu/simulate/recommend`
- `POST /api/v1/admin/zenxiang-liyu/simulate/apply`

`simulate/apply` 仅应用提交的完整奖项配置，复用 `SavePrizes` 的事务性替换语义；不会写入真实用户余额或真实参与流水。

## 测试命令和结果

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache GOMODCACHE=/tmp/sub2api-go-modcache go generate ./cmd/server
```

结果：成功，更新 `cmd/server/wire_gen.go`。

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache GOMODCACHE=/tmp/sub2api-go-modcache go test ./internal/handler ./internal/handler/admin ./internal/server/routes -run 'TestZenxiangLiyu|Test.*Route' -count=1
```

结果：通过。

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache GOMODCACHE=/tmp/sub2api-go-modcache go test ./internal/service ./internal/repository -run 'TestZenxiangLiyu' -count=1
```

结果：通过。

## 风险和关注点

- 奖项单项 POST/PUT 为兼容接口；创建或修改会保持现有配置总概率校验。后台编辑完整档位时应使用 `PUT /prizes`，确保一次提交完整配置。
- records、daily-summary、stats 和 grants 查询均为只读 SQL，不会修改用户余额或参与流水。
- 本任务未改动已审查的事务性 Play 核心流程。
