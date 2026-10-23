# 工作日志 - 2026-10-23

## [15:18] 功能实现：福利活动按账号限制兑换次数
- 文件：迁移247、service/redeem_campaign.go、repository/redeem_campaign_repo.go、admin/redeem_campaign_handler.go、相关兑换流程/Ent/DTO/routes、RedeemCampaignDialog.vue、管理端/用户端RedeemView及API/双语/测试；使用与回退文档 docs/redeem-campaign-limits.md。
- 决策：同活动的余额码、抽奖券码及补发批次共享每账号额度，默认1次、可改1–1000次；独立计数不随码删除恢复，失败全事务回滚，旧码不关联活动。配置锁与条件UPSERT防并发超领。
- 验证：Go相关7包回归通过（工具报告7476项）；真实PostgreSQL18.1/Redis隔离环境5项活动测试及25项原repository子用例-race实际通过，无跳过；前端32项相关测试、ESLint、vue-tsc、前后端构建、git diff --check通过。浏览器真实组件配模拟API验证限额1→3、选择活动和生成关联码，截图已保留。
- 风险与状态：仅限站内账号，不能识别多账号同一人；新增结构需随新版本迁移。回退旧应用前先暂停或禁用未用活动码并保留领取记录。feature/redeem-campaign-limits未提交/合并/推送/部署，未操作生产数据；临时预览页与测试容器已清理。

## [15:23] Git操作：本地合并授权与提交前检查
- 用户明确要求“合并分支”，已授权提交本轮功能并合入本地custom-main；取代此前等待Git指令的状态。两分支起点均为9e5d786a4，custom-main未被其他worktree占用，全部43个改动文件均属于本任务，暂存diff检查通过。
- 方案：功能提交后以merge --ff-only合入custom-main，再验证分支包含关系、工作区干净；不推送、不打tag、不部署。沿用刚完成的相关回归和构建证据，本轮无业务代码追加；最终提交点与合并结果以实际Git核验为准。
