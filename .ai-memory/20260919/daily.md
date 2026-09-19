session-id: 20260919-2247

## [22:47] 动作: 修复上游合并后的两个过时后端测试

- 文件: `backend/internal/service/openai_images_test.go`, `backend/internal/service/sub2api_checkin_service_test.go`
- 决策: 仅更新测试契约，不修改已由现有测试验证的生产逻辑。
- 根因: OAuth 图片映射测试仍返回旧 Responses SSE；签到测试仍把现已支持的 Gemini 当作未支持平台。
- 验证: 两个定向测试通过；`go test ./...` 全部通过；`git diff --check` 通过。
