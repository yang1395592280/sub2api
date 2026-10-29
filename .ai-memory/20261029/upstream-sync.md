# 2026-10-29 上游同步

- 目标：同步 upstream/main 到本地 main，再以 merge 合入 custom-main；保留二改历史和功能。
- 同步前工作区干净：main=3f1c8f36c，custom-main=96d5b7024。
- 已获取上游 a3298b46e（v0.2.19），相对旧 main 新增 73 个提交，164 个文件变更；本地 main 已快进。
- 范围：代码同步、必要冲突适配、本地验证、合并提交；本次不推送、不打标签、不部署、不执行数据库迁移。

## 整合与验证计划

- 分组 ImageCountBilling 字段贯通 API、缓存、Repository 和 Ent，保留二改号池、调度、余额刷新、价格保护字段。
- 账号探测额外信息同时保留上游 Grok 目录观测/CAS 数据及二改锁内合并数据，配套 SQL Mock 列与行同步。
- 图片请求保留二改过刷成功重置、模型映射日志，同时接入上游按实际张数计费及上传诊断。
- Ent 以合并后的 Schema 重新生成，认证快照版本递增以同时失效两侧旧缓存；VERSION 采用上游 0.2.19。
- 验证：前端 lint/typecheck/关键及重叠模块测试/构建，Go 定向及全量 unit、vet、embed 构建与静态页面冒烟，Git 祖先关系/冲突标记/空白检查。
- 数据迁移：上游新增 248_group_image_count_billing.sql，默认 false；现有 248_account_quality_control.sql 按完整文件名记录，均保留，不修改已发布迁移。
- 风险：文本自动合并不代表行为兼容，重点验证元数据并发保存、图片计费、调度及质量检测；真实 PostgreSQL/Redis、付费上游和生产迁移不在本轮验证环境内。

## 进度

- 14 个冲突文件均已整合：VERSION、4 个 Ent 生成文件、GroupHandler、DTO 映射、账号 Repository 及 2 个相关测试文件、APIKey Repository、认证快照、2 个图片请求文件。
- Ent 首次生成因生成文件中的冲突标记无法加载依赖；仅将 8 个受影响生成文件恢复到本次同步前有效版本，再按合并后的 Schema 重新生成成功。原合并输入备份于任务临时目录，未改动 Schema 二改字段。
- 两侧认证快照版本都为 25，合并后使用 26，确保旧缓存失效，调度/价格保护/按张计费字段能一同重建。
- Account Repository 的锁内查询同时读取 Grok 目录、上游能力 metadata、完整 extra；4 个 SQL Mock 调用文件同步为 15 列。回归覆盖同次保存同时保留最新 Codex 门票和 Grok 元数据。
- 前端 lint/typecheck、30 个文件 607 项关键测试、12 个文件 209 项相关测试、生产构建通过。构建有非阻断的 chunk 大小和 Browserslist 旧数据提示；既有 SettingsView 测试有组件 mock/无后端请求日志，测试通过。
- 后端首轮全量 unit 暴露上游新增 Grok Handler 测试的 3 个构造调用不兼容，已补上二改余额/签到/调度结果上报依赖，admin 包复验通过。
- 同轮发现二改质量身份保留逻辑把缺失字段写成 null，破坏显式清空 extra 的语义；保持原测试断言，修为仅保留实际存在的字段，缺失时移除表单伪造值，补齐 2 个防注入/替换用例，定向复验通过。
- 锁内合并及认证快照定向 race 测试通过（repository/service）。质量身份修复后，全量 unit、vet 和 embed 构建最终复验均通过；embed web 测试通过。
- 浏览器使用生产静态构建和隔离的公开设置 fixture，首页、登录入口点击、空表单必填校验均实际验证并截图；首次 SPA 导航遇到一次 CSS preload 失败，直接加载后重试相同导航通过。未登录或连接实际后台。

## 检查点

- 已完成项（带证据）：main 快进到 a3298b46e、冲突整合、前端 816 项测试与构建通过；admin、merge race、质量身份定向及全量 unit 测试通过，日志见任务临时目录。
- 相关文件：本文记录的分组映射/缓存、账号锁内合并、图片请求及 admin_account.go；最终差异以 Git index 为准。
- 未完成项：本记录随 merge commit 提交，最终提交号与祖先关系以 Git 核验结果为准；代码验证已完成。
- 阻塞项：无；真实数据库、Redis、上游付费请求和部署未验证，不作为本地代码合并已通过的声明。

## 最终验证证据

- make test-frontend：lint、typecheck、30 个文件 607 项测试通过。
- 相关模块 Vitest：12 个文件 209 项测试通过，覆盖 Grok 目录、编辑弹窗、分组、质量检测及 API。
- pnpm --dir frontend run build：i18n、vue-tsc、Vite 构建通过。
- go test -race -tags=unit ./internal/repository ./internal/service -run 'TestLockAndMergeAccount(Extra|ProbeExtra)|TestAPIKeyAuthSnapshotRoundTrip_PreservesAutomaticSchedulingAndPriceGuard|TestAPIKeyService_Rejects' -count=1：通过。
- go test -tags=unit ./internal/handler/admin：通过。
- 质量身份定向回归：原显式清空 extra 测试和新增防伪造/替换测试均通过。
- make test-unit 最终全量：通过，service 包 226.027s，其余包通过或无测试；保留首轮失败日志与最终结果，不放宽测试。
- go vet ./... 最终：通过。
- CGO_ENABLED=0 go build -tags=embed -trimpath -o 临时目录/server ./cmd/server：通过；-version 输出 Sub2API 0.2.19。
- go test -tags=embed ./internal/web：通过。
- 暂存 diff --check、未解决冲突检查、冲突标记扫描：通过。main 与 upstream/main 左右差异为 0/0。
- 原始日志与合并输入备份：/tmp/sub2api-sync-20261029.hG8hAp/。本地页面冒烟截图在本会话工具输出中。
- golangci-lint 未安装，未运行；真实 PostgreSQL/Redis 集成测试、实际账号请求及管理员登录后操作未验证。

## 发布注意和决策

- VERSION 跟随上游 0.2.19，本次不创建二改发布版本或标签，不执行 push。
- 上线前先备份数据库，并在测试环境验证 248_group_image_count_billing.sql。新增字段默认 false；迁移 runner 按完整文件名记录，同号的二改质量控制迁移可共存，未修改已发布迁移。
- 认证缓存必须同时承载两侧字段，故选版本 26；保留任一侧版本 25 无法可靠区分其旧快照。撤销条件为认证快照结构再次演进时继续升版，不能退回旧版本复用缓存。
- 撤销本次代码整合应单独制定 revert merge 方案，保留二改历史；数据库回滚另行评估，不能直接删除列或覆盖迁移记录。
