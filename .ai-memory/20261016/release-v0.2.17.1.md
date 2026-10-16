# v0.2.17.1 发布准备

用户明确要求打tag v0.2.17.1并提交推送。基线custom-main=7543bb1f7（已推送v0.2.16.4），本次仅将backend/cmd/server/VERSION改为0.2.17.1并追加发布记录；业务代码继续包含上游v0.2.17、分组用量刷新及一致快照修复。

- 开始时工作区干净，fetch origin后确认origin/custom-main仍为7543bb1f7，可正常快进；远端没有v0.2.17.1。
- `go build -o /tmp/sub2api-release-v0.2.17.1 ./cmd/server`通过；二进制`-version`显示Sub2API 0.2.17.1。git diff --check通过。
- 业务代码与v0.2.16.4一致，复用既有前端3114项、仓储整包unit、9个日桶/用量集成、相关三包race及并发集成race验证；发布工具未修改，四段版本与tag/commit绑定测试沿用前次已通过结果。
- 创建annotated tag后运行真实release plan，核验tag、HEAD及版本解析均为本次发布；原子推送origin/custom-main和精确tag v0.2.17.1。旧tag不移动，不推其他分支/tags。
- 远端RELEASE_ONLY=true，沿用现有GitHub Release归档流程；未修改CI配置，未执行生产部署或迁移。

本条为推送前记录，实际发布提交、远端refs及Release状态以推送后核验为准。未验证生产负载、真实供应商或支付。
