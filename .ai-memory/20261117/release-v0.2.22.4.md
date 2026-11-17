# v0.2.22.4 发布记录

- 授权：用户明确要求打 tag `v0.2.22.4`、提交并推送。
- 修复与优化内容：
  1. **游戏中心控制开关**：`game_center_enabled` 纳入 `PublicSettings`、SSR 注入与前端 `FeatureFlags`，开关关闭时自动隐藏菜单、页面展示维护状态；
  2. **移除用户积分排行榜**：用户侧游戏中心彻底移除积分排行榜区域及相关逻辑；
  3. **福利余额筹码兑换开关**：兑入与兑出面板严格受控于 `balance_to_points_enabled` / `points_to_balance_enabled`，关闭时提供暂停提示并禁用操作；
  4. **签到弹窗交互**：修复跳转 `/checkin` 导致的 404，在游戏中心内直接就地弹出 `ProfileCheckinCard` 签到，并在路由中增加 `/checkin` 重定向兜底；
  5. **文案更名**：用户侧导航菜单、卡片、走势及游戏页面统一更名为“猜大小”；
  6. **统计用户名脱敏**：后端接口与前端表格同时对参与用户的用户名及邮箱进行中间字符遮盖脱敏，保护隐私；
  7. **快速兑换比例显示**：猜大小快速兑换弹窗按后台实际 `balance_to_points_rate` 汇率计算并显示筹码数；
  8. **多套界面样式与后台切换**：默认恢复老源码干净清爽的经典现代卡片设计（`classic`），同时提供至尊赌场（`casino`）与赛博霓虹（`neon`）风格，支持管理员后台保存配置并支持前端实时预览切换。
- 版本：`backend/cmd/server/VERSION` 从 `0.2.22.3` 更新为 `0.2.22.4`。
- 本轮验证：
  - `pnpm typecheck`：通过；
  - `pnpm run check:i18n`：通过；
  - `pnpm build`：构建成功；
  - `go test ./internal/handler/dto -run TestPublicSettingsInjectionPayload_SchemaDoesNotDrift`：通过；
  - `go test ./internal/service -run "TestSizeBet"`：全部通过；
  - `go build -o server ./cmd/server`：编译成功。
- 推送范围：提交后打 annotated tag `v0.2.22.4` 并推送到 `origin/custom-main`。
