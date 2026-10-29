# v0.2.19.1 发布记录

- 授权：用户明确要求打 tag v0.2.19.1、提交并推送。按指定采用四段版本，保留已发布的 v0.2.20；该版本号低于 v0.2.20，版本比较可能不提示更新。
- 起始状态：custom-main=4ad521c65，工作区干净；该提交已从 feature/group-pelican-history 快进合入，无冲突。实时查询远端 custom-main=1298eb8a6（本地祖先），远端无 v0.2.19.1。
- 内容：分组定时鹈鹕配置、固定30分钟、只生成不判分和不干预调度；账号最新结果和完整分页作品档案；旧质量模块一级菜单及二改总开关。用法与回退见 docs/group-pelican-tests.md。
- 版本：backend/cmd/server/VERSION 从 0.2.20 改为用户指定的 0.2.19.1，不回退代码内容。
- 验证：版本解析输出0.2.19.1、sh语法检查、embed发布构建通过，二进制-version输出Sub2API 0.2.19.1。发布工具四段版本和tag/commit绑定两项测试在临时venv中通过；系统Python缺少PyYAML，已在临时环境安装项目固定版本6.0.3，未修改项目依赖。
- 业务验证复用：后端相关170项、最终配置/HTTP36项、race34项、vet/embed；前端相关130项、最终历史2项、lint/typecheck/build；隔离浏览器和PGlite迁移/SQL通过。真实PostgreSQL多实例锁和真实上游仍待测试环境验证。
- 计划：提交版本及记录，创建 annotated tag，git push --atomic 同时推送 origin/custom-main 和 refs/tags/v0.2.19.1；只推这两个ref，不覆盖旧tag，不推main/upstream。
- 工作流：实时查询确认 RELEASE_ONLY=true。标签推送触发现有Release流程生成归档，不发布Docker/GHCR镜像；不修改仓库变量或工作流。
- 边界：本次不部署、不执行生产迁移或启用分组。迁移249上线前备份并在测试环境验证；历史全量保留会增加数据库空间，回旧版仍有旧清理风险。
- 本条为推送前检查点，最终提交点、远端分支/peeled tag和Release运行状态以Git/GitHub核验为准。
