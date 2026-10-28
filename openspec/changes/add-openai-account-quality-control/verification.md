# 调查证据与设计验证记录

日期：2026-10-28；基线：`96f8b5034`（`v0.2.18.1`）；分支：`feature/openai-account-quality-control`。

## 1. 交付状态与边界

当前交付包含需求/设计文档、固定版本的开源源码快照，以及本地实现：质量判分与状态机、PostgreSQL 持久化/任务队列/管理 API、OpenAI/Codex HTTP 与 WS 探测适配、自动不降智调度门、监控页和配置页。两页已支持结果详情、状态事件、批量检测/配置、完整分页载入及服务端聚合时间线。自动检测及控制仍默认关闭。真实管理员浏览器交互、完整全路径集成和多实例故障验收未完成；生产迁移、真实模型调用、部署和启用未执行。

新增迁移文件 `248_account_quality_control.sql` 只在仓库中落盘；前序曾在隔离 PostgreSQL 验证迁移/数据往返，未操作生产数据库。本地没有发起真实模型调用、提交、push、部署或启用控制。局部验证与剩余缺口记录在 §11。

## 2. 前序网站与相关开源核验

以下保留同一任务前序调查结论，不代表本轮重新请求这些线上端点。

| 来源 | 核验结论 | 解释边界 |
| --- | --- | --- |
| [manxue.ai](https://manxue.ai/) 与 [API 说明](https://manxue.ai/api) | 公开异步测试接口，POST /api/v1/tests，按 ID GET 查询结果 | 可参考产品流程，不将第三方检测接入内部凭据主链路 |
| [相关开源项目](https://github.com/w1196396546/manxue-ai) | Apache-2.0；Python/SQLite/Playwright；代码关联旧域名 | 不能据此证明现站完整实现和分类器均已开源 |
| 开源糖果判分 | 最终正文含独立 21 可通过 | 本系统拟严格解析唯一 JSON，避免仅提到正确数字就通过 |
| 开源鹈鹕审核 | SVG 多帧截图 + 视觉模型 | 与现站 HTML 源码分类器不同，后者代码/权重未在该仓库找到 |
| 公开监控数据一次实测 | HTTP 200；60 条中 passed=3、incorrect=1、error=56 | 75%=3/4 有效答案通过率，不能视为 75% 可用率；也不是本站账号样本 |

开源来源若未来实际复制使用，需按 Apache-2.0 保留对应许可与来源声明；首期拟独立实现本地确定性判分，不复制未知分类器。

## 3. 当前项目代码证据

| 文件/能力 | 核对结果及设计影响 |
| --- | --- |
| account_test_service.go / RunPromptBackground | 已能指定账号/模型/Prompt；成功是执行完成；质量调用需补显式参数和隔离统计副作用 |
| scheduled_test_runner_service.go | 普通测试成功可恢复可用性状态；不得解除质量隔离 |
| pelican_test_service.go | previewable/unpreviewable/failed 是生成预览状态，没有质量分类器 |
| openai_account_scheduler.go | 已有 ReasoningEffort，需补有效 ServiceTier 全链路；新/旧/来源/会话均需接入 |
| openai_ws_client.go / openai_ws_forwarder_v2.go / openai_gateway_handler.go | WS 需独立探测及每 turn 准入，不能只在握手检查 |
| admin_auth.go / adminaccess/catalog.go | IsAdmin 管理员与受限管理 API Key；不存在本需求应自造的“超级管理员/子管理员”层级 |
| response/response.go | 已有 Accepted=202、Created=201、分页及标准 envelope，应复用 |
| admin/idempotency_helper.go / service/idempotency.go | JSON helper 固定 200；coordinator=nil 和默认 ObserveOnly 存在降级；通用回调后另存回包存在事务窗口，本模块需局部事务适配 |
| config/config.go | 已有 Timezone 配置与默认 Asia/Shanghai；预算复用配置，数据库时间统一判日 |
| 迁移 runner 与迁移约定（前序核对） | SHA256 不可变、forward-only、启动自动迁移，不得将可执行 Down SQL 放同文件 |

真实路径前缀见 design §2，实施时需重新核对调用关系。新文件/字段/路由名称均为拟议，不冒充当前已有能力。

## 4. 设计自审及修订

- 将最初两表草案扩为四表，补齐事件原子性和集群每日预算。
- 补充 scheduled_slot 的数据库唯一约束、同范围单活动策略、daily_call_limit 与恢复窗口起点。
- 将“连续”明确为滑动时间窗中的有效答案序列；unknown/error 不切断但也不延长旧证据期限。
- 将控制 epoch 新周期、旧任务 fencing、观察历史和启控实际影响分别定义。
- 明确 API 使用真实管理员角色和既有响应结构，复用实际幂等错误码。
- 明确业务提交与可重放回包同事务，避免原任务完成后重试再调用。
- 将缓存同步收敛为首期完整轻快照，避免 updated_at 游标漏掉长事务提交。
- 明确跨日预算、发送意图先落库、受理未知保守消费，不承诺上游 exactly-once。
- 明确存储故障拒绝是推荐方案 D05，紧急停用本身也可能因数据库不可写失败。
- 补齐判题前提、严格输出、单题记忆/相关误差风险与真实效果待验证边界。

## 5. 数学与文档检查

糖果题已用 Python 标准库穷举两种形状各口味数量组合：圆形 8×10×9、五角星 8×7×5，共 201,600 种分配。对每个圆/星取数方案检查是否存在不含目标搭配的分配，得到：

```text
shape_selection_minimum 21
minimum_shape_plans [(9, 12)]
uncontrolled_shape_guarantee 29
allocation_states_checked 201600
```

文档检查包括：UTF-8/末尾换行/无尾随空白，代码围栏配对，文档相对链接/锚点有效，Q01–Q12 均有验收和追踪，D01–D09 齐全，引用的现有代码文件存在；并用 `git diff --check` 和 `git status --short --branch` 确认改动范围。结果以各版实际工具执行输出为依据。

v1.0 实际执行：Python 文档检查退出 0，8 文件、19 个本地链接、9 个 JSON 示例、18 个现有源码路径检查通过，12 项需求、9 项决策、40 个验收场景，issues=[]；Python 数学枚举退出 0，输出与上文一致；`git diff --check` 退出 0；`git log -1 --format='%h %s'` 确认基线为 `96f8b5034 chore: release v0.2.18.1`。这些一次性脚本通过终端 heredoc 执行，没有加入业务测试目录。

检查不得仅依赖 `git diff --check`，因为这些设计文档仍为未跟踪文件。完整性检查须直接读取该目录全部 Markdown。

## 6. 前序现有功能基线（本轮未重跑）

以下是前序调查阶段已执行结果，保留以便交接；本次只调整文档，未重复运行。

backend 目录：

```sh
go test -tags=unit ./internal/service -run '^(TestPelican|TestEvaluateOpenAISchedulerPolicy|TestOpenAIAutoSchedulerSelector)' -count=1
```

退出 0，29 项通过。验证的是现有生成/调度基线，不是新增质量能力。

frontend 目录：

```sh
pnpm exec vitest run src/components/admin/account/__tests__/PelicanBatchTestDialog.spec.ts src/i18n/__tests__/localeKeyCompleteness.spec.ts
```

退出 0，10 项通过。验证的是现有组件和文案。

## 7. 设计阶段尚未验证事项（历史记录）

本节反映 v1.3 设计阶段的状态，已被 §11 的当前实现记录取代。当前未验证项包括多实例并发/崩溃故障、完整 HTTP/API/WS 桩流程、管理员浏览器交互、详情和事件页面、缓存性能、停控回退演练及真实模型效果；生产迁移、部署、真实账号检测和启用仍未执行。

逐项代码与验证状态以 [tasks.md](./tasks.md) 和本记录 §11 为准，不把设计文档检查当成功能验收。

## 8. v1.1 页面与范围修订

用户补充：“本次需求有新页面吗，有配置页面吗，OpenAI 分组只对不降智分组进行检测处理”。本版新增 ui-design.md，明确监控/配置两个独立路由，并同步所有需求、数据、API、任务和验收。

本次只读核对新增证据：

- frontend/src/views/user/keyGroupOptions.ts：既有选项标签“自动不降智分组”，值 openai_auto_cheapest。
- backend/internal/service/openai_auto_cheapest_group_service.go：CandidateGroups 过滤 OpenAI、活跃、非自建池、允许自动调度、用户授权与最高倍率；自建池经来源解析被引用。
- backend/internal/service/openai_auto_cheapest_group_context.go：已有原始 auto 请求上下文、实际来源追踪和失败来源后置。
- backend/internal/service/openai_gateway_scheduling.go：effectiveKey 可带实际 GroupID，但原始模式仍应保留；当前 scheduler 按 qualifiedOnly 两轮遍历全部来源。本版纠正 v1.0 对“廉价组内先宽松兜底”的旧规则描述。
- backend/ent/schema/group.go、frontend/src/router/index.ts、frontend/src/components/layout/AppSidebar.vue：参与自动调度、引用池的真实字段，以及两独立页面可沿用的路由/导航结构。

新增验收 Q01-05、Q07-04、Q11-06、Q11-07，覆盖库存资格变化、两页面、同账号 auto/fixed 边界、内部切组与 WS 模式传递。当前共 44 个场景。此次只改文档，未运行功能测试或真实检测。

v1.1 实际执行：Python 文档/范围检查退出 0，9 文件、25 个本地链接、9 个 JSON 示例、24 个现有源码路径、44 个场景、2 个前端页面路由检查通过，issues=[]；git diff --check 退出 0。git status --short --branch 确认仍只有本设计目录未跟踪，无业务文件变更。数学结论和旧功能基线沿用前序记录，本版未重跑。

## 9. v1.2 原版代码引入与监控模块验证

用户明确要求参考开源已有功能并把代码搬入。已克隆公开仓库，只读取代码，未运行其服务、部署脚本或真实 API 调用。固定提交 5da79674417dd0748682f09b7b7467efd1968ea8，复制全部 17 个 Git 跟踪文件，共 161,925 字节；原文件未修改，新增本地 README、NOTICE 与 SHA-256 清单。

源代码确认了监控/配置页、糖果题/独立数字判分、SVG 校验和四帧视觉审核确实存在。本版将 app.js 的时间线与 Monitor.state 的统计移植为 qualityMonitor.ts，修正范围混合、窗口边界和统计分母；未声称原版 Monitor.state 就使用本方案的有效答案分母。

实际执行：

```sh
# frontend 目录
pnpm exec vitest run src/components/admin/account-quality/__tests__/qualityMonitor.spec.ts
pnpm run typecheck
```

两条退出 0。Vitest 8 项通过，覆盖空样本、3/4 有效通过但 56 次错误、策略/证据范围隔离与去重、滚动窗口/时隙边界、混合状态下钻、非答案结果和无界配置。vue-tsc --noEmit 通过。

根目录通过 Python 验证 source-manifest 全部 17 文件大小/哈希；用 ast.parse 检查原始 3 个 Python 文件，用 node --check 检查原始 3 个 JavaScript 文件，均退出 0。这些语法检查不执行原站，也不证明 Playwright/上游连接可运行；未安装其依赖。

新增两份 TypeScript 文件的定向 ESLint 检查退出 0，No issues found。最终文档/源码完整性检查退出 0：10 份方案文档 + 本地来源 README、40 个本地链接、9 个 JSON 示例、17 个原始文件、45 个验收场景，issues=[]；git diff --check 退出 0。git status 确认变更仅为本功能文档、新监控模块/测试和 third_party 源码引入，没有修改既有文件或提交/push。

在 v1.2 交付时，Vue 页面/路由、管理 API、Go 检测和调度尚未接通。该阶段状态已被本记录 §11 的本地实现更新取代；新增源码复用验收 Q07-05，当时累计45个场景。

## 10. v1.3 整体设计补全

按用户“继续完成原有设计”继续完善文档，本轮未修改业务代码、原始源码、数据库或生产配置。

- 新增implementation-guide，将两页/API/原版函数/拟议组件逐项连接，提供完整观察、启控、三错隔离、最短冷却及两次通过恢复示例。
- 补GET /overview和GET /policies/:id/monitor，明确完整聚合、服务端as_of、scope/epoch、最多288桶和每桶5示例ID、分页下钻及跨请求变化。
- 修正此前“直接接入前端监控模块”的笼统表述：模块只适用于完整且预先过滤代次的小数据集，正式页面不能从一页历史推算全量24H。
- 补全局默认值创建时快照、规则/开关编辑与任务/排期/代次的生效矩阵；当前冷却不追溯改写，自动关闭不解除隔离。
- 增加阈值理想跨度硬校验、预算缺口/延迟警告、规划覆盖与实测覆盖区分，核对4800/1000、864/1000、144min等容量示例。
- 新增10项场景，累计55项；都是待实施验收规范，不是本轮已经通过55项功能测试。

本轮实际执行Python终端文档检查：11份方案文档及来源README，61个本地链接/锚点、10个完整JSON示例、Q01–Q12/D01–D09、55个独立场景检查通过；围栏/UTF-8/末尾换行/尾随空白、聚合示例计数一致，issues=[]。同脚本逐文件复核17个原版文件的大小/哈希和文件集合，仍为161,925字节，全部通过。`git diff --check`退出0；由于文档未跟踪，空白/链接检查直接读取全部文件，不依赖Git diff覆盖。

没有重跑无关前后端测试；本轮无新增可运行页面，不执行虚假的浏览器验收。此前8项监控模块测试、typecheck及原版语法结果沿用§9记录，未当作本轮新验证或完整功能证据。真实效果与生产启用另列E阶段。

## 11. 本地实现与验证（2026-10-28）

### 实现范围

- `backend/internal/accountquality/` 实现严格 JSON 判分、状态归约、策略/运行/事件/预算持久化、手动与定时队列、租约/取消/保留清理、管理 API 和全量监控聚合；新增 forward-only 迁移 `backend/migrations/248_account_quality_control.sql`。
- `backend/internal/service/account_quality_provider.go` 复用 OpenAI/Codex 账号凭据、模型映射、并发槽和代理，支持 Responses HTTP、Chat Completions HTTP 与独立 Responses WS v2 探测；不走普通用户计费/速度记录路径。
- `backend/internal/service/openai_account_quality_gate.go` 将质量决策限制在服务端建立的原始自动不降智请求上下文，并在账号选择和 HTTP/WS 出站前复核。固定分组路径不安装该门；全局检测/控制与策略开关的初始值均关闭。
- 管理端新增检测监控和检测配置两个路由/导航页、API 模块、中英文文案及账号摘要跳转。监控页包含详情抽屉、状态事件、任务取消、筛选和批量检测；配置页支持全量分页目标/策略载入、逐账号来源上下文、批量创建/更新与保存后检测。服务端负责全量 24H 聚合，开源适配模块只转换其有界时间桶供 Vue 渲染，不从分页历史推算总量。
- 原始 `manxue-ai` 源码仍保留 Apache-2.0 许可、NOTICE 和 SHA-256 清单。其 HTML 源码分类器未确认开源，没有被假设为已复用。

### 本轮执行的验证

| 命令/检查 | 结果 |
| --- | --- |
| `go test ./internal/service ./internal/accountquality -count=1` | 退出 0；7,159 项通过 |
| `go test ./internal/handler/admin ./internal/server/routes -count=1` | 退出 0；585 项通过 |
| 判分/状态/范围定向测试 | 退出 0；24 项通过 |
| `go test ./cmd/server -count=1` | 退出 0；4 项通过，覆盖 Wire 生成调用与清理构造 |
| `pnpm run typecheck`（frontend） | 退出 0 |
| 监控模块 Vitest | 10 项通过，包含服务端完整桶适配和不完整聚合拒绝 |
| `pnpm run check:i18n`（frontend） | 退出 0；3 项通过 |
| `pnpm run build`（frontend） | 退出 0；Vite 提示项目既有动态导入拆包和大 chunk 警告，另提示 caniuse-lite 数据较旧 |
| 新增页面/API/监控模块定向 ESLint | 退出 0；无问题 |
| `git diff --check` 与 11 份 OpenSpec Markdown 空白检查 | 退出 0；无空白问题，构建文件未列入 Git 变更 |
| 隔离 PostgreSQL 迁移/往返 | 前序实现会话中通过；本轮未重跑 |

### 未完成或未验证

- 监控页已提供策略详情抽屉、状态事件、分组/状态/证据新鲜度筛选、批量检测、任务取消和结果正文查看；当前仍缺“当前范围全部代次历史”切换、跨策略事件筛选及事件时间/类型筛选。
- 配置页支持分页完整载入合格目标与策略、逐账号来源上下文选择、批量创建与批量规则更新；批量操作每次最多 100 项并由服务端逐项预览。
- 浏览器打开本地地址后被登录守卫送到登录页；没有真实管理员会话，因此未完成真实页面交互验收，也未创建或提交测试账号。
- 多实例预算竞争、worker 崩溃/晚到回调、数据库不可用、重启无可信快照、性能 P95、旧版回退和所有账号隔离故障场景未演练。
- 未执行生产迁移/部署/启用，未向任何真实 OpenAI/Codex 账号发起检测。真实题目通过率、误隔离率和恢复时延仍未知。
- 当前只有本地代码和自动化检查证据；不能据此视为已在生产保护账号。
