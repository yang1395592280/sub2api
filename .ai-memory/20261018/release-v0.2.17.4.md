# v0.2.17.4 发布准备

用户授权发布 `v0.2.17.4`：提交版本更新、创建 annotated tag，并推送 `custom-main` 与 tag。

- 基线为 `v0.2.17.3`（`c0d5729ef`）；待发布功能提交为 `cac3fd8e6`，内容为撤回分组渠道监控与鹈鹕检测扩展、保留并收窄二改集中设置、修复简洁首页加入社群入口。
- 发布版本写入 `backend/cmd/server/VERSION`，由服务端二进制通过 `go:embed` 读取。
- 创建 tag 前验证版本构建与发布工具契约；创建 tag 后验证 tag、HEAD 与 VERSION 一致，再原子推送 `origin/custom-main` 和 `refs/tags/v0.2.17.4`。
- 仓库变量 `RELEASE_ONLY=true` 时，远端工作流只创建 GitHub Release 及归档/checksum，不发布 Docker/GHCR。未部署生产，也未执行数据库迁移。

验证：嵌入式 release 二进制 `-version` 输出 `0.2.17.4`；发布矩阵中非镜像的 9 项测试和镜像脚本语法检查通过。3 个调用镜像脚本的测试只能在 Bash 4+ 运行，macOS 自带 Bash 3.2 因不支持 `${var,,}` 失败；GitHub Actions 使用 Ubuntu Bash，脚本保持原样。
