# 网页工作台持久化设计

修改时间：2026-06-21 22:00 CST

## 1. 结论

第一版新增用户侧「网页工作台」，采用 **后端代发 + 会话持久化** 方案。

设计重点：

- 新增用户侧路由 `/workbench`，在侧边栏展示「网页工作台」入口。
- 页面支持对话和生图两种模式，视觉结构参考用户提供的工作台截图。
- 会话和消息保存到数据库，支持跨浏览器和刷新后恢复。
- 前端不直接调用 `/v1/chat/completions` 或 `/v1/images/generations`。
- 前端调用 `/api/v1/workbench/...`，后端校验当前用户、API Key 和会话归属后代发请求。
- 实际模型请求复用现有 OpenAI 兼容网关、计费、分组、额度、渠道和使用记录链路。

## 2. 背景与现状

当前项目已经具备以下基础能力：

- 前端为 Vue 3 + Vite + Tailwind，已有用户侧 `AppLayout`、`AppSidebar`、`AppHeader` 和路由体系。
- 用户侧已有 API Key 管理接口 `/api/v1/keys`，可获取当前用户可用 Key、状态、配额和所属分组。
- 用户侧已有可用渠道接口 `/api/v1/channels/available`，可展示分组、平台和支持模型。
- 后端已有 OpenAI 兼容网关路由，包括 `/v1/chat/completions`、`/v1/images/generations` 和 `/v1/images/edits`。
- 后端使用 ent schema + service + handler + routes 的分层结构，适合新增工作台会话和消息实体。

因此第一版不新建独立 AI 调用链路，而是在用户侧增加工作台体验，并通过后端工作台服务复用已有网关能力。

## 3. 目标

业务目标：

- 用户可以在站内直接使用自己的 API Key 进行对话和生图。
- 用户可以查看历史会话、切换会话、继续对话、删除会话。
- 生图模式支持常用参数配置，并保存生成结果。
- 页面体验接近截图中的双栏工作台，降低用户从创建 Key 到实际使用的门槛。

工程目标：

- 会话数据按用户隔离，禁止访问或操作他人会话。
- API Key 必须属于当前用户且状态可用。
- AI 请求继续经过现有网关，避免绕过计费、额度、分组和风控。
- 第一版保持接口和数据模型清晰，方便后续扩展流式输出、图片编辑、附件和分享。

## 4. 非目标

第一版不做以下内容：

- 不实现多用户共享会话或公开分享链接。
- 不实现团队协作、评论、收藏夹或标签系统。
- 不实现图片编辑接口 `/v1/images/edits`，只预留模式和字段。
- 不实现 WebSocket 或 SSE 流式输出；对话第一版使用非流式响应。
- 不在工作台中创建或修改 API Key，只选择已有 API Key。
- 不替代现有 API Key、使用记录、可用渠道和订阅页面。

## 5. 方案选择

### 5.1 方案 A：前端直连网关，后端只保存历史

优点：

- 实现路径较短。
- 前端可以直接复用 OpenAI 兼容请求格式。

缺点：

- 前端需要持有并组装 API Key 调用细节。
- 请求成功但落库失败、落库成功但请求失败等状态更难统一。
- 权限校验、错误归档和参数快照分散在前端。

结论：不采用。

### 5.2 方案 B：后端代发并持久化

优点：

- 权限校验集中在后端，安全边界清晰。
- 请求、响应、错误和消息落库可以在一个服务流程内完成。
- 更容易复用现有网关、计费、使用记录和错误处理。
- 后续扩展流式输出、审计、重试和敏感字段截断更自然。

缺点：

- 后端需要新增工作台服务、数据表、接口和测试。
- 代发请求需要谨慎处理超时、错误摘要和响应体大小。

结论：采用。

### 5.3 方案 C：新建完整独立 Chat/Image 服务绕过网关

优点：

- 工作台可以完全掌控请求流程。

缺点：

- 容易绕过现有计费、额度、分组和渠道能力。
- 改动面大，和项目已有网关重复。
- 后续维护成本高。

结论：不采用。

## 6. 数据模型

新增 ent schema：`WorkbenchConversation`。

建议表名：`workbench_conversations`。

核心字段：

- `user_id`：会话所属用户。
- `title`：会话标题，默认从首条用户输入截断生成。
- `mode`：`chat` 或 `image`。
- `api_key_id`：最近使用的 API Key ID，可为空。
- `endpoint`：最近使用端点，例如 `chat_completions` 或 `images_generations`。
- `model`：最近使用模型。
- `last_message_preview`：列表展示摘要。
- `last_error`：最近错误摘要，可为空。
- `message_count`：消息数量。
- `created_at`、`updated_at`、`deleted_at`：复用项目时间和软删除 mixin。

