# v0.2.21.7 发布记录

- 授权：用户明确要求打 tag `v0.2.21.7`、提交并推送。
- 起始状态：`custom-main=ad2c43170`，工作区干净；`origin/custom-main=72aa09ff6`；本地及远端均无 `v0.2.21.7` 标签。题库功能已无冲突快进合入 `custom-main`。
- 版本：`backend/cmd/server/VERSION` 从 `0.2.21.6` 更新为 `0.2.21.7`。
- 内容：智力题库与鹈鹕提示词维护；账号单次及批量多选；按管理员和测试类型记住成功入队的选择；分组两类定时测试独立多选并配置 5–1440 分钟间隔；逐题去重、答案快照、批次汇总与作品卡。
- 已有验证：复用功能提交 `ad2c43170` 的 Go 相关四包 59 项 unit、14 个不同 PostgreSQL18.1/Redis8.4 集成用例、前端相关 30 项及最终 18 项组件复验、ESLint/i18n/vue-tsc/build/Go build，以及正式前端配本地模拟 API 的实际交互验收。本轮没有业务代码变化。
- 本轮验证：`sh -n backend/scripts/resolve-version.sh`、`backend/scripts/resolve-version.sh`、`CGO_ENABLED=0 go -C backend build -tags embed -trimpath -o /tmp/sub2api-v0.2.21.7-server ./cmd/server` 及二进制 `-version` 通过，版本均为 `0.2.21.7`。使用现有临时 venv 执行 `test_plan_accepts_existing_four_part_release_tags` 与 `test_plan_requires_a_tag_for_publication`，2 项通过。提交前执行 `git diff --check`。
- 推送范围：发布提交后创建 annotated tag `v0.2.21.7`，使用 `git push --atomic` 推送 `origin/custom-main` 与该 tag，不 force；最终分支、tag peeled commit 及工作流状态以推送后的 Git/GitHub 核验为准。本记录随发布提交保存，属于推送前检查点。
- 工作流：本轮已通过 `gh variable get RELEASE_ONLY --repo yang1395592280/sub2api` 核实 `RELEASE_ONLY=true`；tag 推送触发既有 Release 流程，仅生成归档、checksums 与 GitHub Release，跳过 Docker/GHCR 发布。
- 升级要求：迁移 254 将活动唯一键从 `(account_id,test_kind)` 改为 `(account_id,test_kind,question_id)`。必须备份数据库并停止全部旧应用实例后升级，不能混跑新旧二进制；历史回填及索引创建应安排维护窗口。回退步骤见 `docs/test-question-library.md` 与 `docs/operations/generation-tests-rollback.sql`。
- 边界：未执行生产迁移、部署或真实付费模型调用；全局 3 路并发、500 秒执行超时及原有队列清理规则保留。
