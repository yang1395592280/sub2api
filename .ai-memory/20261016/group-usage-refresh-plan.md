# 分组用量刷新优化

用户已授权按建议实施。分支：feature/group-usage-refresh。

## 实施与验收

1. 后端将分组缓存新鲜期改为30秒；保留官方日汇总SQL、独立30秒查询超时、singleflight与失败冷却。新增快照读取，返回同一份数据的生成时间、过期状态和刷新失败状态。手动刷新等待共享查询，失败保留旧快照。验收：缓存、并发刷新、取消、失败回退单元测试。
2. GET /admin/groups/usage-summary 可选 refresh=true 和 include_metadata=true；默认数组响应保持兼容，元数据模式返回 items/updated_at/stale/refresh_failed。参数使用布尔校验，沿用管理员权限与简单模式限制。验收：真实handler请求的旧/新响应、非法参数、刷新与失败用例。
3. GroupsView通过现有getUsageSummary请求元数据；30秒可见页刷新，KeepAlive失活、隐藏页面及卸载停止定时器并取消请求；手动刷新先完成在途普通请求，再请求最新结果。数据刷新不清空现有数值；缺失数据展示占位符；用量更新时间与异常提示放在工具栏，累计增加保留记录口径说明。旧后端数组响应不伪造更新时间。验收：真实按钮、可见性、KeepAlive、卸载、并发和失败测试，以及构建页面交互与截图。
4. 执行后端相关测试及race、前端相关回归与i18n/typecheck/lint/build；完成自审并记录结果。

## 影响面与风险

真实调用已用rg核对：GroupsView -> adminAPI.groups.getUsageSummary -> GroupHandler.GetUsageSummary -> DashboardService -> UsageLogRepository.GetAllGroupUsageSummary -> 官方日汇总仓储；服务方法的现有调用保持签名与数组语义。

预计修改4个实现模块、2个语言文件及对应测试。无数据库迁移、生产配置、权限变更或新依赖；查询频率增加由30秒缓存、并发合并、页面可见性和失败冷却控制。无生产数据库连接，因此不声称生产性能收益。

## 规划门禁

A1-A10通过：需求逐项映射、命令/测试/截图证据、兼容接口、可逆工作区diff、现有二改保留、干净分支已确认。B1/B2/B7/B8/B10不适用（无DB写入/新依赖/批处理/不可逆操作/新增CLI）。B3/B4/B5/B6/B9/B11按上文并发、性能、兼容性、测试、影响面与回读实施；图谱作为辅助，rg为事实依据。C1/C2：后端完成、前端完成及验证完成时更新检查点。失败回退当前实现步骤，禁止重置用户工作区。

风格基线：Go遵循gofmt和既有service/handler分层；Vue沿用双引号、分号、script setup与项目现有控件；API与测试遵循各自既有格式。TDD与代码审查在本会话内执行，不委派。

## 检查点

已完成：后端快照与兼容接口、前端可见性刷新和更新时间、旧值保留及口径提示；后端定向与race通过；前端43项回归、typecheck、ESLint、生产构建通过；真实构建页面10项交互检查、桌面/刷新中/失败/手机截图均完成，页面无运行时异常。图谱对Go/Vue符号缺失，已按真实rg调用链复核，生成的辅助图谱已清理。
未完成：无本地实现遗留；未连接生产数据库进行耗时或负载验证，未提交/推送/部署。
阻塞：无。

## 验证与自审

- backend：`go test -tags=unit ./internal/service ./internal/handler/admin -run 'GroupUsageSummary|GroupHandlerUsageSummary|SimpleModeBlocksAdvancedOperations' -count=1` 通过；同命令增加 `-race` 通过。
- frontend：`pnpm exec vitest run src/views/admin/__tests__/GroupsView src/api/__tests__/admin.groups.usage-summary.spec.ts src/i18n/__tests__/localeKeyCompleteness.spec.ts`，8文件43项通过；`pnpm run typecheck`、定向ESLint、`pnpm run build`通过。
- 真实页面：临时Playwright脚本配合本地API fixtures，点击真实刷新按钮检查refresh=true、并发等待期间金额不消失、失败不改时间和金额、恢复清除提示、30秒普通轮询、390px手机无横向溢出。截图与verification.json位于本聊天visualizations的group-usage目录。
- 首轮旧前端断言仍要求无参数调用，已按新增兼容参数更新并扩展行为断言。首次浏览器测试遇到新账号引导遮罩，已使用真实引导已读storage键；直接缩放时的过渡动画影响截图，改用初始390px页面验证，未改业务代码或放宽无溢出断言。
- 全量diff已检查：官方日汇总SQL、数据库迁移、权限与生产配置无变化；新参数为可选，默认旧接口响应保留。失败/取消/恢复/缓存隔离/旧后端兼容/页签缓存均有测试证据。构建仍有原有Browserslist、动态分包和大包提示。

## 本地合并授权

用户随后明确要求合并分支。合并前feature/group-usage-refresh与custom-main均指向ae85f049b，工作区仅含本轮优化及记录文件，目标分支未被其他worktree占用。提交上述改动后采用git merge --ff-only合入custom-main，并检查分支包含关系、冲突和工作区状态。本次无新增业务修改，沿用已完成验证；不推送、打tag、发布、部署或修改版本号。
