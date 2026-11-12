# v0.2.21.2 发布记录

- 授权：用户明确要求打 tag v0.2.21.2、提交并推送。
- 起始状态：custom-main=adb011cf8，工作区干净；该提交已 fast-forward 合入签到月历与翻倍动画；远端 custom-main=31572b838，实时查询无 v0.2.21.2 标签或同名 Release。
- 版本：backend/cmd/server/VERSION 从 0.2.21.1 更新至 0.2.21.2。
- 内容：恢复参考项目的签到月历视图，按日期显示奖励和负收益；增加签到统计、月份切换、翻倍结果卡片与翻牌动画；保留验证码、风险暂停、福利负数和中英文翻译。
- 本轮验证：resolve-version.sh 输出 0.2.21.2，sh 语法检查通过；签到组件与 i18n 测试 6 项通过；typecheck、lint 和前端生产 build 通过；mock 浏览器页面确认 42 格月历、签到记录与翻倍按钮正常渲染。
- 推送计划：发布提交后创建 annotated tag v0.2.21.2，以 git push --atomic 仅推送 origin/custom-main 与 refs/tags/v0.2.21.2，不 force。
- 工作流：RELEASE_ONLY=true；tag 触发现有归档、checksums 与 GitHub Release 工作流，跳过 Docker/GHCR 镜像发布；本任务不部署、不执行生产迁移。
- 上线事项：已有迁移 251、252 和福利分账上线要求仍适用，部署前请备份并按 docs/welfare-balance-checkin.md 执行。
- 本记录为推送前检查点；最终提交、远端分支/tag 及自动发布结果以 Git/GitHub 核验为准。
