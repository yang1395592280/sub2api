# manxue-ai 源码引入

按用户“开源项目里已经实现的可以参考，把代码搬过来”的要求，于 2026-10-28 引入。

- 来源：[w1196396546/manxue-ai](https://github.com/w1196396546/manxue-ai)。
- 固定提交：`5da79674417dd0748682f09b7b7467efd1968ea8`（2026-09-13）。
- 原始代码：[upstream/README.md](./upstream/README.md)；许可：[Apache-2.0](./upstream/LICENSE)；归属：[NOTICE](./NOTICE)。
- 完整性：[source-manifest.json](./source-manifest.json) 列出全部 17 个原始文件的大小与 SHA-256。

## 已搬入的真实能力

| 能力 | 原始代码 | 在 sub2api 中的处理 |
| --- | --- | --- |
| 监控页、统计卡片、24 小时时间线、详情/历史 | upstream/app/web/index.html、app.js、style.css | 保留原版；时间线与监控统计已移植为 TypeScript，供后续 Vue 页接入 |
| 配置页、多节点管理、单次测试、周期/超时/Token 设置 | upstream/app/web/admin.html、admin.js | 保留原版；后续改为选择内部账号，不重复要求输入 Key |
| 糖果题、Responses/Chat 调用、任务与历史 | upstream/app/server.py | 原版完整保留；Go 账号定向调用、严格判分、预算/集群状态按本项目方案适配 |
| SVG 格式/资源校验、随机场景与校验码 | upstream/app/server.py / inspect_svg | 保留原版，作为已有鹈鹕能力的后续扩展参考 |
| 多帧截图与视觉判分 | upstream/app/render_frames.py、visual_review.py | 代码真实存在并已搬入；需要 Chromium 和额外视觉调用，尚未启用 |
| 容器/依赖 | upstream/compose.yaml、app/Dockerfile、requirements.txt | 原版保留用于可复现研究；没有加入 sub2api 部署或构建链 |

## 已移植的模块与重要差异

`frontend/src/components/admin/account-quality/qualityMonitor.ts` 来自原版 renderTimeline/renderState 与 Monitor.state 的监控逻辑，保留时间色块/异常优先/下钻记录设计，改为无 DOM、无请求、无状态写入的 TypeScript 数据模块，后续 Vue 页面直接调用。

适配加入：按 policy_id + scope_hash 隔离、去重、精确滚动 24 小时、运行状态与答案结果分离、无数据返回 null。通过率按 passed/(passed+incorrect)，另提供有效判分覆盖率；原版 Monitor.state 将非 running 的糖果结果（含 error）都计入 completed，不能照抄其分母。

原版 candy_passes 只查独立数字 21，不能作为自动隔离的严格判据。原题文字保留原样用于来源核对；正式控制使用需求稿中明确前提和唯一最终 JSON 的版本。

## 接入状态

本次完成原始源码搬入和首个监控数据模块移植，尚未接通独立 Vue 页面、质量管理 API、PostgreSQL 表或调度控制。自动不降智限定范围继续以需求文档为准。原版的访客输入 Key/公开结果、独立密码认证、SQLite 单实例任务及自动付费重试不直接接入 sub2api。

upstream 原文件保持不变，不复制 .git、数据库、数据目录、凭据、node_modules 或浏览器二进制。本目录不是网站静态发布目录，源文件不会被 sub2api 自动执行。若日后修改原版快照，应另建适配文件并注明变更，不破坏本次校验清单。

详细接入映射见 [source-reuse.md](../../openspec/changes/add-openai-account-quality-control/source-reuse.md)。
