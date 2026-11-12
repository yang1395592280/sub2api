# v0.2.21.3 发布记录

- 授权：用户明确要求打 tag v0.2.21.3、提交并推送。
- 起始状态：custom-main=aef9e009f，工作区干净；退款展示与福利计费安全修复已 fast-forward 合入，无冲突。远端 custom-main=abd49c9dbcfe5d6d160534e09326af3fb99f45f6，实时查询无 v0.2.21.3 标签或同名 Release。
- 版本：backend/cmd/server/VERSION 从 0.2.21.2 更新为 0.2.21.3；本轮无新增业务代码。
- 内容：福利净额非正时全扣充值，正净额受比例及充值匹配上限限制；后台可配置消费抵扣比例，默认2:1，福利份额0关闭；图片冻结保存原比例快照；签到令牌拒绝绕过/重放、金额与翻倍校验、礼遇安全随机；管理员选择福利来源退款、余额分栏与两位金额显示。
- 本轮验证：sh语法检查和resolve-version.sh输出0.2.21.3通过；隔离Linux中11项release_matrix发布工具测试全部通过；CGO_ENABLED=0 go build -tags embed -trimpath -ldflags '-s -w'通过，实际二进制-version显示Sub2API 0.2.21.3。macOS系统Bash3不支持发布脚本大小写扩展，使用Linux复验解决环境差异，未修改发布脚本。
- 业务验证：复用aef9e009f交付前Go全仓unit、相关四包race、build/vet、真实PostgreSQL18.1/Redis8.4的15项钱包集成；前端434文件3448项、lint/build含类型检查、正式构建模拟API浏览器中英/桌面390px。详见wallet-security-policy-plan.md。
- 推送计划：创建发布提交及annotated tag，以git push --atomic仅推送origin/custom-main与refs/tags/v0.2.21.3，不force。
- 工作流：实时核对RELEASE_ONLY=true；tag触发现有多平台归档、checksums和GitHub Release，跳过Docker/GHCR发布。本任务不部署、不操作生产数据、不修改工作流或默认分支；现有sync-version-file成功后会同步默认分支main的版本文件。
- 上线：本次相对v0.2.21.2无新增DDL。同步前后端并排空旧写入实例，核对净福利非正全扣充值、可配比例及旧冻结结算；回退前恢复2:1并排空非默认比例冻结，刷新余额和鉴权缓存，保留所有账务流水，详见docs/welfare-balance-checkin.md。
- 本记录为推送前检查点；最终提交、远端分支/tag和自动Release状态以Git/GitHub核验为准。
