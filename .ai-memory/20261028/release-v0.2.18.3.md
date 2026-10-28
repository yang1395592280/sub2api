# v0.2.18.3 发布记录

- 授权：用户要求提交、打 v0.2.18.3 tag 并推送。
- 改动：修复质量监控与配置导航同时高亮；补齐监控按钮样式；配置改为先选来源组再选组内账号，支持每个账号创建两个模型的独立策略，每批最多 100 条策略。
- 版本：backend/cmd/server/VERSION 更新为 0.2.18.3。
- 分支：feature/openai-quality-ui-fixes 提交后合入 custom-main，创建 annotated tag，原有 tag 保持不变。
- 验证：前序已通过 vue-tsc、定向 ESLint、Vite 生产构建及 diff 空白检查；本次补跑 check:i18n，3 项通过。
- 限制：前序浏览器确认表单结构；本地后端未运行，完整组/账号选择与策略保存流程未验收。
- 推送计划：原子推送 origin/custom-main 和 refs/tags/v0.2.18.3；tag 触发现有 Release 工作流。本次不部署、不执行数据库迁移、不发起真实账号检测。
- 此记录为推送前状态，最终提交及远端 refs 以后续核验为准。
