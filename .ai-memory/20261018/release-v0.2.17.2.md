# v0.2.17.2 发布准备

用户明确要求打 tag v0.2.17.2、提交并推送。功能提交 6cc187d5b3fd4613ec87ca8670a422f974fe5ffe 已从 feature/custom-controls-active-monitor 无冲突快进合入 custom-main；本次仅更新 backend/cmd/server/VERSION 为 0.2.17.2 并追加发布记录。

- 发布包含集中二改开关、V1 分组账号主动监控与内嵌鹈鹕检测，管理员可查看账号及上游消息/请求 ID；迁移 246 新增监控持久化表，新计划默认暂停、鹈鹕总开关默认关闭。
- fetch origin 后确认 origin/custom-main=e4fe283108ddd45deac6af81ae380e4a52027f4d，本地领先一个功能提交，可正常快进；本地及远端均无 v0.2.17.2。
- `go build -tags=embed -ldflags='-s -w -X main.BuildType=release' -o /tmp/sub2api-release-v0.2.17.2 ./cmd/server` 通过，产物 `-version` 输出 Sub2API 0.2.17.2；`sh backend/scripts/resolve-version.sh` 输出 0.2.17.2；`git diff --check` 通过。
- 发布工具四段版本与发布必须绑定 tag 的两项 unittest 通过。业务实现复用本日已通过的 Go/前端定向回归、类型/质量/构建检查、两项隔离 PostgreSQL 18.1/Redis 集成及模拟 API 浏览器交互验证，详见 daily.md 和 docs/channel-monitor-v1-group-design.md。
- 创建 annotated tag 后执行真实 release plan，核对 tag、HEAD 与 VERSION；仅原子推送 origin/custom-main 和 refs/tags/v0.2.17.2，旧 tag 不移动，不推其他分支或 tags。
- 远端 RELEASE_ONLY=true，沿用现有 GitHub Release 归档及 checksums 流程，不发布 Docker/GHCR；不修改 CI 配置或仓库变量。

本条为推送前记录，实际发布提交、远端 refs 和 Release 工作流状态以推送后核验为准。未执行生产部署或迁移，真实供应商与长期调度尚未验证；直接账号抽测消耗上游额度，仅支持普通分组。
