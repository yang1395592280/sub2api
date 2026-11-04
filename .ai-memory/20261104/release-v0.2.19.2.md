# v0.2.19.2 发布记录

- 授权：用户明确要求打tag v0.2.19.2、提交并推送。采用用户指定的四段版本，保留所有既有tag。
- 起始状态：custom-main=00d7abba5，修复已从feature/fixed-group-account-failover快进合入，无冲突，工作区干净。实时查询远端custom-main=b22f8bd40；远端无v0.2.19.2，也无同名Release。
- 内容：固定OpenAI分组池模式失败后优先尝试符合现有调度规则的替代账号；无替代时保留池重试。补齐HTTP200 SSE已知网关forbidden和Chat/Messages非流式错误识别；保留分组权限/倍率、切换上限、原账号延续及已输出正文不重放保护。
- 版本：backend/cmd/server/VERSION从0.2.19.1更新至0.2.19.2；不修改前端package版本、工作流或依赖。
- 验证：resolve-version.sh输出0.2.19.2，sh语法和git diff --check通过；CGO_ENABLED=0的embed发布构建通过，未注入main.Version，二进制-version输出Sub2API 0.2.19.2。四段版本支持和tag/commit绑定两项既有发布工具测试通过，PyYAML==6.0.3只安装于临时venv。
- 业务验证：复用本会话已通过的handler/service/apicompat完整unit包（11项按原条件跳过）和最终定向race检查，本轮只调整版本及发布记录。
- 推送：提交版本记录，创建annotated tag，再以git push --atomic同时推送origin/custom-main与refs/tags/v0.2.19.2；只操作这两个ref，不覆盖已有tag。
- 工作流：实时查询RELEASE_ONLY=true；标签推送触发现有Release流程构建归档及checksums，跳过Docker/GHCR发布；不修改仓库变量。
- 边界：不部署、不操作生产数据库、不调用真实供应商；真实故障请求未回放。
- 本条为推送前检查点，最终提交点、远端分支/peeled tag及Release运行状态以Git/GitHub核验为准。
