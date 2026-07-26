# OpenAI 自建号池分组开发方案

日期：2026-07-26
状态：开发完成，本地自动化验证通过；管理员页面浏览器联调待可用后端环境
关联需求：[OpenAI 自建号池分组需求规格](../specs/2026-07-26-openai-self-hosted-account-pool-requirements.md)

## 1. 目标与核心结论

新增仅供管理员维护账号的 OpenAI 自建号池分组。普通 OpenAI 分组最多绑定一个号池，同一个号池允许被多个普通分组绑定。请求选中普通分组后，统一按“已启用号池优先、号池耗尽后本组兜底”选号；自动最优惠在每个价格分组内部执行相同流程。

运行时必须拆分两个概念：

- `EffectiveGroupID`：有效普通分组，负责权限、倍率、限额、RPM、模型规则、价格保护、粘性命名空间、usage 和审计业务归属。
- `AccountSourceGroupID`：账号来源分组，仅负责候选账号成员范围和成员优先级。

任何情况下都不得用号池替换有效分组的计费和业务上下文。

## 2. 数据模型

### 2.1 groups 新字段

迁移文件：`backend/migrations/191_openai_self_hosted_account_pool.sql`

```sql
ALTER TABLE groups
  ADD COLUMN IF NOT EXISTS group_role VARCHAR(32) NOT NULL DEFAULT 'standard',
  ADD COLUMN IF NOT EXISTS self_hosted_pool_group_id BIGINT;

ALTER TABLE groups
  ADD CONSTRAINT fk_groups_self_hosted_pool
  FOREIGN KEY (self_hosted_pool_group_id) REFERENCES groups(id) ON DELETE RESTRICT;

CREATE INDEX IF NOT EXISTS idx_groups_self_hosted_pool_group_id
  ON groups(self_hosted_pool_group_id)
  WHERE deleted_at IS NULL AND self_hosted_pool_group_id IS NOT NULL;

ALTER TABLE groups
  ADD CONSTRAINT chk_groups_group_role
  CHECK (group_role IN ('standard', 'self_hosted_pool'));
```

约束说明：

- 不对 `self_hosted_pool_group_id` 建唯一索引，以支持 `0..N` 个普通分组共享同一号池。
- 数据库外键保证引用目标存在并阻止删除被引用号池；角色、平台、自关联和嵌套规则由 Service 校验。
- 存量行自动成为 `standard`，关联为空，未配置时行为不变。
- Ent Schema 只增加普通字段，不定义自关联 edge，避免把多对一关系误生成唯一关系。

### 2.2 服务实体与 DTO

`service.Group`、管理端 DTO 和前端类型增加：

- `group_role`
- `self_hosted_pool_group_id`
- `self_hosted_pool_group_name`
- `self_hosted_pool_group_status`
- `referenced_group_count`（号池列表展示）

常量：

- `standard`
- `self_hosted_pool`

## 3. 后端管理规则

### 3.1 创建与更新

管理服务集中执行以下校验：

1. 旧请求未传 `group_role` 时创建普通分组。
2. 只有 `platform=openai` 可设置 `self_hosted_pool`。
3. 号池不得再绑定号池；创建/更新号池时强制清空关联字段。
4. 只有普通 OpenAI 分组可绑定号池。
5. 关联目标必须存在、平台为 OpenAI、角色为 `self_hosted_pool`，且不能是自身。
6. 更新请求未传关联字段时保留原值；显式 `null` 表示解除关联。
7. 号池只允许绑定 OpenAI 账号；账号创建、更新、批量绑定和分组成员操作均执行服务端校验。
8. 被引用号池删除时返回可识别的业务错误，管理员需先解除关联。

号池自身无效的售卖和计费字段在保存时归一化为安全值，并禁止 API Key、用户允许分组、订阅及兑换码直接引用号池。

### 3.2 查询与缓存

- 管理端列表保留号池并返回关联摘要。
- 用户侧可用分组、API Key/订阅/兑换码目标分组及自动最优惠候选统一排除号池。
- 分组角色、关联、状态或成员变更复用现有 scheduler/auth cache outbox 失效机制。
- 读取关联失败时记录结构化告警并降级到有效分组自身。

