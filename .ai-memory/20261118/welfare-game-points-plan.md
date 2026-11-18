# 福利余额与游戏中心积分兑换码

日期：2026-11-18；分支：feature/welfare-column-game-center-fix。

## 范围与验收

1. 接续原任务的 9 个未提交文件，用户列表充值余额、福利余额分列，福利余额支持服务端排序。
2. 游戏中心和猜大小页面以 overview 的可用福利余额为准；显示、输入下限和兑换预估遵循后台配置及后端四舍五入规则。
3. 管理员可生成 game_points 类型兑换码（正整数积分），用户在现有系统兑换入口兑换，积分和流水与核销原子提交，重复兑换不重复入账。

## 已确认实现入口与影响面

- domain/constants.go → service/domain_constants.go → admin GenerateRedeemCodes/CreateCode/GenerateCodes。
- admin/redeem_handler.go 允许新类型；redeem_service.go 沿用 Use 乐观锁及 ent 事务。
- repository/redeem_code_repo.go 使用同一事务写 users.points 与 game_points_ledger。
- frontend types → 管理员 RedeemView 生成/筛选/展示 → 用户 RedeemView 成功提示/历史/跳转。
- 原改动 users 排序/UsersView 与 game_center_service/repo/GameCenterView/SizeBetGameView。

## 风险与兼容

- 无 DDL、生产数据库写入、权限调整、新依赖；使用已有 decimal(20,8) value、BIGINT points 和积分流水表。
- 积分值上限 999999999999（value 字段整数位上限）；拒绝零、负数、小数、非有限数，累计余额检查 BIGINT 溢出。
- Use、积分及流水共享事务，用户行锁串行化同一用户不同码；失败回滚，沿用现有限流和过期校验。
- 不为积分兑换码触发充值返利，不混入充值余额。现有福利活动限领范围维持余额码/抽奖券码。
- 仅扩展类型与响应字段，保持既有兑换数据字段及旧类型语义；原工作区修改保留。
- 未授权 commit/merge/push/部署，不执行。

## 验证路径

- Go 定向 service/handler/repository 回归，新增积分值、事务回滚、并发及比例计算覆盖。
- PostgreSQL 隔离集成：真实兑换入口、重复/并发、流水失败回滚、福利余额总额与排序。
- 前端定向 Vitest、双语完整性、ESLint、typecheck、生产构建。
- 启动本地页面，使用隔离测试数据验证生成、系统兑换、积分展示和比例预估，记录实际结果。

## 门禁核对

- A1–A10：需求对应步骤完整；真实入口已检索；原 9 文件已核对；只改相关代码；错误回到对应实现修复。
- B1：无 DDL，测试数据仅隔离库；B2：无新依赖；B3：同事务及用户行锁；B4：沿用列表分页，排序需表达式；B5：响应只增字段；B6：见验证路径；B7/B8：无生产批写/不可逆操作；B9：grep 复核依赖；B10/B11：沿用现有工具并回读 diff。
- C1/C2：实现、验证两个阶段追加交接记录；Git 保留版本基线，原工作区无覆盖。

## 最终验证（2026-11-18 17:37 CST）

