# v0.2.22.1 发布记录

- 授权：用户明确要求打 tag `v0.2.22.1`、提交并推送。
- 起始状态：`custom-main=ed900c8f2`，已完成上游 `v0.2.22`（提交点 `15871f904`）合并，冲突已全部解决并通过前后端编译与测试。
- 版本：`backend/cmd/server/VERSION` 从 `0.2.22` 更新为 `0.2.22.1`。
- 内容：合入原作者上游 v0.2.22 迭代（含 186 个上游提交，包括 force_openai_ultrafast、Bedrock 速率限制、SiliconFlow Qwen3 补齐等），完整保留用户现有自建账号池、价格分组、额度管理等二改能力。
- 本轮验证：
  - `backend/scripts/resolve-version.sh` 输出 `0.2.22.1`。
  - `make build-backend` 成功构建，二进制运行 `-version` 输出 `Sub2API 0.2.22.1`。
  - `internal/web`、`internal/repository`、`internal/service` 核心单测全部通过。
  - 前端 `pnpm typecheck`（`vue-tsc --noEmit`）、`pnpm lint:check` 及 32 组核心 Vitest 测试套件（633 项）全部通过。
- 推送范围：发布提交后创建 annotated tag `v0.2.22.1`，使用 `git push --atomic` 推送 `origin/custom-main` 与该 tag，不 force。
