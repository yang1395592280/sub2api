## 上游同步：v0.2.15 合入 custom-main

- main 从 3f1a2ea0a 快进到 3a6fd1c9d；以 merge 整合到原 custom-main=d19cc71c6。
- 手工处理 13 个冲突，保留用户二改语义；额外补充合规状态竞态回归并修正嵌入测试的过期资源路径。
- 前端 396 文件/3014 测试、lint、typecheck、build；后端 go test、定向 unit、go vet、普通及嵌入构建均通过。
- 未 push/部署/执行数据库迁移；部署注意 Go 1.27.2 和上游迁移 242。
- 完整文件清单、验证命令、限制与撤销条件见 upstream-sync.md。