- Go：`go test -tags=unit ./internal/service ./internal/repository ./internal/handler ./internal/handler/admin -run 'Test(GameCenterExchange|GamePointsCodes|Redeem|Generate.*Redeem|CreateAndRedeem|GrantRedeem|BuildGamePointsWhere|UserListOrder|BatchImageCleanup)' -count=1`，93 项通过。
- PostgreSQL/Redis：`CI=true go test -tags=integration ./internal/repository -run 'Test(GamePointsRedeem|GameWelfareAssets|RedeemCodeRepoSuite|RedeemCampaign|RedeemReduction|RedeemConcurrentReductions)' -count=1`，39 项通过，使用测试容器，含真实兑换 HTTP、同码/不同码并发、失败全回滚、过期/溢出、福利来源与排序及既有活动/订阅回归。
- 并发反例曾发现 FK KEY SHARE→FOR UPDATE 锁升级死锁；改用 FOR NO KEY UPDATE 后，同用户不同码并发全部到账且不丢积分。同码 8 次并发仅成功一次；注入流水已写后的错误时，核销、积分和流水全部回滚，码可重试。
- Vitest：管理员兑换码、用户兑换、游戏中心、用户列表和双语完整性合计 47 项通过；最后用户列表定向复验 17 项通过，包含列分离、服务端福利排序和保存排序恢复。
- 已通过相关修改文件 ESLint、`pnpm typecheck`、`pnpm build`、`go build ./cmd/server` 和 `go build -tags embed ./cmd/server`。Vite 保留原有 Browserslist 数据过期和大分块提示，无构建失败。
- 实页：启动独立本地后端、Vite、全新 PostgreSQL/Redis，普通用户系统兑换 QA 码成功增加 250 积分，历史与游戏中心流水均可见；再次兑换拒绝。数据库核验充值 7、签到 0.3456、礼遇 1、赠送 1 保持不变，仅 points 从 0 到 250，流水 1 条。
- 实页：游戏中心与猜大小均显示福利 2.35，比例 37 积分/元；游戏中心反向显示 40 积分/元，20 积分预估 0.50。0.1 元按后端规则预估 4 积分；低于后台下限时禁提交。
- 管理员后台首次登录触发协议确认，未擅自接受，已向用户发起授权请求；后台实页生成与用户列表交互未完成。相关入口已有组件、服务及数据库回归覆盖。UI 测试码与数据为隔离库夹具，未将夹具创建冒充后台实页生成成功。
- 截图：`/Users/jaydenyang/.codex/visualizations/2026/11/18/01a1edd7-0fde-7263-bb97-08baa56cb870/game-points-exchange-qa.jpg`、同目录 `size-bet-exchange-qa.jpg`。

## 相关编译问题

- 既有 usage_log_repo_integration_test.go 多一个闭合括号，移除该行以恢复集成测试编译。
- 既有 batch_image_cleanup_test.go 的 mustJSON 与业务文件同名，仅重命名为 mustBatchImageCleanupJSON；未改其断言或业务逻辑。

## 状态

- 三项功能实现及对应自动化验证完成；原 9 个未提交文件已接续保留。
- 无数据库迁移，无生产写入。分支仍为 feature/welfare-column-game-center-fix；未 commit/merge/push/部署。
- 管理员实页验证限制已于 17:56 CST 解除；用户授权仅在临时测试实例确认协议后，补齐后台生成、筛选、用户余额分列及排序和真实生成码兑换流程，见下节。
- 仅撤回本次相关补丁即可回退代码；如已发放新类型码，回退前应先停发并处理未用积分码，旧版本无法兑换该类型。已兑换的积分与流水需保留审计。

## 授权后补验（2026-11-18 17:56 CST）

- 用户明确授权仅在临时测试实例点击协议“确认并继续”。新建独立 PostgreSQL/Redis 及临时管理员，`go build -tags embed -o /tmp/sub2api-points-followup-server ./cmd/server` 成功，直接运行构建产物进行浏览器验证；没有访问生产实例。
- CUA 在 `http://127.0.0.1:18080` 登录临时管理员，填写确认短语并完成协议确认；操作范围仅该隔离实例。
- 后台兑换码页：选择“游戏中心积分兑换码”，填写 250，点击生成，页面显示生成成功；列表类型及面值为积分，类型筛选正常。
- 用户管理页：两名普通测试用户的充值/福利分别为 20/0.50 和 7/2.35，两列独立展示。点击福利表头，升序为 0→0.50→2.35，再点降序为 2.35→0.50→0；重新加载仍保持降序。
- 完整实页路径：后台生成的真实测试码 dc52d59486a54d14e3a203851703f6bb，由普通用户 qa-high@example.com 在系统兑换入口成功使用；成功提示、兑换历史、游戏中心 250 积分及一条 +250 流水均可见。
- 数据库只读核验：码为 game_points/250/used/used_by=3；用户 balance=7、gift_balance=2.3456 未变，points=250；流水 redeem_code，delta=250，before=0，after=250。
- 无新增业务代码修改，复用前轮 93/39/47 项自动化证据。本轮补齐先前受协议限制的管理员页面验证，不存在该项待办。
- 截图目录同前：`admin-game-points-code-qa.jpg`、`admin-welfare-sort-qa.jpg`、`generated-code-user-redemption-qa.jpg`。
