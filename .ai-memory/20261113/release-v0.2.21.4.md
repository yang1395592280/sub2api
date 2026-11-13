# v0.2.21.4 发布记录

- 授权：用户明确要求“打 tag v0.2.21.4、提交推送”。
- 起始状态：`custom-main=d453d60fb`，工作区在版本修改前干净；`origin/custom-main=99d8fb838`；远端没有 `v0.2.21.4` 标签。
- 版本：`backend/cmd/server/VERSION` 从 `0.2.21.3` 更新为 `0.2.21.4`。
- 内容：顶栏每日签到弹窗、中奖/未中奖翻牌效果；刷新余额时获取 sub2api/New API 上游 API Key 当日实际消费、充值比例折算成本和上游账号总额；账号页初始化顺序回归修复；补充文档与测试。
- 验证：上一提交已通过 Go service short 7759 项、定向消费/余额 48 项、race、build/vet；前端相关 38 项、最后签到 6 项、ESLint、类型检查和正式构建；模拟 API 桌面与 390px 浏览器验收。版本发布前另执行 `resolve-version.sh`、shell 语法检查和 embed 构建。
- 推送：创建 annotated tag `v0.2.21.4`，使用 `git push --atomic` 推送 `origin/custom-main` 与 `refs/tags/v0.2.21.4`，不 force。
- 工作流：tag 会触发现有 Release 工作流；当前仓库配置 `RELEASE_ONLY=true` 时跳过 Docker/GHCR，仅生成归档、checksums 和 GitHub Release。本次不部署、不执行生产迁移。
- 上线边界：每日消费当前是北京时间快照，不是历史账本；真实上游凭据和定制接口需上线后联调；New API 超过 10000 条或查询不完整时不提供部分合计。
