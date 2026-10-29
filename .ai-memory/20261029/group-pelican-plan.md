# 分组定时鹈鹕测试与旧质量模块收纳

- 用户确认：2026-10-29，本地新增配置表/来源字段、全量历史，不执行生产迁移/外呼/部署。仅动画生成，不判分，不改变调度。
- 分支：feature/group-pelican-history，基线 custom-main 1298eb8a6。
- 需求：旧三个页面嵌套在“质量监控”，二改总开关默认保持升级兼容，关闭后台检测、API和调度干预。独立鹈鹕测试不受其影响。
- 配置：分组行的“鹈鹕测试”按钮打开配置；仅 OpenAI，默认关闭，单模型，可修改默认提示词；周期固定30分钟，保存开启后首轮就绪。配置独立接口，避免覆盖完整分组表单。
- 数据：新增 group_pelican_test_schedules，account_pelican_tests 新增来源组ID/名称快照。取消最近3条和30天删除，保留旧结果，历史分页。已有被清理数据无法恢复。
- 执行：PostgreSQL事务领取到期组，原子入队及推进时间；任务队列恢复；共用3并发，数据库领取避免多实例重复。实际生成前复核开启/成员/模型。过期在途任务终结，不自动重放付费调用。
- UI：账号列显示最新时间、模型、耗时、Tokens和历史入口；独立账号历史页采用双列作品墙，预览懒加载/沙箱，详情复用现有对话框。
- 验证：迁移在隔离测试库验证（若可用）；后端相关单元/仓储测试，前端组件测试、lint/typecheck/build，浏览器fixture交互（不调用真实上游）。
- 风险：历史完整保留会增长数据库占用；默认不会新增外呼。开关同步到其他实例至多5秒，在发出下一阶段调用前重读设置。

## 检查点
已完成：3/3 本地主线验收（旧模块收纳/总开关、独立分组定时生成、账号最新列/完整历史作品墙）。后端相关 170 项、最终配置/HTTP 36 项、专项 race 34 项、vet、embed 构建通过；前端相关 130 项、最终历史页2项、lint/typecheck/build通过。wire 已重新生成。
相关文件：migration 249、repository/pelican_schedule_repo.go、service/pelican_schedule_service.go、account_test_service.go、accountquality/{service,runner,http}.go、AccountPelicanHistoryView/GroupPelicanDialog、docs/group-pelican-tests.md。
实测：隔离fixture中配置保存/回读、分页、动画/源码、账号入口、关闭旧菜单与路由拦截、新历史独立访问、390px无横向溢出。临时PGlite执行迁移241/242/249和实际SQL，成员筛选、去重、推进时槽、3任务上限、关闭复核、来源快照通过；不冒充多实例PostgreSQL集成验证。
关键边界：仅直接分组成员；同账号已有任务跳过（跨组也去重）；新定时生成跳过401/429健康写入和调度评分。历史不再自动删，旧已删数据不恢复。
未完成：实现无遗留；真实PostgreSQL多实例并发、真实上游需在测试环境补验。未提交/合并/推送/打tag/部署/生产迁移。
阻塞：无本地实现阻塞；本机无可用PostgreSQL/Docker服务，采用PGlite补充SQL验证，限制已在使用文档明确。

## 本地合并

用户已要求合并分支，授权提交功能并快进合入custom-main；基线与目标均为1298eb8a6，目标无worktree占用。新增文档已加入.gitignore精确白名单。提交前diff检查通过，沿用既有测试。最终提交/包含关系/干净状态以Git操作核验为准；本次不push/tag/部署/生产迁移。
