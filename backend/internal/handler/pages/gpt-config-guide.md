# GPT 一键配置

优先使用 CC Switch 一键导入；需要脚本或配置文件时，可继续使用页面底部的手动配置。

本文中的服务地址已经配置为：`https://www.loomex.lol`

## 用 CC Switch 一键接入

不需要手动填写地址，也不需要替换配置文件。安装 CC Switch 后，按下面四步完成导入、启用和连接测试。

### 准备：安装 CC Switch

Windows 选择安装版，macOS 选择 DMG。安装后先启动一次，让系统完成应用注册。

- [下载 Windows 安装版（.msi）](https://github.com/farion1231/cc-switch/releases/download/v3.20.0/CC-Switch-v3.20.0-Windows.msi)
- [下载 macOS 安装版（.dmg）](https://github.com/farion1231/cc-switch/releases/download/v3.20.0/CC-Switch-v3.20.0-macOS.dmg)
- [查看全部版本（GitHub）](https://github.com/farion1231/cc-switch/releases/latest)
- [打开 CC Switch 官方网站](https://ccswitch.io/zh/)

当前教程对应稳定版 v3.20.0，下载来源为 CC Switch 官方 GitHub Release。

### 1. 点击“导入到 CCS”

启动一次 CC Switch，并完全退出正在运行的 Codex。然后回到左侧“API 密钥”页面，在密钥列表中找到你本人状态为“活跃”的密钥，点击这一行右侧的“导入到 CCS”。

浏览器询问是否打开 CC Switch 时，选择允许。

### 2. 核对信息并导入

确认三项信息：

- 应用类型：`Codex`
- 供应商：`Loomex`
- 模型：`gpt-5.5`

确认后点击右下角“导入”。完成后，CC Switch 的 Codex 列表中会出现 `loomex` 卡片。

### 3. 将 Loomex 设为当前配置

在 `loomex` 卡片右侧点击“使用”，让 CC Switch 把它写入当前 Codex 配置。按钮由“使用”变成“使用中”即表示完成。

### 4. 测试连接并重新打开 Codex

点击卡片右侧的连接测试图标。测试通过后，完全退出 Codex，再重新打开。

顶部显示绿色“loomex 连通正常”和延迟时间，即表示连接已完成。卡片显示“使用中”、连接测试显示“连通正常”后，重新打开 Codex 即可开始新对话。

## 手动配置与脚本下载

不使用 CC Switch，或者需要单独生成 `config.toml`、`auth.json` 时，再使用下面的工具。

### 服务地址

请填写：

```text
https://www.loomex.lol
```

然后填写默认模型，并选择或粘贴你自己的 API 密钥。生成内容只会写入当前浏览器下载的文件。

页面提供以下操作：

- **复制 macOS 配置命令**：复制一条命令，在终端中运行完成配置。
- **下载 config.toml + auth.json**：下载配置文件，按提示放到 Codex 配置目录。
- **下载 Windows 配置脚本**：下载 PowerShell 脚本自动完成配置。
- **刷新密钥列表**：重新读取当前账号的 API 密钥。

配置会保留 WebSocket 连接复用，并在覆盖本地文件前创建备份。完成后请完全退出 Codex，再重新打开。

### macOS：复制一条命令完成配置

1. 在左侧选择你自己的密钥。
2. 点击“复制 macOS 配置命令”。
3. 按 `Command + 空格` 打开“终端”，按 `Command + V` 粘贴并回车。
4. 看到完成提示后，完全退出 Codex，再重新打开。

命令会先备份现有配置，再写入新配置。

### Windows：下载脚本自动配置

运行前，请在右下角托盘完全退出 Codex。

1. 在左侧选择你自己的密钥。
2. 点击“下载 Windows 配置脚本”。
3. 右键脚本，选择“使用 PowerShell 运行”。
4. 脚本完成后重新打开 Codex。

## 常见问题

### 没有自动打开 CC Switch

先手动启动 CC Switch，再返回“API 密钥”页面重新点击“导入到 CCS”。浏览器出现外部应用提示时，选择允许。

### 连接测试没有通过

确认密钥仍为“活跃”且属于当前账号。删除 CC Switch 中的旧卡片后重新导入，再进行连接测试。

### 没有读取到密钥

前往左侧“API 密钥”页面复制完整密钥，再粘贴到手动配置区域。只使用你自己的密钥，不要把密钥分享给其他人。
