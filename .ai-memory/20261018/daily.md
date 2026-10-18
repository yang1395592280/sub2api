# 工作日志 - 2026-10-18
session-id: 20261018-1559

## [15:59] 功能实现：集中二改开关与 V1 分组账号主动监控
- 分支 feature/custom-controls-active-monitor；系统设置新增二改功能，21 个开关与自定义菜单逐项启停，局部 PATCH 保留其他设置。原参数页保留详细配置、避免回写集中开关；礼遇关闭同时阻断业务接口并刷新入口状态。
- V1 新增分组卡片和批量配置，免站内 Key；每轮固定一个组内账号逐模型执行、不失败换号；鹈鹕弹窗集成历史/沙箱预览/人工评估、账号快照及真实上游消息/请求 ID。普通用户基于分组/订阅权限过滤、服务端 DTO 脱敏。
- 持久化迁移 246、数据库租约/令牌、日预算/错峰/超时/恢复/清理及 Wire 生命周期接入。修复 Claude Code 限制误测回退组、计划保存后分组改为 composite 的边界；未提供的上游 ID 不伪造。
- 验证：相关 Go 定向回归通过；最新分组测试 9 项、鹈鹕/输出上限 14 项、礼遇路由门禁 1 项通过；前端相关 55 项通过、最后组件/页面 52 项再次通过，typecheck/改动文件 ESLint/中英文完整性/build、Go build、git diff --check 通过。
- PostgreSQL 初因 Docker 未启动跳过，随后启动本机现有 Docker，隔离 PostgreSQL 18.1/Redis 容器实测 2 项通过：迁移、并发领取/预算/UTC 跨日、旧 token 防覆盖、历史摘要和单条 HTML、评估/保留/恢复。测试容器已清理。
- 真实前端应用路由配合本地模拟 API 完成浏览器验证：批量 2 组/2 模型暂停保存、鹈鹕预览及评估、局部菜单与礼遇开关保存成功。截图为模拟数据。详细记录 docs/channel-monitor-v1-group-design.md。
- 限制：只支持普通分组；复合路由和独立自托管池暂不支持。账号直接抽测消耗上游额度、不覆盖 Key 鉴权/计费完整链路；真实供应商与长期调度/生产迁移未验证；新计划默认暂停，鹈鹕总开关默认关闭。未 commit/merge/push/部署。
- 约定：后续新增自定义开关需加入集中白名单与双语文案，并保持旧配置 key 和默认行为。撤销条件：将来需要复合路由或端到端计费监控时另行设计，不能静默改用别组账号。下一步：如准备部署，先在测试环境以少量真实请求核验供应商 ID 与模型兼容性。

## [16:00] 最终收尾：配置缺省一致性与验证环境清理
- CustomFeatureEnabled 与集中设置读取共用默认值，空值保留升级默认，Codex 打票沿配置 fallback；3 项相关设置回归通过。临时 UI/模拟 API 进程与标签页已关闭，测试容器自动清理，无运行中的 Docker 容器；清理本轮生成的后端二进制。未进行生产操作。

## [21:27] 合并授权：二改开关与 V1 分组监控
- 用户明确要求合并分支，授权提交本轮实现及相关记录，再将 feature/custom-controls-active-monitor 快进合入本地 custom-main。
- 合并前两分支均为 e4fe28310，目标未被其他 worktree 占用，改动范围与本轮实现一致；git diff --check 通过。复用已通过的 Go/前端/数据库测试及构建，不追加业务变更。
- 范围仅本地 commit 与 merge --ff-only；不 push/tag/发布/部署或改版本号。合并结果以最终 Git 核验为准。

## [21:31] v0.2.17.2 发布准备
- 本地功能提交 6cc187d5b 已无冲突快进合入 custom-main。用户进一步授权提交、打 tag v0.2.17.2 并推送，更新 VERSION 与本轮发布记录。
- 嵌入发布版 Go 构建、产物 -version、版本解析通过，均为 0.2.17.2；发布工具四段版本与必须绑定 tag 两项测试通过，git diff --check 通过。业务验证复用本日已完成的回归/集成/构建与浏览器记录。
- fetch 后 origin/custom-main=e4fe28310，目标 tag 本地及远端不存在；精确范围为 annotated tag v0.2.17.2 与 origin/custom-main 的原子推送，不推其他 refs、不移动旧 tag。
- RELEASE_ONLY=true，沿用 Release 归档及 checksums，不发布 Docker/GHCR；未执行生产部署/迁移。迁移 246、新计划暂停和鹈鹕默认关闭等限制见 release-v0.2.17.2.md。本条为推送前记录，最终状态以远端核验为准。

