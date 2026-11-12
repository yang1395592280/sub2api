# 会话主题
- 项目：sub2api
- 当前阶段：v0.2.21合并后两处TTL缺陷修复已合入custom-main，按用户指示发布
- 交付安排：用户授权v0.2.21 tag、提交推送；发布结果以远端custom-main和tag、Release核验为准
- 上次会话：session_memory_20261112-0849.jsonl；详情postmerge-v021-bugfix-plan.md
- 交接状态：单元/竞态/构建通过，发布构建版本正确；Docker集成跳过；RELEASE_ONLY=true，不部署

## 最新主题：签到与福利分账
- 项目：sub2api，feature/welfare-balance-checkin。
- 当前阶段：开发与验证完成，未提交/合并/推送/部署。
- 活跃任务：无开发待办；上线迁移与活动开启由部署流程执行。
- 上次会话：session_memory_20261112-0849.jsonl 最新条目；docs/welfare-balance-checkin.md。
- 交接状态：全部主线已验证，细节 welfare-balance-checkin-handoff.md；自动风控需验证码及多信号证据。

## 最新授权：合并签到与福利分账
- 用户授权本地提交并合入 custom-main；合并结果及提交点以 Git 历史为准。已有验证结果继续适用，未授权 push 或部署。

## 最新授权：发布 v0.2.21.1
- 签到与福利分账已提交并合入 custom-main（90cd5c71e，无冲突）。用户进一步明确授权提交版本、打 tag 并推送个人 fork。
- 版本文件为0.2.21.1；发布工具11项、embed构建和二进制版本核验通过。仅推送custom-main与新tag；自动发布实际状态以GitHub核验为准，不部署。
- 交接文档：release-v0.2.21.1.md；上线迁移与回退见docs/welfare-balance-checkin.md。

## 最新授权：发布 v0.2.21.2
- 签到前端月历与翻倍动画已合入 custom-main（adb011cf8，无冲突）。用户授权提交版本、打 tag 并推送个人 fork。
- 版本文件为0.2.21.2；签到组件/i18n、typecheck、lint、build和mock浏览器验证通过。仅推送custom-main与新tag；自动发布实际状态以GitHub核验为准，不部署。
- 交接文档：release-v0.2.21.2.md；上线迁移与回退见docs/welfare-balance-checkin.md。
