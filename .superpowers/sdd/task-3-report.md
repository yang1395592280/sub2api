## Task 3 Report

### 变更结论

已完成 Task 3 所需前端改动：

- 账号编辑弹窗新增 `sub2api` 自动签到配置 UI。
- 新增前端 `testUpstreamCheckin` API 包装。
- 为 `Account.extra` 补充签到状态类型。
- 为编辑弹窗补充签到配置读写与手动测试的回归测试。

### 实际修改文件

- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/api/admin/accounts.ts`
- `frontend/src/types/index.ts`
- `frontend/src/components/account/__tests__/EditAccountModal.spec.ts`

### 实现内容

1. `EditAccountModal.vue`
   - 仅在 `upstream_admin_type === 'sub2api'` 时显示自动签到配置。
   - 新增字段：
     - `upstream_checkin_enabled`
     - `upstream_checkin_url`
     - `upstream_checkin_start_time`
     - `upstream_checkin_end_time`
   - 以上字段从 `credentials` 读取并写回 `credentials`。
   - 新增“测试签到”按钮，调用 `adminAPI.accounts.testUpstreamCheckin(account.id)`。
   - 从 `extra` 读取并展示最新签到状态快照：
     - `upstream_checkin_status`
     - `upstream_checkin_last_run_at`
     - `upstream_checkin_last_success_date`
     - `upstream_checkin_next_run_at`
     - `upstream_checkin_reward_amount`
     - `upstream_checkin_balance`
     - `upstream_checkin_error`
   - 敏感字段继续沿用“留空保留”逻辑，未覆盖既有 `sub2api` 密码 / token 保留行为。
   - 签到 URL 做前端约束：仅允许相对路径，或与 Base URL 同源的完整 URL。

2. `frontend/src/api/admin/accounts.ts`
   - 新增：

```ts
export async function testUpstreamCheckin(id: number): Promise<Account> {
  const { data } = await apiClient.post<Account>(`/admin/accounts/${id}/upstream-checkin/test`)
  return data
}
```

   - 并挂到 `accountsAPI` 导出对象。

3. `frontend/src/types/index.ts`
   - 新增 `UpstreamCheckinStatusSnapshot`。
   - 将签到状态字段并入 `Account.extra` 类型。

4. `EditAccountModal.spec.ts`
   - 新增“显示并保存 sub2api 签到配置”测试。
   - 新增“手动测试签到并刷新状态快照”测试。

### TDD 过程

1. 先补测试：
   - `shows sub2api check-in controls, loads status, and saves check-in config`
   - `tests sub2api check-in manually and refreshes latest status snapshot`

2. 首轮按 brief 命令执行：

```bash
cd frontend && pnpm test -- EditAccountModal
```

结果：
- 新增签到测试先失败，符合 RED 预期。

3. 实现后重新验证：
   - 精确文件测试通过。

### 验证命令

1. RED

```bash
cd frontend && pnpm test -- EditAccountModal
```

结果：
- 新增签到用例失败，证明测试先行生效。

2. GREEN（针对本任务文件精确验证）

```bash
cd frontend && pnpm exec vitest run src/components/account/__tests__/EditAccountModal.spec.ts
```

结果：
- `27 passed`

### 风险与注意事项

- `pnpm test -- EditAccountModal` 在当前仓库会额外匹配到 `BulkEditAccountModal.spec.ts`，其现存失败为：
  - `BulkEditAccountModal > antigravity 映射预设包含图片映射并过滤 OpenAI 预设`
- 该失败与本次 Task 3 改动无关，本次未触碰对应文件，未擅自修复。
- 当前第一版仅支持 `sub2api` 上游管理凭据，符合任务约束。
