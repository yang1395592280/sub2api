package handler

import "embed"

// embeddedPages 提供随二进制发布的内置 Markdown 页面。
// 数据目录中的同名页面优先，便于管理员在运行时覆盖默认内容。
//
//go:embed pages/*.md
var embeddedPages embed.FS
