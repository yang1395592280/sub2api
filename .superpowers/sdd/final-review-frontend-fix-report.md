# Final Review Frontend Fix Report

日期：2026-07-02

## 结论

已修复前端阻塞问题：当 `editUpstreamAdminType === 'sub2api'` 且启用自动签到时，保存前现在会校验开始/结束时间必填、`HH:mm` 格式，以及结束时间必须晚于开始时间。校验失败会阻止提交，并通过现有 `appStore.showError` 提示。关闭自动签到时不会强制时间校验。

## 修改文件

- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/components/account/__tests__/EditAccountModal.spec.ts`

## 实现说明

1. 在 `EditAccountModal.vue` 中新增 `validateUpstreamCheckinSchedule`：
   - 启用自动签到时要求 `start_time`、`end_time` 非空。
   - 使用 `HH:mm` 正则校验时间格式。
   - 将时间转换为分钟数，校验 `end_time > start_time`。
2. 在现有签到 URL 校验前执行上述校验，失败立即 `showError` 并 `return`，因此不会调用账户更新接口。
3. 保存时将签到开始/结束时间做 `trim()` 后写回 credentials。

## 测试补充

新增并通过以下前端测试：

- 启用签到但开始/结束时间为空时，不调用 `update`，并展示错误。
- 启用签到但 `end <= start` 时，不调用 `update`，并展示错误。
- 关闭签到时，不强制时间校验，仍允许保存。
- 保留原有“签到配置保存”和“手动测试签到”用例通过。

## 验证

执行命令：

```bash
cd frontend && pnpm exec vitest run src/components/account/__tests__/EditAccountModal.spec.ts
```

结果：`30` 个测试全部通过。

## 风险与关注点

- 当前新增了保存时的 `HH:mm` 格式校验；浏览器原生 `type="time"` 通常已限制输入格式，但测试和运行时仍保留了前端兜底校验。
- 本次未改动后端接口与调度逻辑。