## 4. 统一调度设计

### 4.1 调度请求

扩展 `OpenAIAccountScheduleRequest`：

```go
type OpenAIAccountScheduleRequest struct {
    EffectiveGroupID     *int64
    AccountSourceGroupID *int64
    // 其余字段保持不变
}
```

兼容策略：现有调用仍可传 `GroupID`，归一化时将其同时视为有效分组和账号来源分组；新增号池流程显式传两个 ID。后续内部逻辑按下表选取：

| 调度行为 | 使用字段 |
| --- | --- |
| 候选账号查询、成员复核、成员 priority | `AccountSourceGroupID` |
| 自动调度分组开关、价格保护、渠道限制 | `EffectiveGroupID` |
| sticky 读取、写入、删除、刷新 | `EffectiveGroupID` |
| 调度健康、审计和事件的业务分组 | `EffectiveGroupID` |

账号即使不属于有效分组，只要属于来源号池且通过所有现有硬过滤，即可被选中。物理账号仍按 ID 使用同一排除集合，保证号池和本组重复成员不会在同一次请求立即重试。

### 4.2 两阶段选择器

新增集中辅助流程，根据有效分组解析已启用的号池：

```text
select(effectiveGroup):
  pool = effectiveGroup.self_hosted_pool_group_id
  if pool 存在、角色正确、OpenAI 且 active:
      使用 effectiveGroup 业务上下文 + pool 成员范围选号
      成功则返回
  使用 effectiveGroup 业务上下文 + effectiveGroup 成员范围选号
```

- 固定分组、普通 load-awareness 路径、Images 路径和统一高级调度路径均复用该流程。
- 强亲和保持原行为：sticky 键仍属于有效分组，但复核账号成员时使用当前来源阶段；失效后按正常阶段继续选择。
- 自动最优惠先按有效倍率排序普通分组，再对每个分组执行两阶段选择。
- 质量优先与可用性兜底在每个来源阶段内复用现有策略；严格质量重选仍禁止低置信兜底。
- 号池关闭、删除、异常或无候选只表示号池阶段耗尽，不将有效分组标记耗尽；只有本组阶段也耗尽后才进入下一个价格分组。

### 4.3 可观测性

在调度决策结构和结构化日志中增加：

- `effective_group_id`
- `account_source_group_id`
- `account_source_type`
- `pool_group_id`
- `pool_fallback_reason`

现有 `group_id` 保持有效分组语义，避免破坏统计和兼容调用。本期不修改 `usage_logs` 表；管理员调度决策审计保存来源字段，普通 usage 继续只记录有效分组。

## 5. 管理端设计

### 5.1 分组表单

- OpenAI 分组增加角色选择：普通分组 / 自建号池。
- 普通 OpenAI 分组显示“优先自建号池”下拉框，选项包含全部 OpenAI 号池，不因已被其他分组选择而隐藏。
- 允许选择关闭号池，并展示“关闭状态下不会参与调度”。
- 号池表单隐藏计费、订阅、用户权限、自动最优惠等无效配置，仅保留名称、描述、状态和账号成员管理。

### 5.2 列表与操作

- 角色列区分普通分组和自建号池。
- 普通分组显示关联号池；号池显示被引用分组数和账号可用情况。
- 关闭号池沿用现有状态操作，并在确认文案中说明关联分组将直接使用自身账号。
- 删除被引用号池时展示后端返回的明确错误。

## 6. 接口兼容

- 管理端 create/update 请求增加可选字段；旧客户端不传字段时行为不变。
- 更新接口对关联字段采用可选可空语义：缺失为“不修改”，`null` 为“解除”。
- API 响应只做加法变更。
- 用户侧接口继续返回相同结构，但过滤号池。

## 7. 实施任务

### Task 1：存储与实体

- [x] 增加迁移、Ent 字段和迁移契约测试。
- [x] 生成 Ent 代码。
- [x] 扩展 `service.Group`、仓储创建/更新/映射和列表摘要。

