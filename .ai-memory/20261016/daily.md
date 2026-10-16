session-id: 20261016-1025

# 工作日志 - 2026-10-16

## [10:25] 修改: API 密钥自动分组改名并梳理当前选号逻辑
- 分支: feature/auto-group-label；仅修改中文 label、shortLabel、configureTitle、分组选项 fallback label 及既有测试预期；未修改后端调度。
- 文件: frontend/src/i18n/locales/zh/dashboard.ts；frontend/src/views/user/keyGroupOptions.ts；frontend/src/views/user/__tests__/keyGroupOptions.spec.ts。
- 当前逻辑: 用户可绑定活跃 OpenAI 分组 + AllowAutoCheapestScheduling + 最高倍率（实际默认0.3）→用户覆盖倍率/默认倍率升序、sort_order、ID→所有关联自建池优先（池强制Legacy）→普通分组逐组先达标再可用性兜底→过滤后的账号channel_price升序，同价层复用健康/容量/公平排序；普通sticky可因更低价或健康逃逸，previous_response_id不可迁移时有强粘性。
- 失败: 单号失败加入请求排除集合，保留同组其他号；真正组耗尽才标记请求级跳过；组熔断60秒内2个不同用户触发、冷却10秒；符合条件的账号故障60秒内3次、隔离5分钟；首字超时默认关闭，开启后仅在未输出语义内容时换号，首字超时最多切换1次且关闭宽松兜底。Legacy/Shadow/调度未开启/健康读取异常会放宽质量筛选。
- 注意: 当前质量指健康与TTFT，不能证明模型能力“不降智”；部分前端倍率提示仍写0.2而实际默认0.3，未扩展修复范围；创建/编辑弹窗原本过滤自动选项，列表分组切换菜单提供自动选项，保持现状。
- 验证: pnpm exec vitest run src/views/user/__tests__/keyGroupOptions.spec.ts src/views/user/__tests__/KeysView.autoGroup.spec.ts src/i18n/__tests__/localeKeyCompleteness.spec.ts src/i18n/__tests__/localesMessageCompile.spec.ts（14通过）；定向eslint通过；pnpm run typecheck通过；pnpm exec vite build --outDir /tmp/sub2api-auto-group-build-20261016通过（已有Browserslist/分包体积提示）。
- 后端逻辑验证: go test ./internal/service -run '^(TestCandidateGroups|TestEffectiveOpenAIGroupRate|TestOpenAIAutoCheapest|TestAutoCheapestSharedPoolStageOrder)' -count=1（22通过）。
- UI: 使用当前真实页面及本地API桩，浏览器验证列表标签、列表分组菜单、配置弹窗新名称；截图/tmp/sub2api-auto-group-ui-20261016.png；未访问真实数据库或供应商。
- 下一步: 用户给出新选号规则后再调整；未commit/push/合并/部署。

## [10:34] 修改: 自动分组默认最高倍率统一为0.5
- 用户明确要求将默认最高倍率改成0.5；沿用feature/auto-group-label及上轮改名。
- 文件: backend/internal/service/api_key.go、api_key_service_test.go；frontend/src/views/user/KeysView.vue、__tests__/KeysView.autoGroup.spec.ts；frontend/src/i18n/locales/zh/dashboard.ts、en/dashboard.ts。
- 默认0.5覆盖新建、切换自动分组、空值/0提交及表单重置；前端表单初始值复用既有默认常量；中英文提示同步0.5。已有明确倍率仍原样保存（含0.3）；无数据库迁移或批量改值。
- 验证: pnpm exec vitest run src/views/user/__tests__/KeysView.autoGroup.spec.ts src/views/user/__tests__/keyGroupOptions.spec.ts src/i18n/__tests__/localeKeyCompleteness.spec.ts src/i18n/__tests__/localesMessageCompile.spec.ts（14通过）；定向ESLint、pnpm run typecheck通过；pnpm exec vite build --outDir /tmp/sub2api-auto-group-rate-build-20261016通过（已有Browserslist/分包体积提示）；git diff --check通过。
- 后端: go test ./internal/service -run '^(TestAPIKeyService(Create|Update)|TestCandidateGroups|TestEffectiveOpenAIGroupRate|TestOpenAIAutoCheapest|TestAutoCheapestSharedPoolStageOrder)' -count=1（29通过）。
- 页面: 新构建产物+本地API桩确认默认显示0.5、提示0.5、清空后实际PUT请求提交0.5及保存后重新打开为0.5；截图/tmp/sub2api-auto-group-default-rate-05.png。开发页面加载停滞后改用已通过构建的preview完成验证；未修改业务代码处理该验证环境问题。
- 状态: 用户本轮要求完成；未commit/push/合并/部署；生产数据及真实上游未验证；新选号规则仍待用户后续说明。

