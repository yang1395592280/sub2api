# v0.2.21.6 发布记录

- 授权：用户明确要求打 tag `v0.2.21.6`、提交并推送。
- 起始状态：`custom-main=9e0455d8b`，工作区干净；`origin/custom-main=fd6b93ec1`；本地及远端均无 `v0.2.21.6` 标签。
- 版本：`backend/cmd/server/VERSION` 从 `0.2.21.5` 更新为 `0.2.21.6`。
- 内容：鹈鹕测试上游余额消耗估算；固定离线糖果智力题批量/单账号测试；动画预览后的结果列；独立红绿时间轴历史；分组默认关闭、每 30 分钟一次的独立计划。标准答案 20，严格判断最终数字，不影响账号健康或调度评分。
- 已有验证：复用功能提交 `9e0455d8b` 的 Go 定向三包 unit/race、PG18.1/Redis8.4 集成迁移、Wire/vet/embed build、前端27项/ESLint/typecheck/build及桌面/390px模拟API实际交互。
- 本轮验证：版本文件/解析断言、`sh -n backend/scripts/resolve-version.sh`、`CGO_ENABLED=0 go -C backend build -tags embed -trimpath` 和二进制 `-version` 均通过，输出 `0.2.21.6`。发布工具四段版本、tag/commit约束2项通过；系统Python缺PyYAML，使用临时venv安装仓库锁定依赖后完成验证。提交前执行 `git diff --check`。
- 推送：创建 annotated tag `v0.2.21.6`，以 `git push --atomic` 推送 `origin/custom-main` 和该 tag，不 force。最终分支、tag peeled commit 与工作流结果以推送后的核验为准。
- 工作流：已读取 GitHub 仓库变量 `RELEASE_ONLY=true`，tag 推送按现有流程生成归档、checksums 和 GitHub Release，跳过 Docker/GHCR 发布。未主动改动仓库配置或默认分支。
- 边界：不执行生产迁移、部署或真实供应商调用。费用为实际 Token 与本地模型价/上游倍率的估算，非实扣账单。迁移253改变队列唯一键与计划主键，升级需停止旧实例，不能混跑旧二进制或直接降级；详细回退要求见 `docs/intelligence-tests-and-upstream-cost.md`。
