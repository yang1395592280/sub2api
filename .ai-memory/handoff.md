# 修复交接

已完成：4 个 Bug 根因修复；57 个测试包、12105 项顶层测试通过；专项 race、go build、git diff --check 通过。
相关文件：backend/internal/handler/gateway_inflight_reservation.go、openai_gateway_handler.go、openai_chat_completions.go；repository/billing_inflight_cache.go、billing_cache.go、api_key_repo.go；service/api_key_service.go、billing_cache_service.go、billing_inflight_reservation.go 及回归测试。
发布准备：用户已授权提交、打标签和推送 v0.2.11.1；VERSION 为 0.2.11.1，版本解析与二进制验证通过。Git 发布范围为 origin/custom-main 和 annotated tag v0.2.11.1；最终结果以远端分支与 peeled tag commit 核验为准。
未完成：Docker 可用后运行 TestAPIKeyCreateLock_ConcurrentRepositoriesRespectLimit 的 PostgreSQL 集成测试；上线时统一重启后端，避免新旧余额缓存 key 混用。发布工具 9 项测试通过，2 项镜像脚本测试受本机 Bash 3.2 限制，须在 Ubuntu/Bash 4+ 环境验证；远端构建结果需另行核验。
阻塞：Git 发布操作无已知阻塞；真实 PostgreSQL 集成验证与镜像脚本本地验证受上述环境限制。

## 2026-10-11 上游同步检查点

已完成：main 快进到 upstream/main=3a6fd1c9d（v0.2.15），custom-main 的 13 个文本冲突已整合；前端 3014 项测试和质量/构建检查、后端默认测试与相关 unit/go vet/普通及嵌入构建通过。
相关文件：13 个冲突文件、合规状态回归、嵌入静态资源测试；完整清单与命令见 .ai-memory/20261011/upstream-sync.md。
未完成：本次未授权 push 或部署；后续部署前需验证真实数据库迁移 242 与登录后核心功能。
阻塞：本地同步无已知阻塞。golangci-lint 未安装，未执行该项检查；真实 PostgreSQL/Redis 集成和浏览器验收尚未执行。

## 2026-10-11 Bug 修复与自填模型检查点

已完成：3 个 Bug 修复及鹈鹕自填模型 ID；后端全量 unit、go vet、embed 测试，前端 10 项测试、lint、build 通过；实际弹窗的手填优先及模型列表失败路径经浏览器本地请求桩验证。
相关文件：backend/internal/handler/openai_gateway_handler.go、backend/internal/service/api_key_service.go、pelican_test_service.go、account_test_service_cn_adaptive.go、frontend/src/components/admin/account/PelicanBatchTestDialog.vue 及回归测试/中英文文案；完整清单见 .ai-memory/20261011/bugfix-pelican-model.md。
未完成：本轮需求无遗留实现项；真实供应商与 PostgreSQL/Redis 集成验证未执行。未 push、部署、执行迁移。
阻塞：本地交付无已知阻塞；自动模式后续每轮多一次 Lite 分组读取，自填模型仍受账号模型配置约束。

## 2026-10-11 v0.2.15.1 发布检查点

已完成：用户已授权本次提交/tag/push；VERSION=0.2.15.1，解析脚本、嵌入构建、二进制版本输出通过，发布工具四段版本与 tag/commit 绑定两项测试通过。
相关文件：backend/cmd/server/VERSION、.github/workflows/release.yml、.github/release-tools/release_matrix.py；发布包含 custom-main 上游同步和 815d8a9e2 的修复。
验收范围：origin/custom-main 和 annotated tag v0.2.15.1；两者远端 commit 应相同。未授权 main/upstream 推送或生产部署。
未完成：Release 归档/镜像构建结果需单独查看远端工作流；真实供应商及 PostgreSQL/Redis 集成验证仍未执行。
阻塞：Git 发布准备无已知阻塞。

## 2026-10-15 账号筛选与兼容性修复检查点

