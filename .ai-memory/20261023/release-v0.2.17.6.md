# v0.2.17.6 发布准备

用户明确授权更新版本、提交、创建 `v0.2.17.6` tag 并推送。本次福利活动限领功能已在 `12fe0026739e6ec6ebb9ffa590d984d1941f4dc9` 无冲突快进合入本地 `custom-main`，合并后工作区干净。

- 发布内容：同一活动的余额码、抽奖券码按站内账号共享成功兑换次数，默认1次、可创建和后续修改为1–1000次；支持补发共享额度、并发防超领、失败回滚和删除码不恢复次数。旧码不追溯关联活动。
- 版本：`backend/cmd/server/VERSION` 更新为 `0.2.17.6`，创建 annotated tag，核对版本、HEAD与tag一致后原子推送 `origin/custom-main` 和 `refs/tags/v0.2.17.6`。
- 远端预检：`origin/custom-main` 为 `9e5d786a42f0877de5791f91c75fd6104441f9ed`，新tag尚不存在；使用现有SSH连接成功，无网络持久配置变更。
- 发布模式：仓库变量 `RELEASE_ONLY=true`，tag触发已有Release工作流，仅发布安装归档及校验文件。本轮不发布镜像、不部署、不执行生产迁移。

验证：`CGO_ENABLED=0 go build -tags=embed -ldflags='-s -w -X main.BuildType=release' -trimpath -o /tmp/sub2api-v0.2.17.6 ./cmd/server` 成功，二进制 `-version` 输出 `0.2.17.6`。9项非镜像发布工具测试、版本解析及 `bash -n .github/release-tools/release-images.sh` 通过。测试按现有requirements将PyYAML6.0.3安装至临时目录，无仓库依赖或系统包修改；本机Bash3.2未执行两个镜像脚本测试方法，此次发布不包含镜像。业务回归、真实PostgreSQL并发/race、前端32项及前后端构建沿用本次功能验证结果。

上线注意：新增迁移247由新版本现有迁移机制应用；生产未在开发阶段修改。回退旧应用前应暂停或禁用尚未使用的活动码，并保留领取计数，旧版无法执行活动限额。使用与回退说明见 `docs/redeem-campaign-limits.md`。

本记录为推送前状态；最终提交、远端refs及Release状态以后续核验为准。既有tag不移动、不覆盖。
