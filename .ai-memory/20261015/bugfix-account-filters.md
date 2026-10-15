# 账号筛选与合并后兼容性检查

目标：移除账号页“更多筛选”折叠；检查近期合并相关链路并修复可复现问题。
分支：feature/fix-upstream-compatibility-account-filters，从 custom-main=2ec18bf30 创建。

## 方案与影响面
- AccountTableFilters.vue 平铺搜索、平台、类型、状态、Privacy、分组，保留事件、选项与窄屏换行；调用方 AccountsView.vue。
- groupFromServiceBase 补齐既有五项计价字段的返回映射；复用调用方为 GroupFromService/Admin/Shallow。价格 JSON 定义和 GroupsView 的编辑保存已存在，不新增契约或迁移。
- 保留利润字段仅 admin 可见的边界，不改数据库/生产配置，不 push。
- 不引入新工具/抽象；折叠 UI 方案舍弃原因是用户明确要求平铺。撤销条件：用户重新要求折叠，或测试证明窄屏布局无法使用。

## 状态
- [x] 实码和引用复核；工作区干净后创建 feature 分支。
- [x] 现有五项组件测试修改前通过；调整平铺实现后五项仍通过。
- [x] DTO 回归先复现失败：非零与零价均被映射成 null；已补齐五项映射。
- [x] DTO 同模块与近期冷却/计费相关测试。
- [x] 前端 lint/typecheck/测试/build 和真实页面宽窄屏交互。
- [x] diff 自审、收尾记录、合回 custom-main。

已知限制：未做真实 PostgreSQL/Redis、供应商及生产部署验证。

## 验证与交付

- 修复前价格回归失败：1.25 和显式 0 都返回 null；补齐 mapper 后 nil/零/非零价格用例通过。
- go test ./internal/handler/admin ./internal/handler/dto：535 项通过。覆盖三类管理读取入口及只改名称的 PUT，利润控制与 simple-mode allowlist 边界未回归；两包 go vet 通过。
- go test -tags=unit ./internal/service -run 相关用例：冷却范围、OpenAI runtime block、429 和 ReserveFixedCharge 共 105 项通过；未发现第二个可确认的后端 bug。
- pnpm --dir frontend run lint:check、typecheck、build 通过；6 个相关测试文件 29 项通过（含筛选 5 项），中英文键完整性 3 项通过。旧 AccountsView 测试桩遗漏 listLatestPelicanTests 产生捕获日志但无失败，实际浏览器无此错误。
- go test -tags=embed ./internal/web：100 项通过；go build -tags=embed -trimpath -o /tmp/sub2api-account-filter-review/server ./cmd/server 通过。
- 本地 Vite + 真实 AccountsView + Playwright + 本地 API 请求桩：五项筛选、组合请求、搜索无结果、清空分组通过；320/375/768/1280/1600px 筛选容器无横向溢出；长选中分组名窄屏正常；console/page error 为 0。未访问真实服务。
- git diff --check 通过；Group DTO 字段枚举无遗漏，moreFilters 引用为 0。
- 代码提交 55a18dcfb，feature 已 fast-forward 合回 custom-main，无冲突；main/upstream/main=9c2cca9e6（v0.2.16），未 push/部署/迁移。
- 已知限制：真实 PostgreSQL/Redis、供应商未验证；golangci-lint 未安装。Browserslist 数据与构建 chunk 大小为已有提示。

## 自审与复盘

两项需求均有运行证据；无新增契约字段、迁移或依赖，不改调度/计费策略。必要测试覆盖 nil/零/非零价格和实际 handler 读写。
上游合并时已有 DTO 字段也可能漏映射，应核对 Group 类型、mapper 和编辑表单闭环，测试覆盖显式零价。
如用户恢复折叠需求可在 feature 调整 UI；数据映射修复应保留。