已完成：去掉更多筛选折叠；修复分组视频模型、搜索和三项音频价格漏返回；回归先失败后通过。535 项 admin/DTO、105 项冷却/计费、100 项 embed、29 项前端相关测试通过，lint/typecheck/vet/构建通过。真实账号页浏览器本地 API 桩验证 320–1600px、组合筛选、搜索无结果、清空分组和长名称。
相关文件：AccountTableFilters.vue、dto/mappers.go、两组后端价格回归、筛选测试和中英文文案；详情见 20261015/bugfix-account-filters.md。代码 55a18dcfb 已从 feature 快进合回 custom-main，无冲突。
未完成：本轮无遗留实现项；未 push/部署/迁移，真实 PostgreSQL/Redis/供应商集成仍未执行；迁移 242–245 上线前需验证。main/upstream/main=9c2cca9e6，合并提交=2ec18bf30。
阻塞：本地修复无已知阻塞；golangci-lint 未安装，未执行该项。

## 2026-10-15 v0.2.16.1 发布准备

已完成：用户授权本次 commit/tag/push，VERSION=0.2.16.1；嵌入构建与二进制版本输出、发布工具四段版本及 tag/commit 绑定两项测试通过。发布包含上游合并 2ec18bf30 和修复 55a18dcfb。
相关文件：backend/cmd/server/VERSION、20261015/session_memory_release_v0.2.16.1.jsonl。
验收范围：origin/custom-main 与 annotated tag v0.2.16.1，推送后远端分支与 peeled tag 应指向同一发布提交。本记录为推送前状态，结果以远端核验为准。
未完成：Release CI 归档/镜像构建结果需另行核验；真实 PostgreSQL/Redis/供应商集成未执行，未执行生产部署或迁移。
阻塞：Git 发布准备无已知阻塞。

## 2026-10-16 v0.2.14.2 自动分组发布准备

已完成：自动不降智分组改名、默认最高倍率0.5、体验优先故障切组，功能提交1404a54；service1938项、handler325项、最终定向service36项及前端14项通过。发布版构建与-version显示0.2.14.2，四段版本及tag/commit绑定两项发布测试通过。
相关文件：backend/cmd/server/VERSION、handler/failover_loop.go及OpenAI入口、service/openai_gateway_scheduling.go、openai_auto_cheapest_group_context.go、frontend/src/views/user/KeysView.vue及回归测试；详情见20261016/daily.md。
未完成：用户已授权提交/tag/push，本记录为推送前状态；将快进合回custom-main并原子推送origin/custom-main和annotated tag v0.2.14.2，两者须指向同一发布提交。远端RELEASE_ONLY=true，仅生成Release归档；生产尚未部署，真实上游和用户密钥上限待核验。
阻塞：Git发布无已知阻塞。代码基线包含v0.2.16.1，按用户指定采用较低编号v0.2.14.2，版本比较更新可能不提示；旧密钥明确上限0.2不会自动放宽。

## 2026-10-16 创建/编辑密钥自动分组选项修复

已完成：v0.2.14.2已推送，远端custom-main与peeled tag同为67026acee。后续截图定位到弹窗显式过滤自动项；已在feature/fix-auto-group-form-option恢复创建OpenAI和编辑选项、修复切厂商模式残留，38项前端回归、ESLint/typecheck/build通过；真实构建页面+本地API桩验证创建0.5、编辑保留0.3、切厂商无非法提交，旧组件验证再次失败。
相关文件：frontend/src/views/user/KeysView.vue:1682、__tests__/KeysView.autoGroup.spec.ts、__tests__/KeysView.spec.ts；详情20261016/daily.md。
未完成：本轮修复未提交/推送/部署，尚未包含在已发布v0.2.14.2；后续补丁发布应使用新tag。真实生产与供应商未验证。
阻塞：本地修复无已知阻塞，不覆盖已有发布tag。

## 2026-10-16 v0.2.16.3 发布准备