## [11:05] 修复: 自动不降智分组体验优先与故障切组
- 用户目标: 排查 0.2 组频繁报错却不切 0.3；体验第一。保留此前改名和默认最高倍率0.5。
- 已确认代码问题: 价格层内先宽松兜底压过高价达标账号；普通故障只排除账号导致换号预算耗在低价组；池模式同号重试放大耗时；HTTP200后断流及Go HTTP/2 reset/无终态EOF未计入账号隔离。生产具体触发原因仍待请求日志/密钥配置确认。
- 修复文件: backend/internal/service/openai_gateway_scheduling.go、openai_auto_cheapest_group_context.go；backend/internal/handler/failover_loop.go、openai_gateway_handler.go、openai_chat_completions.go、openai_images.go、openai_alpha_search.go。
- 策略: 全来源达标扫描后才宽松兜底；记录实际选过账号的物理来源，按本次排除账号数量将失败来源推后（共享池去重且按池ID降序）；自动组可迁移请求不走同号反复重试；固定组/必须原账号续链保留原窗口；HTTP200断流纳入原账号隔离预算；整个可用性阶段均失败后才记录组耗尽，避免尚可恢复的延后池被误记耗尽。
- 保留: 最高倍率硬上限、权限、模型、计费、部分输出后的禁止不安全重放、首字超时严格兜底边界、已有全局/分组调度开关及Legacy/Shadow/健康读取失败兼容；未改变生产配置或数据库。
- 失败复现: 三个真实handler入口，最高倍率0.5、一次换号预算、多个0.2故障号+0.3账号，旧代码调用20,20,20,20,21,21,21,21，无0.3；新实现20,30且HTTP200，结果文本和用量group_id=30通过。上限0.2则仅20,21。
- 测试: 新增backend/internal/handler/openai_auto_group_failover_test.go；更新service/openai_auto_cheapest_group_service_test.go、openai_auto_cheapest_group_context_circuit_test.go。覆盖文本/图片质量、上限、请求隔离、共享池、固定组/绑定续链、200后断流及取消/上游超时。
- Red→Green→Red确认: 修改前新增用例稳定失败；修改后通过；Go -overlay映射隔离旧源码再次稳定失败，未覆盖工作区。临时兼容shim仅让旧私有stage函数接受新增测试参数，不改变旧行为。
- 扩大验证: backend下 go test ./internal/service -run '^(TestOpenAI|TestAPIKeyService(Create|Update)|TestCandidateGroups|TestEffectiveOpenAIGroupRate|TestAutoCheapest|TestMarkOpenAIAutoCheapest|TestShouldQuarantine)' -count=1（1938通过）；go test -tags unit ./internal/handler -run '^(TestOpenAI|TestAutomaticGroupInflight|TestSameAccount|TestEffectiveSameAccount)' -count=1（325通过）；末次隔离函数调整后定向service36通过。go build -o /tmp/sub2api-auto-group-server-20261016 ./cmd/server通过；git diff --check和定向gofmt -l通过。
- 文档: docs/superpowers/specs/2026-10-16-openai-auto-group-experience-first.md，说明证据、策略与边界；该目录受现有.gitignore忽略，文档仅本地保存。
- 限制: 未访问真实数据库/上游，未确认线上密钥上限与调度开关；旧密钥明确保存0.2时不会因默认改0.5而自动允许0.3；No tool output等上下文缺失不能靠盲目重放保证恢复；质量指标不证明模型能力。
- 状态: feature/auto-group-label未commit/push/合并/部署；当前授权的本地修复完成。线上下一步核对反馈密钥上限与调度模式、请求错误轨迹，发布需另行授权。