## 自定义菜单默认图标修复（v0.2.17.2 后）
- 用户截图中的「GPT 一键配置」来自自定义菜单，未上传 icon_svg 时 NavItem.icon 为 null，因此侧栏没有图标。基于 custom-main=076d8c8ee 新建 feature/custom-menu-default-icon，在 AppSidebar.vue 复用现有 terminal 图标作为默认值，覆盖用户、管理员个人区和管理员普通/简洁模式；空白 SVG 按缺省处理，已上传 SVG 继续优先显示。
- 验证：`pnpm exec vitest run src/components/layout/__tests__/AppSidebar.spec.ts` 12 项通过；该文件 ESLint、`pnpm run typecheck`、`pnpm run build`（含双语完整性 3 项及 vue-tsc）通过，`git diff --check` 通过。没有为此低影响图标修复新增单元测试。
- 实际 AppSidebar 组件配本地模拟菜单完成浏览器交互：缺省图标 20px、空白 SVG 回退、上传 SVG 颜色保留、管理员菜单、深色文字同色和折叠 20px 图标。截图保存在本轮 visualizations/sidebar-default-icon.png；临时预览文件/服务和标签页在收尾时清理。
- 修改留在功能分支未提交工作区，尚未 merge/push/tag/部署，不属于已发布的 v0.2.17.2。无需修改现有菜单数据，更新前端构建后即可生效。

## [22:02] v0.2.17.3 发布准备
- 用户新授权提交、打 tag v0.2.17.3 并推送。图标修复提交 51c4e0052 已从 feature/custom-menu-default-icon 无冲突快进合入 custom-main，VERSION 更新为 0.2.17.3。
- fetch 后 origin/custom-main=076d8c8ee，目标 tag 本地/远端均不存在。嵌入发布构建、二进制 -version 和版本解析通过，两项发布工具测试通过；复用上节图标修复回归、质量、构建和浏览器证据。
- 仅原子推送 origin/custom-main 与 annotated tag v0.2.17.3，保留旧 tag。RELEASE_ONLY=true，仅 Release 归档及 checksums。当前为推送前记录，结果以远端 refs 和工作流核验为准；未部署生产。

## [22:43] 撤回本次监控/鹈鹕增强，保留集中设置
- 用户要求先撤回本次渠道监控和鹈鹕增强，只保留集中管理，并提供名单确认。在 feature/retain-custom-feature-settings 上撤回新分组监控/计划/历史/追踪/API/组件/Wire，原有账号鹈鹕和渠道监控相关 14 个文件精确恢复 e4fe28310；保留集中设置、菜单启停及图标、礼遇总开关。
- 移除三项新监控开关，集中 API 类型拆为 customFeatures.ts。18 项开关与中英文集合一致；按 upstream v0.2.17=b0a02a330 核对，5 个 key 为定制新增，13 个上游已有。仅给名单，不擅自收窄其余收录范围，详见 docs/custom-feature-settings-inventory.md。
- 相关 Go 118 项、前端 103 项、ESLint/typecheck/前端 build（含 3 项双语检查）/Go build/diff 检查通过；真实设置组件配模拟 API 验证三项消失及局部 PATCH 精确保存。详情与命令见 rollback-monitor-keep-settings.md。
- migration 246 按已发布历史迁移保留且 checksum 不变，两个新表不再被使用；不删表、不改生产设置。未提交/合并/推送/打 tag/部署；临时 UI 文件、服务和标签页收尾清理。等待用户确认集中页最终收录名单。

## [22:53] 实现：集中开关收窄第一批与社群入口
- 按用户确认收窄前后端白名单为礼遇、兑换购买、社群三项，双语同步；原其他开关存储值不动。HomeView简洁首页复用现有社群组件。
- 新增回归在修复前执行：6项中2项按预期失败（简洁首页缺链接/二维码按钮），其余4项通过。计划及恢复快照见custom-scope-community-plan.md；本批尚待Green和原设置编辑恢复。

## [23:05] 完成：二改名单收窄与社群入口修复
- 「二改功能」GET/PATCH、前端集合和双语仅保留 `zenxiang_liyu_enabled`、`redeem_code_purchase_enabled`、`join_group_enabled` 与自定义菜单；此前被错误集中管理的15项恢复原设置页编辑与保存，存量设置值未改。
- 修复简洁首页遗漏的 JoinGroupEntryButton，链接模式显示跳转入口，二维码模式点击显示弹窗；集中页在开启社群入口后提供前往通用设置填写链接/二维码的提示。更新 docs/custom-feature-settings-inventory.md。
- 验证：修复前HomeView回归6项中2项按预期失败，修复后前端56项、Go定向7项通过；ESLint、vue-tsc、前端生产构建、Go build、diff检查通过。浏览器本地模拟公开设置验证链接与二维码弹窗，临时index配置/Vite/标签页已清理；未连接生产、未提交/推送/部署。

## [23:10] Git：提交并准备合并
- 用户授权将 feature/retain-custom-feature-settings 合并到本地 custom-main。已将撤回监控、最终三项集中设置、社群入口修复、测试和记录提交；合并前 feature 与 custom-main 同起点，使用 fast-forward，不推送、不打 tag、不部署。

## [23:15] 发布准备：v0.2.17.4
- 用户授权提交、tag、推送。版本从0.2.17.3更新为0.2.17.4，并加入发布说明；嵌入式release二进制-version正确输出0.2.17.4。发布工具的9项非镜像测试和镜像脚本语法检查通过；3项镜像脚本执行测试受macOS Bash 3.2不支持`${var,,}`限制，GitHub Ubuntu runner支持该语法。待创建tag、校验并原子推送custom-main和tag；不部署生产。