已完成：用户授权commit/tag/push v0.2.16.3，弹窗修复提交2ea5869；VERSION、发布版二进制-version和解析均为0.2.16.3，两项发布工具测试通过。前端38项、lint/typecheck/build及实际创建/编辑请求验证已完成。
相关文件：backend/cmd/server/VERSION、frontend/src/views/user/KeysView.vue及两组回归；详情20261016/daily.md。
未完成：本条为推送前记录，将快进合回custom-main并原子推送分支和annotated tag v0.2.16.3，远端两者须指向同一发布提交；Release状态以实际运行核验为准，生产尚未部署。
阻塞：Git发布无已知阻塞；沿用RELEASE_ONLY=true，仅生成Release归档，不发布Docker镜像。

## 2026-10-18 二改开关与 V1 分组监控（实现中）
- 已完成：feature/custom-controls-active-monitor 上新增 246 增量迁移、分组监控 service/repository/API 初稿；账号测试追踪 wrapper；二改开关局部保存 API；菜单停用和礼遇路由控制。尚未编译验证，不计作交付完成。
- 相关文件：docs/channel-monitor-v1-group-design.md；backend/internal/service/channel_group_monitor.go、account_test_trace.go、setting_custom_features.go；对应 repository/handler/routes。
- 未完成：编译修正、Wire 生成、前端设置分组与分组卡片/鹈鹕弹窗、核心测试、实际 UI 验证、最终审查。
- 阻塞：无。未连接生产，未发真实模型请求。新计划默认关闭，由管理员配置后开启。

## 2026-10-18 二改监控第二检查点
- 已完成带证据：Go 相关 71 条测试通过；前端 typecheck、改动组件 lint 通过；新组件测试与双语完整性测试通过。Wire 已生成，核心前后端联通代码齐备。
- 相关文件：新增 CustomFeaturesSettings.vue、GroupMonitorPanel.vue、groupMonitor.ts；新增 service/handler 单元测试与 repository 租约/预算集成测试。
- 未完成：旧 SettingsView 测试已按新集中保存行为更新，待回归；构建、浏览器实际交互、权限 HTTP 测试、最终审查及文档收尾。
- 阻塞：Docker CLI 在，但 daemon 不可连接；仓库数据库集成 harness 因此跳过，尚未实测 PostgreSQL 迁移/多实例领取。无真实供应商请求。

## 2026-10-18 二改开关与 V1 分组监控最终检查点
- 已完成带证据：集中 21 个开关与自定义菜单启停、V1 批量分组账号抽测、内嵌鹈鹕历史/沙箱/人工评估/管理员 ID、用户服务端权限与脱敏、数据库租约/预算/超时恢复和 Wire 生命周期。相关 Go 回归、前端相关 55 项及最新 52 项、前后端构建/typecheck/ESLint/双语检查通过，真实页面配本地模拟 API 交互验证通过。
- 数据库阻塞已解除：启动现有本地 Docker，PostgreSQL 18.1/Redis 隔离容器中 2 项集成实际通过（迁移、并发/预算/跨日、旧令牌防覆盖、历史摘要/HTML单条、评估、保留和恢复）；容器已自动清理，初次跳过不计通过。
- 相关文件：docs/channel-monitor-v1-group-design.md；service/channel_group_monitor.go、account_test_trace.go、setting_custom_features.go；对应 repository/handler/routes/tests；frontend CustomFeaturesSettings.vue、GroupMonitorPanel.vue、groupMonitor.ts。完整实现留在 feature/custom-controls-active-monitor 未提交工作区。
- 未完成：真实供应商与长期运行/生产迁移尚未验证；本轮未授权 commit/merge/push/部署。只支持普通分组，复合路由和独立自托管池需后续设计；新计划默认暂停、鹈鹕总开关默认关闭。测试环境少量请求联调是上线前下一步。
- 阻塞：本地实现无阻塞。截图均为模拟数据；直接账号抽测消耗上游额度，不覆盖用户 Key 鉴权及计费链路。

