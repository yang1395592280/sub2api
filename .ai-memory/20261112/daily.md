session-id: 20261112-0849
# 工作日志 - 2026-11-12

## [08:49] 修复: v0.2.21 合并后回归
- 补齐7组账号页mock；安装锁定v11依赖解决旧node_modules翻译异常，Vite配置未改。WS缓存TTL丢失经真实入口复现并修复，定向回归通过，正在全量/race验证。

## [08:58] 验证: TTL两处修复闭环
- WebSocket真实多轮与Chat/Messages四组合通过，Red复核可捕获字段丢失；定向race通过。前端428文件3406测试通过，lint/build及真实浏览器中英/CSP/手机宽检查通过。后端全量最终运行中，build/vet通过。

## [09:01] 完成: 合并后缺陷排查与修复
- 两处TTL丢失已修复，后端全量/build/vet和定向race通过；前端3406全量、619关键、lint/typecheck/build通过；浏览器中英/CSP/手机验证通过。Docker集成跳过，修复未提交、未合并、未推送部署。

## 后续交付: 用户授权合并分支
- 提交本轮已验证修复并以fast-forward方式合入custom-main；最终提交点以Git历史为准。不推送、打tag或部署。

## 发布: v0.2.21
- 用户授权tag、提交与推送；VERSION已为0.2.21。发布构建与二进制版本检查通过，复用已通过的业务验证。先保留本地上游同名tag，再创建二改发布tag并原子推送custom-main和v0.2.21。RELEASE_ONLY=true；结果以远端核验为准，不部署。详情release-v0.2.21.md。

## [13:21] 完成: 签到迁移与福利分账
- 充值原值保留，签到/礼遇/赠送来源分账；9=6+3，负福利不计费、后续同来源奖励抵欠；风控72小时清退及恢复；前后台分项、消费明细与导出完成。见 docs/welfare-balance-checkin.md。
- 后端全量 unit/build/vet、相关 race 与真实数据库集成通过；前端429文件3409测试、lint/typecheck/build通过。真实浏览器验证签到负数、赠送、权限、风控到期与恢复、桌面/手机正式构建中英及CSP。未提交/合并/推送/部署。

## [14:03] 合并授权: 签到与福利分账
- 用户要求将 feature/welfare-balance-checkin 合入 custom-main，授权本地提交及合并；两分支起点均为6d9e8231e，可 fast-forward。沿用已通过的全量/集成/浏览器验证，不推送或部署。具体结果以 Git 历史为准。

## [14:10] 发布准备: v0.2.21.1
- 签到与福利分账已提交为90cd5c71e并无冲突合入custom-main。用户授权新tag、提交与推送；VERSION更新为0.2.21.1，发布工具11项与embed构建/实际版本检查通过。
- 将创建annotated tag并原子推送custom-main和v0.2.21.1，结果以远端核验为准。RELEASE_ONLY=true，沿用现有自动Release工作流，不部署或执行生产迁移。详情release-v0.2.21.1.md。

## [15:12] 发布准备: v0.2.21.2
- 用户授权提交、打tag并推送签到月历与翻倍动画修复；custom-main当前提交adb011cf8，VERSION更新为0.2.21.2。
- 将创建annotated tag并原子推送custom-main和v0.2.21.2；RELEASE_ONLY=true，沿用现有自动Release工作流，不部署或执行生产迁移。详情release-v0.2.21.2.md。

## [16:38] 完成：福利退款与余额显示修复
- feature/refund-welfare-display：管理员可选签到/礼遇/赠送来源退款，可单退充值/福利或混合；行锁+原子事务+流水+持久幂等，来源不足整笔拒绝。负福利保留，退款不造新债，不加累计充值/返佣。
- 余额/签到/礼遇显示统一两位并消除负零；顶部桌面/手机、个人资料概览分别显示充值和福利；流水完整换行，历史到退款切换关闭旧弹窗。API/CSV账目精度保留。
- 验证：前端完整430文件3433项，最终定向30项；lint/typecheck/build；Go三包完整unit/build/vet/定向race；11项真实PG/Redis钱包集成；正式构建本地API模拟浏览器桌面390px/中英/提交参数/超退通过。
- 未提交/合并/推送/tag/部署，无生产改动。指定用户真实负数是否多扣尚无流水证据；只修展示不清账。细节refund-welfare-display-plan.md与docs/welfare-balance-checkin.md。

## [18:53] 安全加固与可配置比例：断电后续做
- 已完成福利净额/比例/正充值匹配上限，DB读取策略、冻结快照、管理员配置及动态规则显示；签到无效令牌拒发、金额验证与翻倍十进制精度、礼遇安全随机。规则与回退文档已更新。
- 恢复后全Go unit、四包race、build/vet、前端434/3448及lint/build、正式构建mock浏览器中英/手机/保存与非法参数验证通过；新微额及非法金额测试通过。
- Docker等待macOS授权已解除，用户回复已完成；真实数据库15项集成重新运行中。详见wallet-security-policy-plan.md。

## [18:55] 完成：福利计费安全与可配置比例
- Docker授权解除后，CI=true的隔离PostgreSQL18.1/Redis8.4实际15项TestWalletIntegration全PASS，含最新微额服务到数据库、快照APIKey绑定、旧notes与capture后取消不重退。最终四包完整unit亦通过。
- 文档/计划已同步净额规则、后台比例入口、保守充值准入、冻结快照、无DDL与回退顺序；4/4主线完成。无本地代码或验证遗留。仅本地开发，保持feature/refund-welfare-display未提交，未合并/push/tag/部署，生产客户流水未核验。

## 后续动作：用户授权本地合并
- 核对工作区73个任务相关文件及既有验证日志，feature/refund-welfare-display 与 custom-main 同基线 abd49c9db，diff检查通过；提交后fast-forward合入custom-main。
- 范围包含退款/两位金额/余额分栏及福利净额安全/可配置比例/签到账务加固；不push、tag或部署，最终提交与分支以Git历史为准。

## [19:03] 发布准备：v0.2.21.3
- 用户明确授权打tag、提交推送；aef9e009f已合入custom-main，VERSION更新0.2.21.3。
- 发布工具11项在隔离Linux全PASS；本机Bash3语法不兼容未改脚本；sh解析、embed构建及实际-version核验通过。业务回归复用当前制品完整记录。
- annotated tag与custom-main将原子推送origin，实时确认RELEASE_ONLY=true；不部署/不改生产数据库。最终以远端refs和Release工作流核验为准。详见release-v0.2.21.3.md。
