# v0.2.21.5 发布记录

- 授权：用户明确要求打 tag `v0.2.21.5`、提交并推送。
- 起始状态：`custom-main=f6de6a4f2`，工作区干净；`origin/custom-main=fd78c3971`；远端没有 `v0.2.21.5` 标签。
- 版本：`backend/cmd/server/VERSION` 从 `0.2.21.4` 更新为 `0.2.21.5`，沿用现有版本管理方式。
- 内容：使用记录充值／福利余额详细扣款显示 8 位小数；独立用户每日统计，汇总签到与臻享礼遇奖励、当前余额、充值／福利消费拆分、前日消费、全站消费排名、活动明细和 CSV 导出。
- 验证：本轮 `git diff --check`、`sh -n backend/scripts/resolve-version.sh`、版本解析断言、`CGO_ENABLED=0 go -C backend build -tags embed -trimpath` 均通过；构建后二进制 `-version` 输出 `0.2.21.5`。
- 已有验证：复用两个功能提交已有的前端测试／类型检查／lint／正式构建、Go 相关单元／集成／race／build／vet，以及桌面和 390px 模拟 API 浏览器验收；本轮仅更新版本与发布记录，未重跑全部业务测试。
- 推送方式：创建 annotated tag `v0.2.21.5`，使用 `git push --atomic` 同步 `custom-main` 和指定 tag，不 force；最终远端提交与 tag 以推送后核验结果为准。
- 工作流：已核对仓库变量 `RELEASE_ONLY=true`；tag 推送触发现有 Release 工作流，按当前配置生成归档、checksums 和 GitHub Release，跳过 Docker/GHCR。
- 边界：本轮不执行部署或生产迁移；每日统计无新增数据库迁移，余额表示当前值，历史未分账记录单列，生产数据与生产规模性能仍需上线验证。
