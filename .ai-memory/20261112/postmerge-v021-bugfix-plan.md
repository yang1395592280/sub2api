# v0.2.21 合并后缺陷排查

目标：复核合并冲突与完整回归，修复可复现问题并保留回归测试；不推送、不部署、不操作业务数据库。
基线：custom-main 519ee380e；修复分支 feature/postmerge-v021-bugfix；开始工作区干净。

## 计划与门禁
- [x] 读取项目约定、相关代码、上次验证与既有记忆。
- [x] 运行前后端完整单测，区分测试依赖缺失与生产缺陷。
- [x] 翻译根因：锁文件与旧node_modules不一致，按锁文件安装后原配置通过；保留真实语言包回归。
- [x] 账号页测试：补齐新依赖并保留原行为断言，不修改生产业务语义。
- [x] 修复后完整前端回归、构建、实际浏览器翻译及 CSP 检查；后端最终全量通过。
- [x] 审查 diff、记录验证和剩余限制。

影响范围：Vite/Vitest 配置、i18n 回归测试和账号页单测。依赖图仅辅助，引用点用 rg 复核。
风险：切换带编译器的 vue-i18n 构建需要显式开启 v9 JIT，验证 CSP 不依赖 unsafe-eval。
数据库、API、权限、生产配置、依赖升级均无计划变更（N/A）；若发现需另行确认。
验收：真实 en/zh 文案、插值与复数可用；完整前端无新失败；后端限制如实报告。

## 初始证据
- 前端 427 文件 / 3404 测试：60 失败（7 组账号页 44 项、真实翻译 16 项），其余 3344 通过。
- 账号页失败为 getActivePinia 缺失；翻译测试输出 message format compilation is not supported。
- 已确认 vue-i18n 9.14.5，运行时 alias 无消息编译器；应用以原始字符串语言包加载。

## 交接
- 已完成：上述基线；日志 /tmp/sub2api-postmerge-frontend.log 与 backend-unit.log。
- 相关文件：frontend/vite.config.ts、vitest.config.ts、src/i18n/index.ts、AccountsView.vue。
- 未完成：失败回归、最小修复与验证。
- 阻塞项：数据库集成需 Docker，本次不操作生产。

## 排查修正与第二轮
- 翻译根因已证伪为仓库缺陷：package/lock 为 v11.4.16，本地 node_modules 为 v9.14.5。pnpm install --frozen-lockfile 后原配置测试通过，已撤回 Vite/Vitest 配置改动。保留真实语言包回归。
- 7 组账号页 44 失败已通过补齐 adminSettings/Pelican mock 解决，8文件53测试通过，未改生产业务。
- 初始后端 go test -tags=unit ./... 退出0，上一轮内存阈值未再失败。
- 新确认 P1：WS passthrough Relay Usage 缺少 30m/12h字段，解析、逐轮合并、累计及 adapter 转发均丢失，导致 12h缓存按普通缓存价计费。真实 WebSocket 入口 red: expected short=2, actual=0。
- 修复方案：复用既有 HTTP TTL 校验提取到 internal/pkg/openai/usage.go，让 HTTP 与WS共用；补足Relay与adapter字段，保持未知/非法TTL回退规则。
- 验证：真实多轮入口（mixed/short/unknown/invalid）、共享parser边界、progressive与累计、现有HTTP TTL测试；补全量后端和race。无DDL、权限、接口结构变更或生产操作。
- 依赖影响面 rg：Relay→adapter→OpenAIForwardResult→RecordUsage→UsageTokens/UsageLog；无需更改计费公式。图谱无Go/Vue符号，使用rg实码核对。
- 全量前端复测：3405通过/1失败，唯一失败为新增fixture键被静态key检查误当生产key；已改为变量引用，未修改检查器。
- 前端lint与正式构建已通过（现有chunk大警告）。

## 当前交接
- 已完成：WS真实入口与HTTP TTL定向green；后端全量基线通过；账号页与语言包定向通过。
- 相关文件：backend/internal/pkg/openai/usage.go、service/openai_ws_v2/passthrough_relay.go、openai_ws_v2_passthrough_adapter.go、对应新回归。
- 未完成：WS包全量、race与后端全量复测、正式构建浏览器翻译/CSP检查、清理图谱产物、交付。
- 阻塞项：无本地主线阻塞，数据库集成仍受Docker环境限制。

