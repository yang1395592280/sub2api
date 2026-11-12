# 签到与福利分账交付检查点

更新时间：2026-11-12 13:21（Asia/Shanghai）

## 已完成（带证据）

- feature/welfare-balance-checkin：充值/签到/礼遇/赠送分账、9=6+3、负福利排除、图片冻结原路释放、幂等流水、消费拆分与历史、签到及翻倍、风控观察/清退/恢复和前后台页面均完成。
- 福利允许负数，取消不足禁止翻倍；同来源奖励抵欠额，充值不追扣。旧余额原值保留；迁移 251、252 只新增字段表。
- 后端 go test -tags=unit ./... 退出 0；初轮已有内存阈值用例偶发超标，原断言单独复核及最终全量均通过；go build ./...、go vet ./... 退出 0。
- 真正 Docker PostgreSQL/Redis 集成 TestWalletIntegration / TestUsageBillingRepository 通过，非跳过，覆盖并发、幂等、负数恢复、礼遇负数、图片释放/对账、风控证据、纯福利不足及删除账号不阻塞清退。
- 定向 service/repository/handler race 通过；前端 429 文件 / 3409 测试通过；lint/typecheck/build 通过。
- Playwright 真实后端交互：配置、只赠福利、签到、翻倍 -0.5、充值/赠送不受影响、用户禁止管理员访问、手机无溢出；72h 观察到期、仅清签到、原因展示、暂停翻倍 403、管理员恢复均通过。
- 内嵌正式构建真实浏览器：中英文正常翻译、CSP 无 unsafe-eval、无页面/CSP 错误、桌面与 390px 手机截图通过。修复 vue-i18n 运行时编译版本配置。

## 相关文件

- docs/welfare-balance-checkin.md：业务规则、管理入口、限制、验证和迁移回退。
- docs/superpowers/plans/2026-11-12-welfare-balance-checkin.md：全部步骤完成（目录按原仓库约定忽略）。
- backend/internal/service/wallet.go、repository/wallet*.go、checkin*.go；migrations/251、252；前端余额/签到卡和管理签到页。
- 验证输出 /tmp/sub2api-welfare-*-final.log；浏览器证据 /tmp/sub2api-welfare-qa/。仅隔离测试环境；凭证不进入仓库。

## 未完成

- 无开发待办。用户已授权本地提交并合入 custom-main，实际提交及合并状态以 Git 历史为准；未 push、未部署。
- 实际上线需备份、执行迁移并同步前后端、旧缓存失效、配置验证码后再开启活动。不能直接回退旧版并保持活动运行，详细步骤见使用文档。

## 阻塞与限制

- 无授权阻塞。规则以 9=6+3 为准，消费顺序签到→礼遇→赠送；原站内计价为 USD。
- 自动风控依赖多信号与管理员证据复核；未配验证码不满足完整自动组合条件，不能保证识别所有仿真浏览器脚本。
- 本任务临时 QA 服务、Vite 与容器在交付前停止；截图和日志保留本地临时目录，生产环境未操作。
