# v0.2.18.1 发布准备

用户明确要求打 v0.2.18.1 tag、提交并推送；链接后的版本文字按当前 v0.2.18 基线解析为 v0.2.18.1。

- 分支：custom-main，包含上游 v0.2.18 合并（85231d2ce）及重试时长修复（f04e41a45）。
- 版本：backend/cmd/server/VERSION 从 0.2.18 更新为 0.2.18.1；创建 annotated tag，同一发布提交绑定分支与 tag。
- 远端预检：origin/custom-main 为 74b9738363febdeee8280aec5571ddaf3ae7865c，本地领先 50 个提交且无分叉；v0.2.18.1 tag 不存在。
- 推送范围：使用 atomic push 同时推送 origin/custom-main 和 refs/tags/v0.2.18.1；不移动或覆盖既有 tag。
- 发布模式：已读取仓库变量 RELEASE_ONLY=true；tag 触发现有 Release 工作流，发布安装归档及校验文件，不发布镜像。本轮不部署、不执行数据库迁移。
- 业务验证复用本轮已完成证据：前端 410 个文件共 3147 项测试；后端 service、handler、repository unit 测试；ARM64/amd64 各 19 项定向回归；go vet 与 embed 构建全部通过。详见 postmerge-bugfix.md。
- 本次版本验证：提交前重新执行 embed 构建及二进制 -version 校验；提交并打 tag 后检查 resolve-version.sh、tag 指向、HEAD 及工作区状态；推送后核对远端分支、tag 和 Release 工作流。
- 上线注意：新增上游数据库迁移及兼容边界见 upstream-sync.md；真实数据库和第三方上游集成未在本轮运行。

本记录为推送前状态；最终发布提交、远端 refs 及 Release 状态以后续核验为准。
