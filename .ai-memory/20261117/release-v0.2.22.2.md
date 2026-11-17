# v0.2.22.2 发布记录

- 授权：用户明确要求打 tag `v0.2.22.2`、提交并推送。
- 起始状态：`custom-main` 已合并 `feature/size-bet-game`（合并提交 `f018a1aa2`），完整实现比大小游戏与游戏中心迁移、纯福利余额对接、签到区间缩小至 0.01~0.10、游戏总开关与后台维护模式、拟物筹码与动效。
- 版本：`backend/cmd/server/VERSION` 从 `0.2.22.1` 更新为 `0.2.22.2`。
- 本轮验证：
  - `backend/scripts/resolve-version.sh` 输出 `0.2.22.2`。
  - 后端单元测试通过：`go test ./internal/service -run TestSizeBet`。
  - 后端全量构建成功：`go build ./cmd/server`。
  - 前端多语言完整性：`pnpm run check:i18n` 3 项全部通过。
  - 前端类型检查：`pnpm typecheck`（`vue-tsc --noEmit`）无错误通过。
  - 前端生产构建：`pnpm build` 输出成功。
- 推送范围：发布提交后创建 annotated tag `v0.2.22.2`，使用 `git push` 推送至 `origin/custom-main` 与该 tag。
