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
