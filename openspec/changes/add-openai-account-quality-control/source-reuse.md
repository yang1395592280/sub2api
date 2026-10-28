# 开源代码复用与引入记录

版本：v1.3；日期：2026-10-28。用户明确要求参考开源项目能力并搬入代码。已引入完整原始源码、移植监控统计，并接通正式页面/API/质量调度；本页说明来源和复用边界，生产部署与真实效果仍未验收。

## 1. 来源和可复现性

| 项目 | 值 |
| --- | --- |
| 来源仓库 | https://github.com/w1196396546/manxue-ai |
| 固定提交 | 5da79674417dd0748682f09b7b7467efd1968ea8 |
| 原版提交日期 | 2026-09-13 |
| 本地原版 | [third_party/manxue-ai/upstream](../../../third_party/manxue-ai/upstream/README.md) |
| 许可与归属 | [Apache-2.0](../../../third_party/manxue-ai/upstream/LICENSE)、[本地 NOTICE](../../../third_party/manxue-ai/NOTICE) |
| 完整性 | [source-manifest.json](../../../third_party/manxue-ai/source-manifest.json)，17 文件，161,925 字节，逐文件 SHA-256 |

原始文件未改写；不带 .git、运行数据或凭据，不安装其 Python/Chromium 依赖，不挂入现有部署。今后修改/翻译的文件需在文件内注明来源与变更；实际分发适配代码时须随制品附许可/归属，不能只在开发目录保留声明。

## 2. 原版已有能力与复用落点

| 原始实现 | 已存在的具体能力 | sub2api 处理 |
| --- | --- | --- |
| app/web/index.html | 四统计卡、24H 色块时间线、图例、检测频率/下次时间、画廊、历史弹窗 | 作为监控页信息结构来源，移植到 Vue/现有组件风格；不嵌入原站公开页面 |
| app/web/app.js / renderTimeline、showSlot | 按时间槽归类、异常优先着色、点击查看单条或多条历史 | 已移植数据计算到 qualityMonitor.ts，点击交互在后续 Vue 页接入 |
| app/web/app.js / renderState、server.py / Monitor.state | 近 24 小时计数、通过率、最新结果、倒计时 | 已移植基本统计并修正分母；倒计时等待真实后台 next_run_at，不由前端伪造调度 |
| app/web/app.js / showDetail、renderGallery | 结果详情、原始回答、用量、分页/筛选、生成结果预览 | 复用交互组织，走本站管理员 API；历史不公开，不混合不同账号条件 |
| app/web/admin.html、admin.js / renderNodes、showConfig | 节点列表、独立测试、配置回显、周期/超时/Token、自动开关 | 改为内部账号/策略列表；复用本站鉴权，不再复制一套密码或输入上游 Key |
| app/server.py / CANDY_PROMPT、candy_passes | 糖果原题、原版独立数字 21 判分 | 源码已保留；原版判分只作对照，正式自动控制仍用严格最终 JSON |
| app/server.py / call_model、perform_test | Responses/Chat 请求、检测结果、临时错误重试 | 参考协议格式与异常路径；正式执行复用 Go 定向调用，不原样开启付费重试 |
| app/server.py / inspect_svg | SVG 格式、脚本/外链等校验与 nonce | 原版搬入；后续与现有鹈鹕生成预览比较后再适配 |
| app/render_frames.py、visual_review.py | SVG 四帧截图、渲染隔离、视觉检查、JSON 结果解析 | 原版搬入，可作为后续实验，不等于现站 HTML 源码分类器；尚未启用 |
| app/server.py / Monitor、Handler | SQLite 历史、单进程任务、管理员/访客 API | 仅参考流程，不能替换本站 PostgreSQL、集群租约、权限和内部账号路由 |

没有依据的网站能力不列为“已搬入”。相关开源包含 SVG 视觉审核，但未提供已确认的现站 HTML 分类模型/权重。

## 3. 已移植的监控代码

正式适配文件：[qualityMonitor.ts](../../../frontend/src/components/admin/account-quality/qualityMonitor.ts)。测试：[qualityMonitor.spec.ts](../../../frontend/src/components/admin/account-quality/__tests__/qualityMonitor.spec.ts)。

入口 buildQualityMonitor(runs, options)：显式传 policy_id、scope_hash、服务端 now_ms、检测间隔；输出 summary 和 timeline，不访问网络、不读取凭据、不写任何账号或调度状态。

- 继承原版“24H 时隙 → 按结果着色 → 保存该时隙全部记录用于下钻”的逻辑。
- 记录先按策略/证据范围过滤，再按 run_id 去重；避免不同账号、旧路由和分页重复被混算。
- 窗口精确覆盖当前服务端时间向前 24 小时，边界时隙不因对齐整点而丢样本；不接受未来记录或非法时间。
- running/queued 和 error/incorrect/unknown 等分别保留，色块只是监控摘要，不代表质量状态机。
- pass_rate=passed/(passed+incorrect)，无有效答案时 null；answer_coverage=有效答案/全部任务，缺样本时 null。
- 只返回记录 ID 用于后续管理员下钻，不渲染模型 HTML，不执行生成代码；间隔限制 300–86,400 秒，避免无界分配。

原版 Monitor.state 将所有非 running 的糖果结果（含 error）放进 completed，其百分比与本方案“有效答案通过率”不同。之前的现站公开样本观察和这个开源实现也不能混称为同一个统计器。

该模块仍不由 Vue 页面直接调用：它面向完整小数据集且不验证代次/24H 完整性。正式页面使用 Go 服务端聚合接口，避免把分页历史误当全量样本；两者统计口径可用本模块作为回归对照。页面可操作不表示生产已部署或质量控制已启用。

## 4. 需要适配而非原样接入的边界

1. 原版节点保存 API Key；本站选择已有账号 ID 与有效来源上下文，凭据只从现有内部路径取用。
2. 原版独立密码/访客公开结果不接入；本站保持现有管理员权限，拒绝普通用户/受限管理 Key。
3. 原版 SQLite/进程锁不能满足多实例；正式任务按既定 PostgreSQL 事务/租约/预算设计。
4. 原版临时故障重试和双项生成会多次消耗额度；本方案不自动重放受理未知的付费请求。
5. 原版“降智·自动删除”标签不能映射成实际删除本站账号或记录；本站使用可解释的观察、隔离、复测恢复。
6. 检测库存与调度控制只作用于现有自动不降智模式；固定组共享账号不被全局禁用。
7. 原版生成/视觉审核不等于严格质量证据；首期自动控制只使用已评审的确定性题目与判分版本。

## 5. 实施状态与后续对接

| 项目 | 当前状态 |
| --- | --- |
| 固定版本、完整源码、许可、归属、哈希清单 | 已完成 |
| 原版监控/配置/检测代码审阅与复用映射 | 已完成 |
| TypeScript 时间线/监控统计、8 项行为测试 | 已完成，证据见 verification |
| Vue 监控/配置页面、路由、导航和管理员 API | 已接通；结果正文详情与完整事件交互仍待补齐 |
| Go 检测、管理 API、数据库迁移、队列/预算及 HTTP/WS 调度控制 | 核心代码已实现；隔离数据库并发和端到端场景仍待验收 |
| 真实账号检测、部署、启用 | 未执行 |

接下来按 tasks 的 A–D 阶段把现有模块接入页面和真实数据，优先复用本表已有代码，不重建独立 Python 网站。源码搬入授权不等于本轮已经完成新增表/接口/共享调度的全部实施与生产启用。
