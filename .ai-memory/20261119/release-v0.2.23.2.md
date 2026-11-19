# v0.2.23.2 发布记录

- 授权：用户明确要求打 tag `v0.2.23.2`、提交并推送。
- 基线：`custom-main` 已合并签到日历正负金额配色，合并提交 `d246c115b`，功能提交 `a919a17a5`。
- 范围：正数与零值沿用绿色；负数金额、图标、边框及背景改为玫红；修正日历深色模式样式选择器。
- 版本：`backend/cmd/server/VERSION` 从 `0.2.23.1` 更新为 `0.2.23.2`。
- 发布提交：`4a1e9258213acd4ee38c367eee43f101805e528c`；annotated tag 为 `v0.2.23.2`，tag object 为 `4d8c80282bc6dcf86ba4450e9aeef0f0153897c0`。
- 本地验证：版本解析脚本输出 `0.2.23.2`；`go build -tags embed -trimpath -o /tmp/sub2api-v0.2.23.2 ./cmd/server` 通过，`-version` 输出 `Sub2API 0.2.23.2`；`git diff --check` 通过。功能阶段的签到组件 6 项测试、ESLint、前端 build（含 3 项 i18n 和 vue-tsc）以及桌面/390px 浅深色实际组件验证已通过，使用本次未再修改的配色代码。
- 推送：`git push --atomic origin custom-main refs/tags/v0.2.23.2` 成功；远端 tag peeled commit 与发布提交一致。
- Release 工作流：已触发，记录时为 `in_progress`，尚未确认制品发布完成；https://github.com/yang1395592280/sub2api/actions/runs/44445291397 。
- 未执行：生产部署、数据库迁移；本次未修改后端业务逻辑或生产配置。
