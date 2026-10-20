# v0.2.17.5 发布准备

用户明确授权提交、创建 `v0.2.17.5` tag 并推送。礼遇余额缓存修复已在 `c09229472` 无冲突快进合入本地 `custom-main`。

- 发布内容：抽奖、幸运币及猜大小余额变更事务成功后，失效计费余额和用户鉴权缓存，解决礼遇到账后余额缓存偏低导致的 403；保留现有在途预留、事务及幂等语义。
- 版本更新：`backend/cmd/server/VERSION` 从 `0.2.17.4` 更新为 `0.2.17.5`。创建 annotated tag 后核对 tag、HEAD 和 VERSION，再原子推送 `origin/custom-main` 与该 tag。
- 远端预检：`custom-main` 为 `42f9e7ed4`，尚无 `v0.2.17.5`。直连 DNS 失败，使用 macOS 现有系统代理 `127.0.0.1:7890` 完成只读检查；代理仅用于本轮命令，不修改 Git 或系统配置。
- 发布模式：已读取仓库变量 `RELEASE_ONLY=true`，tag 推送触发已有 Release 工作流，发布归档及 checksum，不发布 Docker/GHCR。生产部署不在本轮范围内。

验证：`CGO_ENABLED=0 go build -tags=embed -ldflags='-s -w -X main.BuildType=release' -trimpath` 成功，生成二进制 `-version` 输出 `0.2.17.5`；发布矩阵 9 项非镜像测试通过，`bash -n .github/release-tools/release-images.sh` 通过。默认 Python 缺少 PyYAML，测试临时加载机器上已有的 PyYAML 路径，无安装或仓库依赖变更。镜像脚本执行测试依赖 Bash 4+，本机 Bash 3.2 未执行该两项测试方法。业务回归、race 和后端 build 沿用本轮修复已通过的验证。

本记录为推送前状态，推送结果及 Release 状态以后续远端 refs 和工作流核验为准。回退如需执行，应创建新的修复提交与新版本，不移动既有发布 tag。
