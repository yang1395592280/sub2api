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
