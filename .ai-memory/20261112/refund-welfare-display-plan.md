# 余额展示与福利退款修复

基线：feature/refund-welfare-display，abd49c9db（v0.2.21.2）。本轮仅本地开发验证，不发布、不操作生产数据。

## 已确认口径
- 金额界面两位小数，计费、存储及导出保留精度；小于半分的负值显示0.00，实际余额不清零。
- 用户管理退款可单独退充值、单独退福利或同时退；管理员选择签到/礼遇/管理员赠送来源，只扣所选正余额，不能超退。
- 充值与福利扣减同事务、用户行锁、持久幂等及分账流水；不改变原有负福利及活动规则，不产生新负债。
- 顶部个人余额桌面/手机明确区分充值与福利，可用额度以服务端字段为准。

## 开发步骤
- [x] 统一金额展示，修复流水截断、礼遇长小数、顶部余额分栏。
- [x] 退款来源选择与API/handler/service/repository原子扣减，兼容受限管理员响应。
- [x] 回归金额、只退福利/充值/混合、来源隔离、余额不足、回滚、幂等、并发。
- [x] typecheck/lint/build、隔离数据库和实际浏览器验证，最终diff复核、文档记忆。

影响链路已通过rg实码确认：UserBalanceModal→admin/users.updateBalance→UserHandler.UpdateBalance→AdminWalletRefunder→wallet_admin→wallet_transactions→wallet_history。无需DDL、依赖升级、权限或生产配置变更。
风险与回退：扣款属于账务关键路径，必须同事务验证；新字段为可选增量且仅福利退款必需；旧版本不支持该请求，回退代码前停用福利退款。已发生流水保留，不删账。

## 验收与交接
- 已完成：金额显示、余额分栏、管理员选择退款来源三项主线；同事务/幂等/受限响应保持；额外修复历史弹窗叠加，个人资料概览也分栏。
- 证据：/tmp/sub2api-refund-full-frontend.log（430/3433）；final-frontend.log（3文件30项，含新增API5项/概览/弹窗切换）；full-go.log（三包完整unit）；integration.log（11项真实PG/Redis）；browser.log（正式构建模拟API、桌面/390px/中英、无pageerror）。上述日志均以前缀/tmp/sub2api-refund-命名。lint/typecheck/build、Go build/vet、最终定向race、git diff --check通过。
- 边界：没有DDL或生产操作，没有支付订单退款改动；截图中的-0.0414776显示为-0.04，真实负数不改。无法从截图确认多扣，指定用户真实历史账目未核验。
- 复盘：退款入口以截图和调用链为准，早期支付订单方向误判已全部撤销；浏览器QA补齐API数据形状及持久语言偏好，避免把测试桩缺陷当产品缺陷。
- 决策：用户选择管理员明确来源，舍弃自动消费顺序扣福利；保留可追溯来源和历史负债。若后续支持一次多来源退款，应扩展明确分项请求及幂等比较。
- 未完成：无本地代码待办；未提交/合并/push/tag/部署。当前feature/refund-welfare-display，基线abd49c9db。
- 阻塞：无。
