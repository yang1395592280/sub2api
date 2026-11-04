# 固定分组账号故障切换修复计划

日期：2026-11-04。用户已在前轮诊断后明确授权修复。分支：feature/fixed-group-account-failover。

## 验收
- OpenAI 固定分组、池模式、未输出正文且可迁移：同组健康B优先于A的原地重试；全部调度检查复用现有选号入口。
- 没有可用B时继续原有有界池模式重试；请求级容量错误、必须原账号延续、非OpenAI路径保留原语义。
- HTTP200流内结构化upstream_error + 已知网关forbidden安全文案触发有界切换，但不据此禁用账号；普通permission/content/safety错误不扩大识别。
- 切换次数、失败账号排除、固定组权限及倍率、并发槽释放保持正确；Responses/Chat/Messages/Images/AlphaSearch/WS共享同一候选预取帮助方法。

## 实现与验证
1. 新增真实handler模拟上游回归测试，先在旧实现确认HTTP403和SSE403无法达到A→B的失败证据。
2. handler新增保存一次替代选号结果的帮助方法，先尝试排除A的真实调度，成功直接复用结果；失败不改变原排除集合并继续旧重试。各入口只接入闭包和有界预选，不另造调度过滤。
3. service仅新增受限的网关错误识别，将可换号与账号失效副作用分离。
4. 最小回归→相关handler/service/accountquality测试→必要的全包验证；运行gofmt、git diff --check，审查所有变更与资源生命周期。

## 影响与风险
- 无数据库/接口契约/生产配置/远程写入。只有失败路径增加一次真实选号；成功预选不重复选号、不重复抢槽。
- 预选结果在取消/重试预算耗尽等提前结束时必须释放；未选中时不得清空已失败账号或改变其他分组权限。
- 图谱为可选且未配置，本轮以rg调用链复核代替：6个handler入口、流内error/response.failed与WS副作用调用。
- 回退：撤回本分支的相关局部补丁；不改main/custom-main，不覆盖用户改动。

## 规划门禁
A1–A10：用户目标、退出条件、依赖、风险、验证、回退、文件范围、权限边界、干净工作区、失败回到复现均已检查。
B1/B2/B7/B8：N/A（无DDL/新依赖/数据批量写/不可逆操作）；B3/B4/B5/B6/B9/B10/B11：并发槽/故障路径成本/协议兼容/回归测试/rg依赖/命令参数/回读检查。
C1/C2：多文件同构接入，每5文件检查diff；阶段边界更新本记录。

## 当前状态
已完成诊断及旧实现 Red 回归（/tmp/sub2api-fixed-group-red.log）。真实 handler 回归已验证 Responses/Chat/Messages 的 HTTP403、流内 response.failed/error、透传开关、流式/非流式与 EOF 无空行等60个组合，均按 A→B 切换；正文输出后不重放、策略拒绝不换号、单账号保留池重试均通过。

实现：六个入口复用现有调度闭包预选并保存一次选号结果；共享识别仅扩展已知网关 upstream_error，保持凭据失效副作用原边界。非流式兼容转换保留原始终止事件，防止错误 type/status 等字段丢失。

兼容验证：同组两账号旧用例按新策略更新为 A→B；自动分组与 helper 资源释放/预算/绑定/容量边界定向测试通过。handler 与 apicompat 全包通过；service 全包及新增 images/alpha-search 真实入口与 race 检查运行中。当前无失败；最终结果后续追加，不推送/合并/部署。

## 最终结果
- 本地修复完成；无实现遗留。`go test -json -tags=unit ./internal/handler ./internal/service ./internal/pkg/apicompat` 退出0，三个包全部通过；11项测试按既有环境条件跳过，未调用真实供应商/生产数据库。日志：/tmp/sub2api-fixed-group-full.jsonl。
- 补充的完整正向矩阵60场景、images/alpha-search、单账号回退、正文后不重放、内容策略拒绝及预选资源/取消/绑定/预算边界：`go test -race -tags=unit ./internal/handler -run '^(TestOpenAIFixedGroupFailover_|TestOpenAIFailoverSelection_)' -count=1` 退出0（2.966s），无竞态报告。日志：/tmp/sub2api-fixed-group-race-verified.log。
- 附加入口测试最初缺少计费存储模拟；已补充UsageBillingRepository桩，保留normal模式真实分组过滤，不用simple模式绕开权限。修正后的全部定向race测试通过；生产实现未因此调整。
- `gofmt`、`git diff --check`通过；回读六入口与共享识别、原错误副作用、资源移交/提前结束释放调用链。无数据库、外部契约、依赖或前端变更。
- 分支feature/fixed-group-account-failover，仅本地改动，未提交/合并/推送/部署；上线需要后续发布操作。真实故障request_id未提供，未宣称生产复现或验证。

## 2026-11-04 13:58 合并授权
用户明确要求合并分支。当前修复与记忆记录将提交到feature/fixed-group-account-failover，再以merge --ff-only合入custom-main；二者基线b22f8bd40一致。此前测试证据有效，本轮不改生产实现。最终提交点、合并状态和干净工作区以Git核验为准，不推送/tag/部署。
