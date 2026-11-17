# v0.2.22.3 发布记录

- 授权：用户明确要求打 tag `v0.2.22.3`、提交并推送。
- 修复内容：
  - 修复 [game_center_repo.go](file:///Volumes/workspace/中转站/sub2api/backend/internal/repository/game_center_repo.go) 中 `buildGamePointsWhere` 函数对 `StartTime` 与 `EndTime` 仅添加到 `args` 而缺少 SQL 条件子句导致带时间过滤时报错 500 的 Bug；
  - 增加单元测试 `game_center_repo_test.go` 保证 SQL 构建与参数绑定正确无误。
- 版本：`backend/cmd/server/VERSION` 从 `0.2.22.2` 更新为 `0.2.22.3`。
- 本轮验证：
  - `go test -v ./internal/repository -run TestBuildGamePointsWhere` 全部通过。
  - `go test ./internal/service -run TestSizeBet` 全部通过。
  - `go build ./cmd/server` 全量编译成功。
- 推送范围：提交后打 annotated tag `v0.2.22.3` 并推送到 `origin/custom-main`。