### Task 2：管理端契约与业务校验

- [x] 扩展管理服务输入、DTO 和 Handler 映射。
- [x] 实现角色/关联/删除/成员平台校验。
- [x] 排除所有用户侧直接使用号池的入口。
- [x] 补充管理服务与仓储测试。

### Task 3：调度上下文拆分

- [x] 扩展调度请求并兼容旧 `GroupID`。
- [x] 成员范围与成员优先级使用来源分组。
- [x] sticky、门控、价格保护、渠道限制和审计使用有效分组。
- [x] 补充底层调度器双分组测试。

### Task 4：固定分组和自动最优惠两阶段调度

- [x] 实现统一来源阶段解析与选择辅助函数。
- [x] 接入 load-awareness、Capability、Images 和模型解析路径。
- [x] 接入 Responses、WebSocket、Chat、Messages、Embeddings、Images、Live、Count Tokens 和 Alpha Search 入口。
- [x] 自动最优惠候选排除号池。
- [x] 验证 `A -> P(B) -> B -> P(C) -> C`、关闭跳过、共享号池和重复账号排除。

### Task 5：管理端界面

- [x] 扩展前端 Group 类型和 create/update 请求。
- [x] 增加角色、号池绑定和列表摘要展示。
- [x] 号池表单隐藏无效业务配置并增加启停提示。
- [x] 补充前端组件测试和中英文文案。

### Task 6：验证与收尾

- [x] 运行 Ent、migration、repository、service 相关 Go 测试。
- [x] 运行前端类型检查、相关组件测试和全量 Vitest。
- [ ] 启动管理端并用桌面/移动视口验证创建、绑定、共享绑定、关闭和列表展示。当前 Vite 已启动，但本地无后端且浏览器会话不是管理员，访问管理页面被权限守卫重定向；需在可用联调环境补验。
- [x] 更新需求文档实施状态与本开发文档检查项。

## 7.1 实际验证结果

- Go：迁移、Service、Repository、DTO、管理 Handler、网关 Handler 相关测试通过；`internal/service` 全包重跑通过。
- 前端：`pnpm typecheck` 通过；Vitest 240 个测试文件、1548 个测试全部通过。
- 浏览器：Vite 在 `http://127.0.0.1:5173/` 正常启动；因 API 后端不可达且会话无管理员权限，未完成管理员分组页的视觉和真实接口联调。

## 8. 测试矩阵

| 场景 | 预期 |
| --- | --- |
| 存量普通分组 | 角色为 `standard`，无关联，行为不变 |
| B、C 同时绑定 P | 两条关联均保存成功，解除 B 不影响 C |
| 固定 B，P 可用 | 只选择 P 账号，业务分组仍为 B |
| 固定 B，P 耗尽 | 选择 B 自身账号 |
| P 关闭 | 不查询/不获取 P 的并发槽，直接选择 B |
| 自动最优惠 A/B/C | 严格执行 `A -> P(B) -> B -> P(C) -> C` |
| P 与 B 重复账号 | 已失败物理账号不立即重试 |
| B/C 倍率不同且都命中 P | 分别使用 B/C 的有效倍率与 usage 归属 |
| 最大倍率 0.15 | 不进入 C 及其号池阶段 |
| 非 OpenAI 账号加入 P | 服务端拒绝 |
| API Key/订阅指向 P | 服务端拒绝，用户选择器不展示 P |
| 被引用 P 删除 | 删除失败并提示先解除关联 |

## 9. 风险、灰度与回滚

主要风险是把账号来源分组误用于计费/价格保护，或只接入部分 OpenAI 请求入口。通过统一选号入口、双分组单测和 usage 归属回归降低风险。

灰度顺序：先上线存储和配置，再对单个固定分组绑定，最后进入自动最优惠候选。重点观察号池命中、来源降级、错误率、首字延迟和最终有效分组倍率。

运行时回滚无需删除字段：关闭号池或解除关联即可恢复普通分组原选号。数据库迁移为加法变更；如必须结构回滚，应先清空关联，再删除外键、索引和两个字段。历史 usage 与审计不回写。
