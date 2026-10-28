# v0.2.18.5 发布记录

- 授权：用户要求打 tag v0.2.18.5、提交并推送。
- 功能提交：63400c61f 已通过 Fast-forward 合入 custom-main，工作区起始干净。新增糖果与鹈鹕动画双重检测、自动四帧视觉评审、作品墙/色块/分项详情、评审配置、旧策略升级及逐阶段预算；保留普通组仅观测的边界。
- 版本：backend/cmd/server/VERSION 更新为 0.2.18.5；嵌入前端的 Go 构建、二进制 -version 与 resolve-version.sh 均确认新版本。
- 验证：9项非镜像发布工具测试、两个发布脚本语法检查及 diff 检查通过。沿用本次功能 Go核心race74/服务定向race、隔离PG23、前端24、Chromium真实渲染3、lint/type/build及浏览器验收证据。系统Python缺PyYAML，改用隔离venv按已有requirements安装后测试通过。
- 发布范围：提交版本与记录，创建 annotated tag，原子推送 origin/custom-main 与 refs/tags/v0.2.18.5。未改旧 tag 或其他本地分支。
- 工作流：已确认 RELEASE_ONLY=true，现有 tag 工作流生成 GitHub Release 与安装包，跳过镜像发布和Telegram通知；不手动修改 main。
- 限制：正式启用双检测前需部署私网渲染器并配置支持图片的评审模型，每次完整检测至多3次上游调用。真实上游（含WS多模态）尚未联调，渲染器Docker镜像构建因官方镜像下载缓慢中止。本次不执行生产部署、迁移或启用；未运行两项依赖新版Bash的镜像脚本测试，仅做语法检查。
- 本条为推送前检查点，最终提交、远端refs及Release工作流状态以后续Git/GitHub核验为准。
