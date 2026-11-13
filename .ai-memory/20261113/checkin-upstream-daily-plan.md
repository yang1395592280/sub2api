# 顶栏签到与上游每日消费

- 分支：feature/checkin-dialog-upstream-daily-spend；基线 99d8fb838；初始工作区干净。
- 验收：顶栏每日签到打开现有完整日历；个人资料移除卡片；中奖/未中奖均有翻牌与独立视觉反馈；请求失败可重试且不伪造结果。
- 验收：刷新余额同时获取当前 API Key 当日实际扣费；按已维护账号对应上游分组/价格；保留上游扣费原值并按充值比例折算成本；缺失/失败/跨日缓存不冒充零。
- 来源：本仓库和 upstream/main 的 gateway_handler.go、usage_handler.go；QuantumNous/new-api 0bda086fb4c2f83ac0397e4a60d90686578d67ad 的 controller/log.go、model/log.go、controller/token.go。
- 方案：sub2api 优先读取 /v1/usage 的 daily_usage（Asia/Shanghai），必要时管理凭据 /usage/stats；New API /api/log/self 按日期分页，按 token_id 聚合，避免同名令牌误归属。100 页/10000 条和总超时限制，不交付部分合计。
- 影响面：AppHeader→新签到入口→BaseDialog→ProfileCheckinCard→签到 API；ProfileView；上游余额 service→账号 extra→OpenAIUpstreamBalanceCell→AccountsView；中英文文案与相关测试。沿用 Vue/TS 单引号、Go gofmt。
- 风险与缓解：上游版本差异明确状态；统计失败不阻断余额；凭据仅复用已有后端解析且不写日志；无 DDL、新依赖、生产操作、推送；变更可通过 Git 局部回退。
- 验证：签到成功/失败/重复点击/两个结果；上游空数据/分页/混合 token_id/重名/错误/限额/充值比例；定向 Vitest、Go unit、typecheck、lint、构建、模拟上游 API 的浏览器桌面与移动端操作截图。
- 门禁：A1-A10 已核对；B1/B2/B8 不适用（无迁移/依赖/不可逆）；B3 重复提交沿用后端幂等与前端禁用；B4 非网关热路径、限制读取规模；B5 保持原接口，仅扩展 extra；B6 定向测试；B7 非批量更新；B9 已 rg 复核调用链、图谱不可用；B10/B11 本地命令与回读；C1/C2 每阶段记录。
- 环境：Python/Go/pnpm 可用、frontend/node_modules 已存在；技能初始化 dry-run 通过，仅采用现有工具，不引入其他 IDE hooks。
- 状态：两项本地开发与验收完成；未提交、合并、推送或部署。

## 实现与决策（2026-11-13）

- 顶栏按钮位于加入群聊左侧；懒加载完整签到卡片，个人资料移除旧入口。洗牌期间两面均为问号，返回服务端结果后翻牌；中奖绿色光环和彩屑，未中奖紫色涟漪。无客户端随机结果；支持减少动画、失败重试与重复点击保护。
- 原签到规则保持：失败扣 1.5R，今日净收益 −0.5R。卡面与金额明确显示负值；未修改钱包、概率或活动规则。
- 余额刷新同时缓存当前 Key 的北京当日扣费、充值比例折算成本、上游用户总额；多条本地渠道共用 Key 或上游用户时不能重复求和。按 token_id 归属，避免同名/改名误匹配；New API 优先使用 quota_per_unit。
- 附带最小修复 AccountsView 的 immediate watch 早于 accounts 初始化问题；先用开启质量监控的测试复现 ReferenceError，再移动 watch 至 useTableLoader 之后，保留原逻辑。
- 拒绝使用余额差值（含充值/赠送）、标准价 cost（不是实际扣费）、New API token_name 汇总（重名与改名错配）、最近日志（可能截断）；仅在上游提供按 Token ID 聚合接口时撤销完整分页方案。
- 新增说明文档 docs/upstream-daily-usage-and-checkin.md，并为该文档增加既有 docs/* 忽略规则的单文件例外。

## 实际验证

- Go：service short 测试 7759 项通过；最终 provider/每日消费定向 48 项通过、race 通过；go build ./... 与 go vet ./internal/service 通过。
- 前端：最终相关 Vitest 38 项通过；最后问号卡面调整后签到 6 项再次通过。修改文件 ESLint 通过；pnpm run build（含 i18n、vue-tsc -b）通过，最终构建日志 /tmp/sub2api-checkin-daily-final-build.log。保留既有大 chunk 提示。
- 浏览器：正式构建 + 本地模拟 API；顶栏打开、签到成功、日历更新、中奖 ×2、未中奖 −½ 与负金额、个人资料入口移除、刷新余额消费/分组/价格同列、390px 弹窗滚动和无横向溢出通过。最终构建账号页面和顶栏可加载。
- 浏览器验收时重构建引起临时服务器缓存旧静态资源；仅修正临时验收脚本为 no-store 并重载验证，未修改产品缓存策略。临时鹈鹕接口 fixture 的列表结构也已修正。
- 没有真实站点凭据、验证码或定制版本联调；本次不含历史账本/历史回填。超时、分页不完整与超过 10000 条时不交付部分合计。
- 本地截图保存在本会话 visualizations 的 checkin-daily-usage 目录；验收浏览器已关闭、viewport 已恢复；临时服务已停止。

## 后续授权：本地分支合并

- 用户明确要求“合并分支”，授权提交当前已验证修改并合入 custom-main。
- 两分支当前同基线99d8fb838、无分叉，使用 fast-forward；原验证覆盖当前代码，合并前 diff 检查通过。最终提交点及合并结果以 Git 历史为准。
- 此授权不包含 push、tag、发布或部署。
