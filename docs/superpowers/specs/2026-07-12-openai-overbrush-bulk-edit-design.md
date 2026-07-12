# OpenAI OAuth 超刷批量编辑设计

日期：2026-07-12

## 背景

账号管理的单账号编辑弹窗已经支持为 OpenAI OAuth 账号设置“是否超刷”，但批量编辑弹窗没有对应入口。管理员批量选择 OpenAI OAuth 账号后，只能逐个进入编辑弹窗修改，操作成本较高。

## 目标

- 在账号批量编辑弹窗中增加“是否超刷”配置。
- 沿用单账号编辑的适用范围，仅对 OpenAI OAuth 账号开放。
- 支持批量开启和批量关闭超刷。
- 未选择修改超刷时，不改变目标账号的现有配置。
- 超刷批量更新不得修改账号的 `schedulable` 字段。

## 非目标

- 不处理开启超刷后账号调度状态变化的问题。
- 不修改超刷运行时行为、连续 429 阈值或设置页面。
- 不支持 OpenAI API Key、setup-token 或其他平台账号。
- 不新增后端接口或数据库字段。

## 交互设计

批量编辑弹窗沿用现有字段的“修改复选框 + 值控件”模式：

- 仅当本次目标账号全部满足 `platform === 'openai'` 且 `type === 'oauth'` 时展示“是否超刷”。
- 外层复选框表示本次批量操作是否修改超刷配置，默认不勾选。
- 内层开关表示目标值，默认关闭；外层复选框未勾选时，内层开关不可操作并降低视觉权重。
- 文案复用单账号编辑已有的 `admin.accounts.openai.overbrush` 和 `admin.accounts.openai.overbrushDesc`，不新增重复翻译。

混合平台、混合账号类型、筛选结果中包含非 OpenAI OAuth 账号时不展示该配置，避免把无效字段写入不支持超刷的账号。

## 数据流

批量弹窗继续调用现有 `POST /api/v1/admin/accounts/bulk-update` 接口：

- 未勾选外层复选框：payload 不包含 `openai_overbrush_enabled`。
- 勾选并开启内层开关：发送 `extra.openai_overbrush_enabled: true`。
- 勾选并关闭内层开关：发送 `extra.openai_overbrush_enabled: false`。

示例：

```json
{
  "account_ids": [1, 2],
  "extra": {
    "openai_overbrush_enabled": true
  }
}
```

批量更新后端已经对 `extra` 使用 JSONB 顶层合并语义，因此 `false` 可以覆盖旧的 `true`，且不会清除其他 `extra` 字段。请求不包含 `schedulable`，后端的指针字段保持 `nil`，账号调度开关不会被本功能主动修改。

## 代码范围

- `frontend/src/components/account/BulkEditAccountModal.vue`
  - 增加资格判断、外层修改标志、内层目标值和界面控件。
  - 在 `buildUpdatePayload` 中仅按外层修改标志写入 `extra.openai_overbrush_enabled`。
- `frontend/src/components/account/__tests__/BulkEditAccountModal.spec.ts`
  - 增加显示条件和 payload 回归测试。

## 测试计划

- 全部选中 OpenAI OAuth 账号时显示超刷批量编辑项。
- OpenAI API Key、setup-token、其他平台及混合目标不显示该项。
- 勾选修改并开启时，仅提交 `extra.openai_overbrush_enabled: true`。
- 勾选修改并关闭时，仅提交 `extra.openai_overbrush_enabled: false`。
- 未勾选修改时，不提交超刷字段；若没有其他修改，不调用批量更新接口。
- 两种提交场景都断言 payload 不包含 `schedulable`。

## 风险

- 筛选结果模式依赖调用方提供的 `selectedPlatforms` 和 `selectedTypes` 摘要；现有批量编辑的其他平台专属配置使用相同机制，本次保持一致。
- 批量接口的 `extra` 是合并而非删除语义，因此关闭时保留显式 `false`。运行时判断只认严格的 `true`，行为与删除键等价。
