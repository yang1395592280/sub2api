# v0.2.18.4 发布记录

- 授权：用户要求打 tag v0.2.18.4、提交并推送。
- 功能提交：adeddb926 已通过 Fast-forward 合入 custom-main，工作区起始干净。新增检测记录页面与入口，改进监控/配置布局和组名展示，允许普通 OpenAI 来源组检测，调度控制仍限自动不降智来源组。
- 版本：backend/cmd/server/VERSION 更新为 0.2.18.4；本地嵌入前端的 Go 二进制 -version 和 resolve-version.sh 均输出 0.2.18.4。
- 验证：本轮 9 项非镜像发布工具测试、release-images.sh 与 resolve-version.sh 语法检查、diff 检查通过。沿用本次功能 Go 定向 27 项、前端 18 项、ESLint、类型检查、生产构建及模拟 API 浏览器验收证据。
- 发布范围：创建 annotated tag，并原子推送 origin/custom-main 与 refs/tags/v0.2.18.4；未修改旧 tag 或其他本地分支。
- 工作流：已通过 GitHub API 确认 RELEASE_ONLY=true；tag 触发现有 Release 工作流，跳过镜像推送与 Telegram 通知。默认分支 main 的 VERSION 同步由既有工作流管理，本轮不手动修改其引用。
- 限制：没有执行真实数据库/Redis/上游集成；未执行生产迁移、部署或启用配置。未执行两项依赖新版 Bash 的镜像脚本测试，仅做脚本语法检查。
- 此文件为推送前检查点，最终提交、远端 refs 与 Release 工作流状态以后续核验为准。