## 已验证进展
- 第二条生产缺陷：ResponsesUsage typed JSON在Chat/Messages终止事件覆盖时遗漏TTL。四种真实Forward入口（Chat/Messages × stream/nonstream）Red确认均为 expected2/actual0；补内部json:"-"元数据及copy后定向green，API输出不增加字段。
- 共用TTL解析器：HTTP、WS、typed转换复用同校验，未知/不完整/负数/小数/字符串/矛盾明细保持aggregate回退。WS多轮清理、progressive保留/覆盖和累计通过。
- Red复核：临时移除WS/typed adapter字段后回归均失败（/tmp/sub2api-postmerge-*-ttl-red-confirm.log），已恢复生产修复。
- 前端全量：428文件3406测试通过；make test-frontend加入真实运行时语言包测试，重新执行中。
- 后端 go build ./...、go vet ./... 退出0；WS与转换定向race退出0。
- 浏览器：正式构建preview的登录入口，mock本地公开配置，中英偏好刷新、无unsafe-eval CSP、390px宽度检查通过，无pageerror或编译/CSP错误。初版脚本错误假设登录页有语言按钮，已核实不存在并改为语言偏好刷新，不是产品缺陷。
- 自启动preview已停止；新图谱产物移到/tmp，不纳入业务diff。

## 验收与审查
- 无新增依赖/数据库迁移/API字段/权限变更；仅内部usage字段保真。生产计费行为从丢失明细回退恢复为既定TTL规则。
- 分支和工作区：feature/postmerge-v021-bugfix，修复未提交/未合并/未push。
- 待完成：最终后端完整单测、make前端必需检查，完成文档及diff复核。

## 最终交付与自审
- 两处业务缺陷（2/2）已复现、修复、Green与Red复核，正式回归留仓；没有放宽旧断言。
- go test -tags=unit ./... 退出0（service235.480s）；go build ./...、go vet ./... 退出0。内存阈值测试本轮全量通过，未修改阈值。
- WS与Chat/Messages修复定向go test -race通过。
- 前端428文件3406测试通过；make test-frontend退出0（lint/typecheck及31文件619测试）；pnpm run build退出0。
- 真实浏览器：生产构建中英标签、无unsafe-eval CSP、390px无横向溢出，无pageerror。API为本地mock。截图在/tmp/sub2api-postmerge-login-zh.png和-en.png。
- 数据库缓存TTL集成测试实际再次运行，但Docker不可用，harness主动跳过；不算验证通过。
- 现有测试仍有Vue stub警告与mock网络AggregateError日志，退出0；正式构建仍有大chunk警告。
- 无数据库结构/迁移/权限/API输出变更、无新依赖、无部署或推送；锁文件与package未改，Vite配置未改。
- gofmt与git diff --check通过。生成图谱和预览服务已清理。
- 当前修复留在feature/postmerge-v021-bugfix未提交，custom-main仍为519ee380e；不创建版本或tag。
- 无修复代码待办；真实供应商和数据库集成不在已验证范围。

## 复盘
- 假设验证：先发现本地v9翻译失败，经检查package/lock及frozen install证伪为仓库缺陷，撤回临时配置改动；以后先比对installed版本和lock版本。
- 数据契约：新增计费桶需检查HTTP解析、typed协议转换、WS relay逐轮合并/累计及adapter；共享已有校验避免规则漂移。
- 若上游后续为每条路径原生补齐TTL保真，可撤回本地适配并保留回归验证。

## 后续合并授权
- 用户明确要求“合并分支”，授权提交本轮已验证修复并合入custom-main。
- 合并前custom-main与feature/postmerge-v021-bugfix同为519ee380e，采用提交后fast-forward合并；最终提交点以Git历史为准。
- 复用上述完整验证结果，合并时检查diff、工作区及祖先关系；不push、tag或部署。