## [11:13] 发布准备: v0.2.14.2提交、tag与推送
- 用户已明确授权commit/tag/push；功能提交1404a54e6ecfcc41fbadf73e5b680834af6c8017，包含改名、默认0.5、体验优先故障切组及回归测试。
- 版本: 当前代码基于v0.2.16.1，仍按用户指定使用v0.2.14.2；仅同步backend/cmd/server/VERSION，不回退源码。该编号低于既有版本，版本比较更新可能不提示升级。
- 本轮验证: go build -o /tmp/sub2api-release-v0.2.14.2 ./cmd/server通过；-version显示Sub2API 0.2.14.2；bash backend/scripts/resolve-version.sh返回0.2.14.2；git diff --check和暂存区检查通过。
- 发布工具: 两个Python运行环境缺少PyYAML，随后在临时venv安装requirements-release.txt并执行四段版本及tag/commit绑定两项测试，均通过；未修改项目依赖或CI。
- 纠正上条描述: 共享池用池ID归因和去重，排序依据为本请求各物理来源失败次数升序，并非池ID降序。
- 发布范围: 快进合回custom-main；annotated tag v0.2.14.2；仅推送origin/custom-main与该精确tag，不推main/upstream或其他tags。远端refs核验及工作流状态以实际推送后结果为准。
- 自动化: 远端RELEASE_ONLY=true，tag触发现有Release归档流程，不发布Docker镜像；默认分支main的VERSION由既有workflow在Release成功后同步，未改workflow。
- 未验证: 生产部署、真实数据库/供应商、反馈用户密钥上限及调度开关；已有显式上限0.2仍阻止切0.3。

## [11:31] 修复: 创建/编辑密钥缺少自动不降智分组选项
- 现场: 用户截图为创建密钥→OpenAI→分组下拉，普通分组在、自动选项消失。
- 根因: KeysView.vue的formGroupOptions在创建和编辑两条路径均显式过滤kind=openai_auto_cheapest；git blame定位到既有e946d24b8b。上一轮仅覆盖列表入口，创建测试直接emit自动值，未检验真实选项可见性，漏检此入口。
- 范围: 从custom-main=67026acee创建feature/fix-auto-group-form-option；修改KeysView.vue及KeysView.autoGroup.spec.ts、KeysView.spec.ts。恢复创建OpenAI和编辑自动选项；其他创建厂商仍按platform过滤；切换厂商经selectedGroupOptionValue清空模式与group_id，候选更新也按完整选择值校验。
- Red: 更新测试后旧源码30通过6失败，包含实际下拉无选项、编辑缺项及切厂商自动模式残留。Green: pnpm exec vitest run src/views/user/__tests__/KeysView.autoGroup.spec.ts src/views/user/__tests__/KeysView.spec.ts src/views/user/__tests__/keyGroupOptions.spec.ts（38通过）；定向ESLint、pnpm run typecheck、Vite构建与git diff --check通过。
- Red确认: 临时Vite插件仅在测试加载时使用HEAD旧KeysView.vue，真实下拉回归再次稳定失败；未还原或覆盖工作区文件。
- 浏览器: 当前构建+本地API桩确认OpenAI下拉自动选项可点击且默认0.5；切Anthropic后选择与倍率栏清空，未重新选组时无创建请求；切回OpenAI实际创建提交自动模式、group_id=null、上限0.5；编辑已自动密钥选项显示正确，实际PUT保留显式0.3。截图/tmp/sub2api-auto-group-form-option-restored.png。
- 发布实况补记: 上一轮已原子推送origin/custom-main与annotated tag v0.2.14.2，远端分支和peeled tag均为67026aceeed2522d2aa6c0dc7eee3399a1950eaa；Release运行38844543275已触发。本轮修复未包含在该tag内，不移动已发布tag。
- 限制: 本轮仅本地修复，未commit/push/部署；未访问真实数据库、供应商或生产页面；构建保留既有Browserslist/大分包提示。