## 2026-10-18 二改监控本地合并授权
- 用户已要求合并 feature/custom-controls-active-monitor，授权提交本轮实现及记录并快进合入本地 custom-main；合并前两分支均为 e4fe28310，目标未被其他 worktree 占用，无分叉。
- 复用已通过的相关 Go、前端、隔离 PostgreSQL 集成与构建证据，不追加业务变更。
- 本次范围为本地提交与 git merge --ff-only；不推送、不打 tag、不部署、不执行生产迁移。最终提交点、包含关系与干净状态以合并后的 Git 核验为准。

## 2026-10-18 v0.2.17.2 发布准备
- 已完成：功能提交 6cc187d5b 已无冲突快进合入 custom-main；用户新授权 commit/tag/push v0.2.17.2。VERSION、嵌入发布构建产物 -version 与解析均为 0.2.17.2，发布工具两项测试通过，复用本日功能回归与真实隔离 PostgreSQL 集成证据。
- 相关文件：backend/cmd/server/VERSION、20261018/release-v0.2.17.2.md、session_memory_release_v0.2.17.2.jsonl；功能详情见 docs/channel-monitor-v1-group-design.md。
- 验收范围：仅原子推送 origin/custom-main 和 annotated tag v0.2.17.2，远端分支与 peeled tag 须指向同一发布提交；本条为推送前记录，以后续 refs 和 Release 工作流核验为准。RELEASE_ONLY=true，仅 Release 归档及 checksums。
- 未完成：未生产部署/执行迁移 246，真实供应商及长期调度尚未验证；只支持普通分组，新计划默认暂停、鹈鹕总开关默认关闭。
- 阻塞：Git 发布准备无已知阻塞，不更改旧 tag、其他分支或 CI 配置。

## 2026-10-18 自定义菜单默认图标
- 已完成：feature/custom-menu-default-icon 上为无 SVG 的自定义菜单提供现有 terminal 默认图标，修复「GPT 一键配置」图标缺失；覆盖三处菜单构建，空白 SVG 回退，上传图标优先。
- 相关文件：frontend/src/components/layout/AppSidebar.vue；详情见同日 daily.md。侧栏现有 12 项回归、ESLint/typecheck/build（含 3 项双语检查）通过，真实组件配模拟菜单验证缺省/上传/管理员/深色/折叠显示。
- 未完成：代码尚未提交/合并/推送/部署，未包含于已发布 v0.2.17.2。无已知实现阻塞，部署新的前端构建后生效。

## 2026-10-18 v0.2.17.3 发布准备
- 已完成：用户授权 commit/tag/push，图标修复提交 51c4e0052 已无冲突快进合入 custom-main；VERSION=0.2.17.3，嵌入发布构建、二进制版本、解析与两项发布工具测试通过，复用本轮侧栏 12 项及前端质量/构建/浏览器验证。
- 相关文件：frontend/src/components/layout/AppSidebar.vue、backend/cmd/server/VERSION；详情见 20261018/release-v0.2.17.3.md。
- 验收范围：仅原子推送 origin/custom-main 与 annotated tag v0.2.17.3，远端分支及 peeled tag 须指向同一发布提交；本条为推送前记录，以后续 refs 与 Release 核验为准。RELEASE_ONLY=true，不发布 Docker/GHCR。
- 未完成：未生产部署，图标需新前端构建上线才生效。无已知发布准备阻塞。

## 2026-10-18 撤回监控与鹈鹕新增实现
- 用户最新要求取代此前监控扩展方案：在 feature/retain-custom-feature-settings 撤回 6cc187d5b 中 GroupMonitor/监控鹈鹕和测试追踪，保留集中设置、菜单管理及图标；原监控/账号鹈鹕/Wire 等 14 文件精确恢复 e4fe28310。已发布 v0.2.17.2/3 不改写。
- 三项新监控开关已移除，余下 18 项与菜单暂保留，等待用户按来源名单确认最终收录范围；来源比对当前已合入 upstream v0.2.17：5 个 key 定制新增，13 个上游已有，不等于项目仅有 5 个二改能力。名单 docs/custom-feature-settings-inventory.md。
- Go 118 项、前端 103 项、质量/类型/前后端构建通过；真实设置组件和模拟 API 验证过期控件移除及局部保存；详见 20261018/rollback-monitor-keep-settings.md。
- 迁移 246 原文件/checksum 保留，两个表不再使用；未执行生产迁移/删表/部署。修改尚未提交或合并推送，无已知实现阻塞；下一步等待用户确认 5 项加菜单是否足够，或是否保留其他现有开关。

