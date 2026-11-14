# 题库与多选生成测试

## 使用

- 管理菜单「题库与提示词」分别维护智力题、鹈鹕提示词；支持新增、编辑、启用/停用。智力题标准答案为 1–12 位非负整数，比较前规范前导零；提示词应要求只返回最终数字、不联网。
- 账号的测试/重新测试、批量鹈鹕测试和批量智力题测试都通过题库多选。每个账号与每个选中题目生成一条独立记录，每条记录对应一次上游调用。
- 首次默认选择第一道启用题；以后恢复当前管理员、当前类型上次成功入队的选择。取消、请求失败或零条入队不会替换偏好；失效选项会提示用户处理，不静默换题。
- 手动单批最多 100 个账号、20 道题，总计 1,000 条记录；每类新建题库最多 500 项。同账号同类型同题已有排队/运行记录时，仅跳过该组合。界面显示真实入队与跳过数量。
- 分组的两类自动测试独立保存启用状态、模型、选题及执行间隔（整数 5–1440 分钟，默认 30）。启用或修改选题/模型时创建立即到期的时间槽，后台每分钟扫描；只更改间隔时，下一次时间按最近入队时间重新计算。修改运行中的任务不会重新调用上游。
- 账号表显示最近批次汇总，点击可查看全部题目。智力题历史按批次列出各题答案、标准答案与用量；鹈鹕历史按批次展示作品卡。分页以批次为单位。
- 每条历史保存题名、提示词及标准答案快照。编辑和停用题目不会修改历史；定时任务执行前会复核分组成员、选题及启用状态。标准答案不会额外发给模型。
- 沿用全实例最多 3 路生成并发、单次生成 500 秒超时与既有队列清理规则（运行超 10 分钟、排队超 6 小时标记失败）。大量慢任务应分批执行。测试不改变调度评分。

## API 与兼容

题库维护和新的多选入口要求完整管理员，受限管理密钥不能维护题库或使用多选接口。选题偏好身份来自当前认证主体，客户端不能指定其他管理员。

- `GET /api/v1/admin/test-questions?test_kind=intelligence|pelican`
- `POST /api/v1/admin/test-questions`、`PUT /api/v1/admin/test-questions/:id`：`test_kind,name,prompt,expected_answer,enabled`；已有题目类型不可修改。
- `GET /api/v1/admin/test-questions/preferences?test_kind=...` 返回 `question_ids`。
- 现有账号测试接口新增 `question_ids` 数组。省略该字段时继续兼容旧单 prompt 请求；空数组是无效多选，不能用它发起任务。
- latest 接口 `include_batch:true` 返回每个账号最近批次的所有记录，`batch_id` 可精确查询发起批次，避免后续批次覆盖进度。
- 历史接口 `group_batches=1` 使用批次分页，`total` 为批次数、`total_records` 为独立记录数；旧请求保持按记录分页。
- 分组配置新增 `question_ids`、`interval_minutes`；旧请求省略间隔仍为 30 分钟，省略选题仍兼容 prompt 路径。

## 升级与回退

新增迁移：`backend/migrations/254_generation_test_questions.sql`，由现有 runner 事务执行。会建立题库及管理员偏好表、回填历史快照、迁移分组自定义提示词并更换活动任务唯一索引。历史内容保留，原默认题自动建立。

**升级必须停止全部旧应用实例并备份数据库，再运行新版本。不能将旧、新二进制混跑**：活动任务唯一键由 `(account_id,test_kind)` 改为 `(account_id,test_kind,question_id)`，旧 SQL 的冲突目标与新索引不匹配。大表回填及索引创建会持有表锁，应在维护窗口执行。先确认迁移成功，再启用自动测试。

回退到旧版本时，先关闭两类定时测试，等待队列排空，再停止全部应用。备份后执行 `docs/operations/generation-tests-rollback.sql`；脚本会检查活动任务是否仍有同账号同类型多题，发现冲突立即拒绝回退。脚本关闭自动测试、恢复旧唯一索引及 30 分钟约束，保留全部新题库、偏好及历史列。随后启动旧版本，按需重新配置自动测试。

手动回退后迁移记录仍存在；如再次升级本功能，必须在停机状态恢复以下约束及索引，不能直接重复执行完整 254 迁移：

```sql
BEGIN;
ALTER TABLE group_pelican_test_schedules
  DROP CONSTRAINT group_pelican_test_schedules_interval_minutes_check,
  ADD CONSTRAINT group_pelican_test_schedules_interval_minutes_check CHECK(interval_minutes BETWEEN 5 AND 1440);
DROP INDEX idx_account_pelican_tests_active;
CREATE UNIQUE INDEX idx_account_pelican_tests_active
  ON account_pelican_tests(account_id,test_kind,question_id) WHERE status IN ('queued','running');
COMMIT;
```

## 验证

- 后端单元测试：`cd backend && go test -tags=unit ./internal/service ./internal/repository ./internal/handler/admin -run 'Test(Pelican|Intelligence|GenerationQuestion)' -count=1`
- 真实 PostgreSQL/Redis：`cd backend && go test -tags=integration ./internal/repository -run 'Test(Pelican|GenerationQuestion)' -count=1`（需要 Docker；确认未被 harness 跳过）。包含从旧表结构和历史自定义数据升级的验证。
- 前端：题库页、多选器、批量/定时弹窗、批次进度、两类历史页和原详情弹窗的针对性 Vitest；`pnpm build` 包含 i18n 完整性及 Vue 类型检查。
- 浏览器：正式构建配本地模拟 API，验证新增/停用、两类批量 2 账号 × 2 题、取消后恢复、成功后记忆、空选择和非法间隔阻止提交、45/60 分钟定时配置保存恢复、整批结果及逐题标准答案/作品卡。此验证没有调用真实付费模型。页面辅助的版本/用量接口未完整模拟，其错误不作为生产回归结论。
