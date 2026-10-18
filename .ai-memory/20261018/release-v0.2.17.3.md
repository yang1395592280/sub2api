# v0.2.17.3 发布准备

用户明确要求打 tag v0.2.17.3、提交并推送。将自定义菜单默认图标修复提交为 51c4e0052ed641084f649870ccb99b19f0b7725f，再从 feature/custom-menu-default-icon 无冲突快进合入 custom-main；更新 backend/cmd/server/VERSION 为 0.2.17.3。

- 基线 v0.2.17.2=076d8c8ee9624796718765f3114e41da00fdf68c；fetch origin 后确认远端 custom-main 仍在基线，目标 tag 本地和远端均不存在。
- 本补丁为未上传 SVG 的自定义菜单提供 terminal 默认图标，空白 SVG 按缺省处理，已有上传图标优先。覆盖用户、管理员个人区和管理员普通/简洁模式，无新增接口或数据库迁移；继续包含 v0.2.17.2 的业务功能。
- 图标修复已验证：侧栏现有 12 项回归、ESLint、typecheck、前端 build（含 3 项双语完整性检查）通过；实际组件配模拟菜单验证默认/上传/空白 SVG、管理员、深色和折叠显示。证据详见同日 daily.md。
- 本轮 `go build -tags=embed -ldflags='-s -w -X main.BuildType=release' -o /tmp/sub2api-release-v0.2.17.3 ./cmd/server` 通过，产物 `-version` 显示 Sub2API 0.2.17.3；版本解析输出 0.2.17.3，发布工具四段版本和发布必须绑定 tag 两项测试通过，`git diff --check` 通过。
- 创建 annotated tag 后执行真实 release plan，核对 HEAD/tag/VERSION，再仅原子推送 origin/custom-main 与 refs/tags/v0.2.17.3。保留旧 tag，不推其他 refs。
- 远端 RELEASE_ONLY=true，沿用 GitHub Release 归档及 checksums，不发布 Docker/GHCR，不修改 CI 配置或仓库变量。

本条为推送前记录，最终提交、远端分支与 peeled tag commit、Release 工作流状态以推送后核验为准。未部署生产；更新前端构建后图标修复才会生效。