## 2026-10-18 集中页归属纠正第一检查点
- 已完成：用户确认非本人二改项移出集中页，API白名单、前端keys和双语已收窄到3项；简洁首页入口组件已补，修复前失败测试捕获两种缺失。
- 相关文件：setting_custom_features.go、customFeatures.ts、中英文customFeatures.ts、HomeView.vue；计划20261018/custom-scope-community-plan.md。
- 未完成：恢复15项原设置编辑/保存，更新测试/清单，运行Green/构建和浏览器验证；代码均未提交。
- 阻塞：无，未访问生产配置，线上是否还缺URL/二维码待管理员配置核验。

## 2026-10-18 集中页归属纠正完成
- 已完成带证据：集中页最终仅有礼遇、兑换购买、社群三个开关和自定义菜单；15项原功能开关已恢复在对应设置页保存。简洁首页补社群入口，链接和二维码弹窗两种模式的浏览器本地模拟公开配置均实际验证。
- 相关文件：backend/internal/service/setting_custom_features.go、frontend/src/api/customFeatures.ts、frontend/src/views/admin/SettingsView.vue、frontend/src/views/HomeView.vue、docs/custom-feature-settings-inventory.md；详细计划与Red证据见20261018/custom-scope-community-plan.md。
- 验证：前端56项、Go定向7项、ESLint、typecheck、pnpm build（含双语3项）、go build、diff检查通过。临时页面配置、Vite服务及浏览器标签均已清理。
- 未完成：本轮已提交，待快进合并到本地custom-main；不推送、不打tag或部署。生产配置未读取，管理员仍需填写链接或二维码才能展示社群入口。
- 阻塞：无。

## 2026-10-18 v0.2.17.4 发布准备
- 已完成：用户授权本次提交、annotated tag和推送。VERSION已更新为0.2.17.4，release二进制-version及发布矩阵9项非镜像测试通过；镜像脚本语法检查通过。
- 相关文件：backend/cmd/server/VERSION、20261018/release-v0.2.17.4.md；待提交、创建tag并原子推送origin/custom-main与refs/tags/v0.2.17.4。
- 限制：3个镜像脚本执行测试在本机Bash 3.2因`${var,,}`不支持无法运行；GitHub Actions Ubuntu runner具备Bash 4+。未部署生产、未执行迁移。

## 2026-10-20 礼遇余额缓存403修复
- 已完成带证据：绿云用户1287缓存单键失效，回源后与数据库余额一致；抽奖/幸运币/猜大小事务成功后同步失效余额与鉴权缓存；13个HTTP+miniredis/Lua场景及相关三包回归、race、后端build、diff检查通过；旧逻辑overlay再次触发失败。
- 相关文件：backend/internal/service/zenxiang_liyu_service.go、service/wire.go、cmd/server/wire_gen.go、handler/zenxiang_liyu_balance_cache_test.go；详情20261020/liyu-balance-cache-plan.md和daily.md。
- 未完成：本地修复在feature/fix-liyu-balance-cache，未commit/merge/push/部署；无生产礼遇/收费请求测试，未批量处理其他用户历史缓存。
- 阻塞：本地实现无阻塞；发布部署需按用户后续指令执行。

## 2026-10-20 17:10 本地合并授权
- 用户明确要求“合并分支”，现已授权提交本次修复并合并到本地 custom-main；此前未授权提交/合并的记录为历史状态。
- 合并前两分支均为 42f9e7ed4，目标分支未被其他 worktree 占用，仅本次 8 个文件待提交，diff 检查通过；下一步提交并以 merge --ff-only 合入，核对工作区干净。
- 测试结论沿用前阶段通过结果；本轮不推送、不打 tag、不发布、不部署、不删除分支。
