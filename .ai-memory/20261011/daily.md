## 上游同步：v0.2.15 合入 custom-main

- main 从 3f1a2ea0a 快进到 3a6fd1c9d；以 merge 整合到原 custom-main=d19cc71c6。
- 手工处理 13 个冲突，保留用户二改语义；额外补充合规状态竞态回归并修正嵌入测试的过期资源路径。
- 前端 396 文件/3014 测试、lint、typecheck、build；后端 go test、定向 unit、go vet、普通及嵌入构建均通过。
- 未 push/部署/执行数据库迁移；部署注意 Go 1.27.2 和上游迁移 242。
- 完整文件清单、验证命令、限制与撤销条件见 upstream-sync.md。

## [13:56] Bug 修复：WebSocket 自动调价与鹈鹕自填模型

- 修复自动低价 WebSocket 后续 turn 调价、鹈鹕新增平台遗漏、自适应测试漏传 prompt；鹈鹕弹窗增加自填模型 ID。
- 后端全量 unit/go vet/embed 测试、前端 10 项定向测试/lint/build 通过；浏览器本地请求桩交互验证通过。
- 自填仍遵守账号模型配置；自动模式每轮增加 Lite 分组读取。未 push/部署/真实供应商调用。
- 文件、命令和限制见 bugfix-pelican-model.md。

## [14:04] 发布：v0.2.15.1

- 用户授权提交、打 tag 并推送；范围为 origin/custom-main 和 annotated tag v0.2.15.1，不推送 main/upstream。
- backend/cmd/server/VERSION 从 0.2.15 更新为 0.2.15.1；版本解析、嵌入后端构建及二进制 -version 验证通过。
- 发布工具的四段版本和 tag/commit 绑定两项测试通过；系统 Python 缺 PyYAML，使用临时 venv 安装仓库锁定的 PyYAML==6.0.3 完成检查。
- Tag 发布说明包含上游同步、3 个 Bug 修复及鹈鹕自填模型；Git 推送验收以远端 custom-main 和 peeled tag 均指向发布提交为准。
- Tag 会触发 Release 工作流；Git 推送成功不代表镜像/归档构建成功，需单独核验。
