# v0.2.16.4 发布准备

用户明确要求打tag v0.2.16.4并提交推送。本次发布基于custom-main=cf988d214，已包含上游v0.2.17、分组用量刷新优化及一致快照修复；按用户指定使用二改版本号0.2.16.4。

## 范围

- backend/cmd/server/VERSION从0.2.17改为0.2.16.4；无业务代码追加修改。
- 已fetch origin，origin/custom-main仍为ae85f049b，是当前custom-main的祖先；远端尚无v0.2.16.4。
- 提交本次版本号和记录，创建annotated tag v0.2.16.4；原子推送origin/custom-main与精确tag，不推其他本地分支/tags，不移动旧tag。
- 远端RELEASE_ONLY=true；现有tag workflow会构建GitHub Release归档和校验文件，跳过Docker/GHCR发布。Release成功后workflow会按既有流程同步默认分支VERSION，未修改CI配置。

## 验证

- `go build -o /tmp/sub2api-release-v0.2.16.4 ./cmd/server`通过；二进制`-version`显示Sub2API 0.2.16.4。
- `bash backend/scripts/resolve-version.sh`返回0.2.16.4；git diff --check通过。
- 在临时venv安装现有requirements-release.txt后，ReleaseMatrixTest.test_plan_accepts_existing_four_part_release_tags及test_plan_requires_a_tag_for_publication均通过；未修改项目依赖。
- 业务代码复用本轮前端全量3114项、repository整包unit、9个日桶/用量PostgreSQL集成测试、相关三包race、并发集成race连续3次和服务端构建的已通过验证；详情见postmerge-bugfixes.md及upstream-v0217-sync.md。
- 打tag后再运行真实release plan核对tag/HEAD/VERSION一致，推送后核验远端分支与peeled tag同一提交，并查看Release workflow触发状态。

## 边界

本条为推送前记录，Git提交/tag绑定、远端refs及Release状态以实际执行核验为准。未执行生产部署/迁移，未验证生产负载、真实供应商及支付。版本号按用户指定，代码基线包含更新的上游v0.2.17。
