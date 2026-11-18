# 2026-11-18

## 17:37 CST 福利余额列、游戏兑换比例与积分兑换码

- 接续 Antigravity 任务 a7e20aa4-4687-467f-8dac-7321a2387800 的 9 个未提交文件，在 feature/welfare-column-game-center-fix 完成三项功能。
- 后台用户余额分列、福利余额服务端排序；游戏两页遵循 overview 可用福利、后台比例和最小值；新增 game_points 兑换码，核销、积分与流水共用事务，防重复和整数溢出。
- 关键代码：backend/internal/service/redeem_service.go、repository/redeem_code_repo.go、service/game_center_service.go、repository/user_repo.go；frontend/src/views/admin/{RedeemView,UsersView}.vue、user/{RedeemView,GameCenterView,SizeBetGameView}.vue。完整影响面及命令见 welfare-game-points-plan.md。
- 验证：93 项 Go 定向、39 项 PostgreSQL/Redis 集成、47 项前端回归；相关 ESLint/typecheck/双语/前后端构建通过。普通用户真实页面验证 250 积分到账、历史、流水、重复拒绝及双向比例、下限和舍入。
- 风险与限制：无 DDL/生产操作；管理员实页受首次协议弹窗限制，未代用户接受协议，已有组件和后端验证。保留旧码语义，新码回退前需处理未兑换码。额外仅修复两处原有测试编译错误。
- 未执行 commit/merge/push/部署；本地测试容器、服务及临时凭据在收尾清理，截图留在工作区外的 visualizations 目录。

## 17:56 CST 授权后补齐后台实页验证

- 用户授权仅在临时测试实例确认《Sub2API 部署与运营合规承诺》；重建隔离环境并执行确认，未访问生产。
- 真实后台生成 250 游戏中心积分码及类型筛选通过；真实用户列表充值/福利分列、福利升降序及刷新后排序记忆通过。
- 该后台生成码由普通用户在系统兑换成功使用，游戏中心显示 250 及对应 +250 流水。只读 SQL 确认码已核销、points=250，原充值和福利不变。
- 本轮 embed 构建通过，无新增业务修改，不重复扩大自动化回归。原有“后台实页待确认”限制已解除，当前无功能验证待办；详细记录及截图见 welfare-game-points-plan.md。

## 18:00 CST 合并到 custom-main

- 用户明确要求“合并分支”。已将已验证功能提交 `2ae5e1707` 从 `feature/welfare-column-game-center-fix` 以 `git merge --ff-only` 合入本地 `custom-main`，无冲突。
- 当前未执行 push、tag 或部署；合并后工作区保持干净。
