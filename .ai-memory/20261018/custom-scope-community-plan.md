# 集中页收窄与社群入口修复

状态：执行中。分支 feature/retain-custom-feature-settings；继续此前未提交的监控撤回，不重置用户文件。

## 范围与验收
1. 按用户确认，集中页及 GET/PATCH 仅收录 zenxiang_liyu_enabled、redeem_code_purchase_enabled、join_group_enabled 和自定义菜单；其余15项恢复原设置页可编辑与保存，不改存量设置值。
2. 社群入口：开关开启且有链接/二维码时，标准首页、简洁首页与后台头部均显示；关闭或无目标时不显示。集中页补配置提示及进入通用设置的按钮。全屏自定义首页维持管理员内容语义。
3. 定向 Go/前端回归、typecheck、ESLint、构建、实际浏览器本地模拟配置验证；不提交/推送/部署、不改数据库结构。

## 假设与证据
- H1：简洁首页漏挂入口（高）。HomeView.vue 简洁分支只有 LocaleSwitcher/文档/模型广场，无 JoinGroupEntryButton；默认分支及 AppHeader 均已挂载。先新增回归复现。
- H2：启用但未配置目标（中）。JoinGroupEntryButton.vue 明确要求 URL 或图片非空；补配置指引，保持空目标隐藏行为。
- 保存链路：集中 PATCH 使用原 join_group_enabled，刷新公共设置；后端/组件测试覆盖已有链接和图片不被改写。未访问生产配置，无法断言线上具体原因。

## 规划门禁
A1-A10：需求/步骤/出口/字段/风险/验证已列；范围12个代码、测试、说明文件；Git及逐文件任务前快照可恢复；无不明人工改动，已有差异为本任务前序工作。失败先定位对应测试，不覆盖整份文件。
B1/B2/B4/B5/B8：N/A，无DDL、新依赖、性能算法、外部系统写入或不可逆操作。B3：复用现有单次保存与缓存刷新。B6：测试见验收。B7/B9/B11：分批最小修改并diff/grep回读，图谱不可用以rg逐引用核对。B10：命令不更改生产状态。C1/C2：分批检查点记录至handoff。

## 诊断
问题是简洁首页缺失入口组件；证据为 HomeView.vue 两个首页分支不对称；下一步先添加失败测试，再复用现有 JoinGroupEntryButton。

失败计数：0；预期Red不计失败。
