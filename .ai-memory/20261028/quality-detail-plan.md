# 糖果详解与鹈鹕判定说明

目标：完整展示版本对应题目、新检测的模型解题说明；区分鹈鹕未评审、评审异常及证据不确定，并展示四项标准及配置入口。

影响链：Condition → ScopeHash / provider prompt / scorer → 保留期内详情 API → 详情组件；配置默认值和旧策略手动升级入口同步。作品墙复用同一判定状态。

边界：不新增 DDL；不自动迁移旧策略、不改生产评审配置、不调用真实上游。旧 v1 仍使用唯一 answer JSON；v2 使用严格 answer + explanation JSON，说明仅供展示，分数仍由整数答案决定。手动升级归档旧策略、重新积累证据，保留原检测方式和开关。新旧证据 ScopeHash 隔离；自动控制范围和阈值不变。

验收：题目、答案、模型说明、原文分层展示；旧记录明确未采集；正文过期/不请求正文不得泄露说明；Markdown 禁止脚本、图片、链接和外部资源；鹈鹕四项全 true 才通过、false 为未通过、null 为不确定，未调用评审时明确提示缺失前置条件。

验证：Go 严格解析/版本隔离/保留期回归；provider 双协议请求回归；Vue 详情/配置升级/状态/Markdown 测试；i18n、ESLint、类型与构建；模拟 API 本地浏览器交互及截图。

进度：分析完成，开始实现。风险为模型未遵循新 JSON 格式会记为 unknown；旧记录无法补出未采集说明，须升级策略后重新检测。回退可继续用旧 v1 策略，无数据库回滚。

## 实现检查点
- 已完成：v2 严格解析、旧v1兼容、版本证据隔离；详情保留期内提取说明；糖果完整题目/Markdown，鹈鹕状态/四标准/配置直达；新建与手动升级入口。
- 证据：核心 accountquality 全包 race92 通过；service双协议新旧版及未配置评审定向race通过；前端28通过；ESLint、vue-tsc、前后端构建通过。真实浏览器已查看题目/解答表格/未评审标准并保存截图。
- 相关文件：scorer.go、http.go、account_quality_{suite,provider}.go、QualityCandyDetail.vue、QualityRunDetailDialog.vue、qualityPresentation.ts、OpenAIQualitySettingsView.vue。
- 未完成：最后UI边界路径（旧记录/过期/不确定）、文档与diff收尾；状态短文案小改后重跑前端相关测试/构建。
- 阻塞：无；未调用真实上游，Docker当前未运行，本轮无DB结构/持久化流程变更，无数据库集成声明。

## 最终验收
- 本地2条需求均完成：糖果详解、鹈鹕标准/状态澄清；不更改视觉判分或自动控制阈值。
- 最终核心race99、service定向race、前端28、i18n/ESLint/vue-tsc/前端构建及普通/最终embed后端构建通过；HTTP包含正文/省略正文/过期正文各覆盖糖果和双检。
- CUA验证新题目/详解表格、未评审、null不确定、旧记录和正文过期，并修正深色对比度。临时验收环境已清理。
- 无代码阻塞；未验证真实付费模型、生产部署和DB集成，未执行Git提交/合并/推送。