## [11:34] 发布准备: v0.2.16.3提交/tag/push
- 用户明确指定v0.2.16.3并授权提交推送；弹窗修复提交2ea5869262688cb17a676470e78537c132637716，VERSION已同步0.2.16.3。
- 复用上一轮38项前端回归、ESLint/typecheck/build及真实页面/请求验证；本轮go build -o /tmp/sub2api-release-v0.2.16.3 ./cmd/server通过，-version与版本解析均为0.2.16.3；发布工具四段tag及commit绑定两项测试通过。
- 范围: 从feature/fix-auto-group-form-option快进合入custom-main，创建annotated tag v0.2.16.3并原子推送origin/custom-main和精确tag；旧tag保持原样。本条为推送前记录，最终以远端refs核验为准。
- 发布沿用RELEASE_ONLY=true，仅创建GitHub Release与归档；未执行生产部署，真实供应商/数据库未验证。

## [17:23] 实现: 分组用量刷新与更新时间优化
- 用户已授权按建议修改；分支feature/group-usage-refresh，未commit/push/合并/部署。
- backend/internal/service/dashboard_service.go：缓存新鲜期5分钟改30秒；新增同一份数据的快照时间/过期/刷新失败元数据；显式刷新等待singleflight查询，失败保留旧值及旧时间并遵循30秒失败冷却。
- backend/internal/handler/admin/group_handler.go与frontend/src/api/admin/groups.ts：新增可选refresh、include_metadata，默认数组契约保留；旧后端数组兼容，不伪造更新时间。
- frontend/src/views/admin/GroupsView.vue及中英文文案：可见页面每30秒读取用量，隐藏/KeepAlive失活/卸载暂停并取消请求；点击刷新等待最新快照，保留现有数字；工具栏显示更新时间和失败提示；累计附保留记录口径说明，未返回的分组用量显示占位符。
- 验证：后端定向service/admin handler与race均通过；前端分组页/API/i18n共43项通过；typecheck、定向ESLint、生产build、git diff --check通过。
- 浏览器：真实生产构建+本地API fixtures，10项检查通过，覆盖真实按钮请求参数、等待中旧值、失败与恢复、30秒轮询、手机布局；桌面与手机截图及verification.json保存在本聊天visualizations/group-usage。
- 自审：无SQL/迁移/权限/生产配置变更；无新依赖；保留官方按天汇总与索引修复。查询频率增加由30秒缓存、并发合并、可见性暂停和失败冷却控制；生产负载与真实数据库未测，不声称性能提速。原有构建告警仍在。
- 参考：.ai-memory/20261016/group-usage-refresh-plan.md；临时图谱Go/Vue索引不全，已用rg调用链复核并清理本轮生成文件。

## 合并授权：分组用量刷新优化
- 用户明确要求合并分支，授权提交本次优化并合入本地custom-main；不包含push、tag、发布、部署或版本号修改。
- 合并前核对：feature/group-usage-refresh与custom-main均基于ae85f049b，改动均属于本轮任务；目标分支未被其他worktree占用，可快进合并，git diff --check通过。
- 本轮沿用上条后端定向/race、前端43项及lint/typecheck/build、页面10项检查结果；本次仅提交与合并，无业务代码追加修改。

## 同步：上游v0.2.17合入custom-main
- 用户明确要求同步最新迭代并合回custom-main。开始时工作区干净，custom-main=18a01cb9e（已包含本轮分组用量优化）；git fetch upstream后，main由9c2cca9e6快进到upstream/main=b0a02a330，共47个新提交，采用merge合入custom-main。
- 4处文本冲突：VERSION采用上游0.2.17；account_test_service.go同时保留二改connect_timing和上游提示词/图片事件；AccountTableFilters.vue及其modify/delete测试冲突保留本地内联布局、无障碍标签和回归测试。AccountsView保留原有lg伸缩类，避免与本地w-full筛选栏组合后桌面工具栏换行。
- 兼容修正：上游新增Codex导入测试两处NewAccountHandler调用补齐本地依赖参数；门票compact回归改为覆盖可用模型正常回退、GPT-5.5已退役时保留原模型门控；新增Antigravity真实请求路径提示词/图片/耗时联合回归。
- 验证：初次后端范围回归36个包通过，admin编译和service两项断言失败；修正测试兼容问题后admin整包通过，service整包复测通过（211.670s），新联合回归通过。内存断言初次整包运行波动，单独连续3次及后续整包通过，未放宽断言。总计38个后端包最终通过，go build通过。
- 前端：26文件294项通过，布局收尾后9文件56项复测通过；typecheck、修改文件ESLint、生产build通过；真实构建+本地API fixtures共13项交互通过，账号/分组桌面和390px截图已检查，pageErrors为空。
- 保留：分组用量30秒缓存/刷新/元数据实现与本次同步前完全一致；无数据库迁移、依赖变更或生产配置修改。上游新规则禁止ChatGPT登录类账号路由GPT-5.5，API Key上游不受该退役规则影响，已有映射不会自动迁移。
- 范围：仅本地main同步及custom-main合并提交，不push/tag/发布/部署；未连接生产数据库、真实支付或供应商。构建保留已有Browserslist/分包提示；上线前建议核对GPT-5.5映射及真实充值退款流程。
- 详情：.ai-memory/20261016/upstream-v0217-sync.md；Git最终提交点以合并历史为准。

