# v0.2.21 发布记录

- 授权：用户明确要求打tag v0.2.21、提交并推送。
- 起始状态：custom-main=8f0a44682，工作区干净；已包含上游同步519ee380e与本轮TTL修复。实时远端custom-main=3e4724636，是本地分支祖先，无远端v0.2.21标签或同名Release。
- 内容：上游v0.2.21与既有二改合并；补齐WebSocket及Chat/Messages转换的30m/12h缓存写入用量传递和正式回归，补全账号页测试mock与运行时翻译测试。
- 版本：backend/cmd/server/VERSION已为0.2.21，不重复修改版本文件或前端package。
- 标签处理：本地v0.2.21是原作者annotated tag，object=43f15294c477bec427d586ddfc3f918ce1284c98、commit=10ba8f9b4dc2e6023d4a9879ab77b21841babd10；先以本地upstream/v0.2.21保留完整tag对象，再创建指向本次发布提交的annotated v0.2.21。上游远端和既有远端标签不作修改。
- 验证：resolve-version.sh输出0.2.21，sh语法通过；CGO_ENABLED=0 embed构建通过，未注入main.Version，二进制-version显示Sub2API 0.2.21。
- 业务验证：复用上一轮后端全量unit、定向race、build/vet及前端3406全量、619关键测试、lint/typecheck/build和本地浏览器验证；本次仅补发布记录。Docker不可用，数据库集成仍跳过。
- 发布工具环境：系统Python缺PyYAML，使用临时venv安装锁定PyYAML==6.0.3；系统Bash3.2不支持Linux CI所用语法，在/tmp构建Bash5.3后完整11项发布工具测试全部通过。不修改项目代码或系统默认shell。
- 推送：提交发布记录后，以git push --atomic仅推送origin/custom-main和refs/tags/v0.2.21；本地upstream/v0.2.21不推送。
- 工作流：实时查询RELEASE_ONLY=true；标签推送触发现有Release构建归档与checksums，跳过Docker/GHCR镜像发布。不修改仓库变量、不部署或执行生产迁移。
- 本记录是推送前检查点；最终提交点、远端分支与peeled tag、Release运行状态以Git/GitHub核验为准。