索引：

- `user_id`
- `user_id, updated_at`
- `deleted_at`

新增 ent schema：`WorkbenchMessage`。

建议表名：`workbench_messages`。

核心字段：

- `conversation_id`：所属会话。
- `user_id`：冗余用户 ID，便于权限过滤和查询。
- `mode`：`chat` 或 `image`。
- `role`：`user`、`assistant` 或 `system`。
- `content`：文本内容或用户生图提示词。
- `api_key_id`：本次请求使用的 API Key ID，可为空。
- `endpoint`：本次请求端点。
- `model`：本次请求模型。
- `request_options`：JSON，保存尺寸、质量、背景、格式、压缩、张数等参数快照。
- `response_metadata`：JSON，保存 token、图片数量、上游响应 ID、耗时等非敏感摘要。
- `image_outputs`：JSON，保存图片 URL、data URL 或后端返回的可展示引用。
- `status`：`pending`、`success`、`error`。
- `error_message`：截断后的错误摘要。
- `created_at`、`updated_at`、`deleted_at`：复用项目时间和软删除 mixin。

索引：

- `conversation_id, created_at`
- `user_id, created_at`
- `status`
- `deleted_at`

## 7. 后端接口

新增用户接口，挂载在 `/api/v1/workbench`，全部需要 JWT 登录。

### 7.1 会话列表

`GET /api/v1/workbench/conversations`

查询参数：

- `mode`：可选，`chat` 或 `image`。
- `page`、`page_size`：分页。

返回当前用户未删除会话，按 `updated_at` 倒序。

### 7.2 创建会话

`POST /api/v1/workbench/conversations`

请求体：

- `mode`：`chat` 或 `image`，默认 `chat`。
- `title`：可选。
- `api_key_id`：可选。
- `endpoint`：可选。
- `model`：可选。

返回新会话。

### 7.3 消息列表

`GET /api/v1/workbench/conversations/:id/messages`

只返回当前用户自己的会话消息，按 `created_at` 正序。

### 7.4 删除会话

`DELETE /api/v1/workbench/conversations/:id`

软删除当前用户自己的会话和消息。若会话不属于当前用户，返回 404 或 403，保持与项目既有风格一致。

### 7.5 发送请求

`POST /api/v1/workbench/conversations/:id/send`

请求体：

- `mode`：`chat` 或 `image`。
- `api_key_id`：必填。
- `endpoint`：`chat_completions` 或 `images_generations`。
- `model`：必填。
- `input`：用户消息或生图提示词。
- `options`：模式相关参数。

行为：

1. 校验会话属于当前用户。
2. 校验 API Key 属于当前用户、未删除、状态可用。
3. 写入用户消息。
4. 根据 `mode` 构造网关请求。
5. 使用该 API Key 通过现有网关发起请求。
6. 成功时写入助手消息或图片结果。
7. 失败时写入错误消息，并返回可展示错误摘要。
8. 更新会话标题、摘要、模式、模型、API Key 和 `updated_at`。

## 8. 网关调用规则

对话模式：

- 端点：`/v1/chat/completions`
- 请求体包含 `model` 和 `messages`。
- `messages` 由当前会话历史中成功的 `user` 与 `assistant` 文本消息构成，第一版可限制最近 N 条，避免请求体过大。
- 响应优先解析 `choices[0].message.content`。

生图模式：

- 端点：`/v1/images/generations`
- 请求体包含 `model`、`prompt`、`n`、`size`、`quality`、`background`、`output_format`、`output_compression` 等受支持字段。
- 对空值参数不发送，避免兼容上游因未知字段报错。
- 响应解析 `data[]` 中的 `url`、`b64_json` 或项目网关返回的兼容字段。

超时和响应体：

- 使用比普通管理接口更长但有上限的超时，例如 60 秒。
- 错误摘要截断，不保存完整敏感响应。
- 图片 base64 可能较大，第一版如直接保存，需要限制张数和响应大小；若现有网关返回 URL，优先保存 URL。

## 9. 前端页面

新增文件建议：

- `frontend/src/views/user/WorkbenchView.vue`
- `frontend/src/api/workbench.ts`
- `frontend/src/views/user/__tests__/WorkbenchView.spec.ts`

路由：

