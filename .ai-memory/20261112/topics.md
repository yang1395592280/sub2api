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

## 最新主题：福利退款与金额展示
- feature/refund-welfare-display（基线abd49c9db）：三项主线本地开发验证完成；管理员选择福利来源、原子退款、金额两位、顶部及个人资料余额分栏。
- 回归与限制见refund-welfare-display-plan.md；正式构建API模拟浏览器、真实隔离数据库均验证。无生产账目核验，未提交/合并/push/tag/部署。

## 最新主题：福利净额计费安全与抵扣比例配置
- 分支feature/refund-welfare-display，保留前轮退款/显示修改；本轮本地开发完成，最新真实数据库集成正在运行。
- Go全仓unit/race/build/vet、前端434文件3448项/lint/build及正式构建mock浏览器已通过；详见wallet-security-policy-plan.md。未发布授权，不提交/合并/push/tag/部署，不改生产账目。

## 最新交付：福利安全与比例配置完成
- 4/4主线完成；Docker授权后15项真实PG/Redis集成实际全部PASS，其余Go全仓unit/相关race/build/vet、前端3448项/lint/build、正式构建模拟API浏览器已通过。
- 当前无开发验证待办，无生产数据调整。功能分支未提交/合并/push/tag/部署；详细证据见wallet-security-policy-plan.md与docs/welfare-balance-checkin.md。

## 最新授权：合并福利计费与退款修复
- 用户授权将 feature/refund-welfare-display 当前已验证改动提交并合入 custom-main；基线 abd49c9db 一致，计划 fast-forward，结果以 Git 历史为准。
- 不push、tag或部署；既有开发验证结果适用于本次合并。

## 最新授权：发布v0.2.21.3
- 用户授权提交版本、打tag并推送；功能aef9e009f已合入custom-main，版本文件0.2.21.3，发布工具11项/Linux、embed构建和二进制版本核验通过。
- 仅原子推送origin/custom-main与新tag，RELEASE_ONLY=true；沿用自动Release流程，不部署/不操作生产数据。最终refs和Release状态以GitHub为准，记录release-v0.2.21.3.md。
