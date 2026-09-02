# Loomex 独立静态主页

该目录保存 `www.loomex.lol`（主域名）和 `www.loomex.space`（三网优化域名）共用的独立静态主页源码。主页与 Sub2API Vue 应用、API 路由分开部署，避免浏览器首页逻辑影响现有 API 请求。

## 文件

- `index.html`：静态主页。
- `loomex-icon.png`：主页 Logo 和 favicon。

## 服务器路径

生产环境当前部署到：

```text
/www/wwwroot/loomex-home/
```

部署前应先备份服务器上的现有文件，再将本目录中的 `index.html` 和 `loomex-icon.png` 安装到上述目录，并保持 `www:www`、`0644` 权限。

部署后至少执行以下检查：

```bash
sudo /www/server/nginx/sbin/nginx -t -c /www/server/nginx/conf/nginx.conf
curl -fsSL --compressed https://www.loomex.lol/
curl -fsSL --compressed https://www.loomex.space/
```

同时使用桌面端和移动端浏览器检查首页地址区块，确认文本完整显示且没有横向溢出。
