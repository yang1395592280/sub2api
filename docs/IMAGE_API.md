# 图片生成 API 对接说明

本文档面向 `https://www.loomex.site` 的 OpenAI 兼容图片接口客户。

## Base URL

```text
https://www.loomex.site/v1
```

## 鉴权

所有请求都需要在 Header 中携带 API Key：

```http
Authorization: Bearer YOUR_API_KEY
Content-Type: application/json
```

## 文生图

`POST /images/generations`

### 请求示例

```bash
curl https://www.loomex.site/v1/images/generations \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "一只橘猫坐在未来城市的窗边",
    "size": "1024x1024",
    "quality": "high",
    "output_format": "png",
    "response_format": "url"
  }'
```

### 返回示例

```json
{
  "created": 1710000007,
  "model": "gpt-image-2",
  "data": [
    {
      "url": "https://www.loomex.site/v1/images/files/abc123.png"
    }
  ]
}
```

如果请求 `response_format` 为 `b64_json`，返回会包含 `b64_json` 字段：

```json
{
  "created": 1710000007,
  "model": "gpt-image-2",
  "data": [
    {
      "b64_json": "iVBORw0KGgoAAAANSUhEUg..."
    }
  ]
}
```

## 图生图

`POST /images/edits`

### JSON 请求示例

```bash
curl https://www.loomex.site/v1/images/edits \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "把背景替换为夜晚的海边",
    "images": [
      {
        "image_url": "https://www.loomex.site/example/source.png"
      }
    ],
    "size": "1024x1024",
    "response_format": "url"
  }'
```

### Multipart 请求示例

```bash
curl https://www.loomex.site/v1/images/edits \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -F "model=gpt-image-2" \
  -F "prompt=把背景替换为夜晚的海边" \
  -F "size=1024x1024" \
  -F "response_format=url" \
  -F "image=@source.png"
```

## 参数

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `model` | string | 否 | 图片模型，推荐 `gpt-image-2`。 |
| `prompt` | string | 是 | 图片描述或编辑指令。 |
| `size` | string | 否 | 常用值：`1024x1024`、`1824x1024`、`1024x1824`、`2048x2048`、`3840x2160`、`2160x3840`。 |
| `response_format` | string | 否 | `url` 或 `b64_json`，默认建议使用 `url`。 |
| `output_format` | string | 否 | 输出格式：`png`、`jpeg`、`webp`。 |
| `quality` | string | 否 | 输出质量，例如 `auto`、`medium`、`high`。 |

## 错误格式

错误响应保持 OpenAI 兼容格式：

```json
{
  "error": {
    "message": "Image generation failed",
    "type": "image_generation_failed",
    "code": "image_generation_failed"
  }
}
```
