# v0.2.23.1 发布记录

- 授权：用户明确要求打 tag `v0.2.23.1`、提交并推送。
- 基线：`custom-main` 已包含上游 `0.2.23` 同步提交 `18912179e` 及游戏中心积分兑换功能。
- 版本：`backend/cmd/server/VERSION` 从 `0.2.23` 更新为 `0.2.23.1`。
- 发布提交：`560944a8443f459c202cea7d0a35abc2ae9a6f61`，annotated tag 为 `v0.2.23.1`。
- 验证：版本解析脚本输出 `0.2.23.1`；带 `-X main.Version=0.2.23.1` 的 embed 后端构建及 `-version` 输出通过；此前同步回归已通过。`.github/release-tools` 单测因本机缺少 `PyYAML` 未执行成功。
- 推送：`origin/custom-main` 与 `refs/tags/v0.2.23.1` 已推送并核验，tag peeled commit 与发布提交一致。
- 未执行：生产部署、数据库迁移和真实供应商请求。
