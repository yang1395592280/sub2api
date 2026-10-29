# v0.2.20 发布记录

- 授权：用户明确要求打 tag v0.2.20、提交并推送。
- 起始状态：custom-main=672d45643，工作区干净；远端 custom-main=b3737dc4b，尚无 v0.2.20 标签。远端主线为本地主线祖先，无需覆盖历史。
- 内容：包含已合入的上游 v0.2.19（a3298b46e）及糖果说明、鹈鹕评审标准和状态展示等二改；14 个上游同步冲突与质量身份缺失字段兼容问题已解决。
- 版本：backend/cmd/server/VERSION 从 0.2.19 更新为 0.2.20。
- 本轮验证：resolve-version.sh 输出 0.2.20，脚本语法检查、embed Go 构建通过，构建产物 -version 输出 Sub2API 0.2.20。业务代码未再修改，沿用本次合并的全量 unit、定向 race、go vet、前端 607+209 项测试、lint/typecheck、前后端构建和隔离页面冒烟证据。
- 计划：提交版本与本记录，创建 annotated tag；通过 git push --atomic 一次推送 origin/custom-main 与 refs/tags/v0.2.20，保留所有已发布标签与分支历史。
- 工作流：本轮实时查询确认 RELEASE_ONLY=true。标签推送会触发现有 Release 工作流生成 GitHub Release 和安装包，不发布镜像；不手动更改其他分支或仓库变量。
- 发布边界：不执行生产部署、数据库迁移或功能启用。新增图片按张计费迁移默认关闭；上线前备份数据库并在测试环境验证。双检测仍需私网渲染器及视觉评审配置。
- 限制：真实 PostgreSQL/Redis、付费上游、管理员登录后操作和 golangci-lint 未验证，详见 upstream-sync.md；本轮没有扩大检查或更改测试断言。
- 本条为推送前检查点；最终版本提交、标签和远端 refs、Release 工作流状态以后续 Git/GitHub 实时核验为准。