## [18:09] 排查/修复：分组用量两次读取的快照一致性
- 用户要求查看其他Bug并修复；custom-main=edaf58734且工作区干净，从其创建feature/postmerge-bugfixes。未commit/push/合并/发布/部署。
- 排查了本次上游更新与二改交点、用量刷新、模型退役/映射及自动调度；前端全量407文件3114项通过，后端repository基线unit通过。
- 已确认根因：custom_group_usage_rollup_repo.go读取水位和汇总金额分为两条SQL，缺少同一数据库快照；并发日志清理可混读旧日桶与新日志。隔离PostgreSQL 18集成Red实测：清理前1.75、清理后0，原实现返回1.25。
- 修复：DB/Conn开启ReadOnly+RepeatableRead短事务；保留参数化尾段索引SQL、日桶规则及调用方事务生命周期，查询/提交失败不返回金额。新增并发清理和下一次查询看到0的回归，以及begin/watermark/summary/commit失败回归。
- Green：repository整包unit、9个分组日桶/用量PostgreSQL集成测试通过；服务端build通过。race检查进行中，最终以收尾记录为准。
- 图谱：辅助查询返回Go符号索引缺失，已用rg复核真实调用链；只清理本轮生成的图谱产物。无数据库迁移/生产数据/配置/扣费写入变更；未验证生产负载。详见postmerge-bugfixes.md。
- 收尾验证：repository/service/admin handler用量相关unit -race三包通过；PostgreSQL并发清理集成-race连续3次通过，下一次读0也通过；服务端构建、gofmt、git diff --check通过。源文件仅仓储实现与两份测试，未提交/合并/推送。

## [18:12] 合并授权：上游合并后的用量一致性修复
- 用户明确要求合并分支，授权提交feature/postmerge-bugfixes的本轮修复和记录，再快进合入本地custom-main。
- 合并前两分支均为edaf58734，目标分支未占用；仅3份Go实现/测试和4份本轮记录，git diff --check通过。复用本轮已通过的测试与构建，不增加业务代码。
- 范围：本地提交与git merge --ff-only；无push/tag/发布/部署或版本号变更，最终状态以Git核验为准。

## [18:17] 发布准备：v0.2.16.4提交/tag/push
- 用户明确授权打tag v0.2.16.4并提交推送；基线custom-main=cf988d214（已包含本轮修复与上游v0.2.17），VERSION按用户指定改0.2.16.4。
- 版本构建、二进制-version、版本解析与两项四段tag/commit绑定发布工具测试通过；业务代码沿用本轮验证，无额外业务修改。
- fetch origin后确认远端custom-main=ae85f049b可快进，远端tag不存在；计划annotated tag和atomic push，仅custom-main及该tag。RELEASE_ONLY=true，沿用GitHub Release归档流程，无生产部署。推送完成以远端refs与CI状态核验为准，详见release-v0.2.16.4.md。

## [18:35] 发布准备：v0.2.17.1提交/tag/push
- 用户明确授权v0.2.17.1提交、tag、推送；基线custom-main=7543bb1f7与origin一致，工作区干净，远端tag不存在。
- VERSION改0.2.17.1，go build及二进制-version通过；业务代码与v0.2.16.4一致，复用已通过业务与发布工具验证，未追加业务修改。
- 计划annotated tag及atomic push，仅custom-main与精确tag；RELEASE_ONLY=true，沿用Release归档流程，旧tag保持原样，无生产部署。结果以远端refs及CI核验为准；详见release-v0.2.17.1.md。