- `/workbench`
- `requiresAuth: true`
- `titleKey: workbench.title`
- `descriptionKey: workbench.description`

侧边栏：

- 在用户菜单中把「网页工作台」放在「仪表盘」之后或「API 密钥」之前。
- 管理员的个人菜单同样显示该入口，保持用户自测能力一致。

页面布局：

- 左侧会话面板：新建会话、会话列表、删除当前会话。
- 右侧主面板：
  - 顶部配置区：API 端点、API Key/额度、模型、模式切换。
  - 生图参数区：尺寸、比例、质量、背景、格式、压缩、张数、参考图入口预留。
  - 内容区：消息气泡或图片网格。
  - 底部输入区：文本输入和发送/生成按钮。

响应式：

- 桌面端保持截图中的左右布局。
- 窄屏下会话列表折叠到顶部或抽屉，主工作区优先可用。
- 控件固定尺寸，避免发送状态、长模型名或长会话标题导致布局跳动。

## 10. 前端状态与交互

初始加载：

1. 并行加载会话列表、API Key 列表、可用渠道和模型。
2. 若无会话，显示空状态并允许创建新会话。
3. 若有会话，默认选中最近更新会话并加载消息。

发送：

1. 禁用发送按钮并展示 loading 状态。
2. 调用 `/send`。
3. 成功后追加返回消息并刷新会话摘要。
4. 失败后展示错误消息，保留用户输入便于重试。

模型选择：

- 第一版可从 `/channels/available` 聚合模型候选。
- 允许用户手动输入模型名可以作为后续增强，不纳入第一版。

API Key 选择：

- 只展示当前用户 active 的 API Key。
- 展示名称、所属分组、剩余额度或已用额度摘要。
- 若无可用 Key，提示去 API 密钥页创建。

## 11. 权限与安全

- 所有 `/api/v1/workbench` 接口必须使用 JWT 用户身份。
- 查询会话和消息时必须带 `user_id` 条件。
- `api_key_id` 必须属于当前用户。
- 不向前端返回完整未脱敏的敏感错误响应。
- 不记录完整 Authorization 头、API Key 原文或上游敏感错误体。
- 软删除会话不影响使用记录、计费记录和 API Key 本身。

## 12. 错误处理

常见错误：

- 没有可用 API Key：前端提示创建或启用 API Key。
- API Key 不属于当前用户：后端拒绝，前端展示通用错误。
- API Key 配额耗尽或过期：后端返回网关错误摘要，消息状态记为 `error`。
- 模型不可用：提示检查模型和可用渠道。
- 上游超时：保存错误消息，允许用户重试。
- 生图响应过大：返回明确错误，提示减少张数或降低尺寸。

错误消息保存原则：

- 面向用户可理解。
- 截断长度。
- 不包含 API Key、Cookie、Authorization 或完整上游响应体。

## 13. 测试

后端：

- 创建会话会绑定当前用户。
- 用户不能读取、发送或删除他人会话。
- 用户不能使用他人 API Key 发送。
- 对话发送会写入用户消息和助手消息。
- 生图发送会写入用户消息和图片结果。
- 网关失败会写入错误消息并更新会话错误摘要。
- 会话列表按更新时间倒序返回。
- 删除会话后列表和消息接口不再返回。

前端：

- 侧边栏显示「网页工作台」并可跳转。
- 工作台初始加载会话、API Key 和模型候选。
- 可以创建会话、切换会话、删除会话。
- 对话/生图模式切换会展示对应控件。
- 发送中按钮禁用，成功后渲染返回内容。
- 失败后展示错误状态并保留可重试输入。

手工验证：

- 使用真实 active API Key 完成一次对话。
- 使用支持生图的模型完成一次生图。
- 刷新页面后历史会话和消息仍存在。
- 使用另一个用户登录不能看到前一个用户的会话。

## 14. 风险

图片响应体可能很大。第一版需要限制张数、响应大小和保存策略；优先保存 URL，只有必要时保存 base64。

不同上游对生图参数支持不完全一致。第一版应只发送非空参数，并在 UI 中保留默认值，避免不必要的未知字段。

非流式对话会在长响应时等待较久。第一版先保证持久化和稳定性，后续可以在同一 `/send` 语义下扩展 SSE。

会话历史过长会让请求体变大。第一版后端应限制发送给上游的最近消息数量，同时数据库仍保存完整历史。

模型候选来自可用渠道聚合，不一定等于每个 API Key 实际可路由模型。发送失败时需要给出清晰错误，后续可按 API Key 所属分组进一步收窄模型列表。

