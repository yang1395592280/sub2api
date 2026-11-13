# 2026-11-13

- 用户每日统计：独立管理员按日聚合签到／礼遇／两日消费分账／当前余额／全站名次；本地验证完成，未提交/合并/push/部署。详见 user-daily-stats-plan.md、session_memory_user-daily-stats.jsonl、docs/user-daily-statistics.md。历史余额为当前值，旧记录无分账不按当前策略猜测。

- 顶栏每日签到与上游每日消费：已完成本地实现及验收，feature/checkin-dialog-upstream-daily-spend 未提交/推送/部署。详见 checkin-upstream-daily-plan.md、session_memory_20261113-1023.jsonl 和根目录 docs/upstream-daily-usage-and-checkin.md。
- 后续用户授权“合并分支”：提交本次修改后 fast-forward 合入 custom-main，最终状态以 Git 为准；不推送或部署。
- 后续用户授权发布 `v0.2.21.4`：更新 VERSION、提交、annotated tag、atomic push；发布工作流按仓库现有 RELEASE_ONLY 配置运行。
- 使用记录详细扣款：feature/usage-wallet-precision 将充值／福利及来源明细恢复为8位；前端64项、ESLint/typecheck/build与本地模拟API浏览器验证通过。未推送或部署，详见 session_memory_usage-wallet-precision.jsonl。
- 后续用户授权“合并分支”：提交使用记录扣款精度修改后 fast-forward 合入 custom-main；实际结果以Git核验为准，不推送或部署。

- 后续用户授权“合并分支”：提交用户每日统计功能后 fast-forward 合入 custom-main；基线3ee90ee51，最终结果以Git核验为准，不推送或部署。
