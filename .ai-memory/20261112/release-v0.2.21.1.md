# v0.2.21.1 发布记录

- 授权：用户明确要求打 tag v0.2.21.1、提交并推送。
- 起始状态：custom-main=90cd5c71e，已 fast-forward 合入签到与福利分账功能，无冲突；远端 custom-main=6d9e8231e6a6e4e709fd996750f702e7d9501ea2，实时查询无 v0.2.21.1 标签或同名 Release。
- 版本：backend/cmd/server/VERSION 从 0.2.21 更新至 0.2.21.1；无新增业务改动。
- 内容：签到及翻倍、充值与签到/礼遇/管理员赠送福利分账、消费福利最多抵总额三分之一、福利允许负数且负来源不参与消费、前后台分项展示、风控72小时观察清退及复核恢复。旧余额保留原有待遇。
- 本轮验证：resolve-version.sh 输出 0.2.21.1，sh 语法检查通过；11 项 release_matrix 发布工具测试通过；CGO_ENABLED=0 go build -tags embed -trimpath -ldflags '-s -w' 构建通过，生成二进制 -version 显示 Sub2API 0.2.21.1。
- 业务验证：复用功能交付已通过的后端全量 unit/build/vet、定向 race、真实 PostgreSQL/Redis 集成；前端 429 文件 / 3409 测试、lint/typecheck/build；真实后端浏览器及正式内嵌构建中英文/CSP/390px 验证。证据见 welfare-balance-checkin-handoff.md。
- 推送计划：发布提交后创建 annotated tag v0.2.21.1，以 git push --atomic 仅推送 origin/custom-main 与 refs/tags/v0.2.21.1，不 force。
- 工作流：RELEASE_ONLY=true；tag 触发现有归档、checksums 与 GitHub Release 工作流，跳过 Docker/GHCR 镜像发布；现有 sync-version-file job 成功后会自动同步仓库默认分支的版本文件。本任务不修改工作流或默认分支设置。
- 上线事项：包含迁移 251、252；部署前备份，按 docs/welfare-balance-checkin.md 同步前后端并处理缓存、验证码与活动开关。本任务未部署、未执行生产迁移。
- 本记录为推送前检查点；最终提交、远端分支/tag 及自动发布结果以 Git/GitHub 核验为准。
