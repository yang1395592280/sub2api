# Workbench Records Image Edits Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make admin workbench record detail open in a modal with image enlargement, improve perceived page load speed, and make workbench image edits report clear failures when the gateway errors or returns no image.

**Architecture:** Keep the existing Vue admin view and Go workbench service boundaries. Frontend changes stay in `AdminWorkbenchView.vue` and its focused spec. Backend changes stay in the workbench service/client and the existing workbench service tests.

**Tech Stack:** Vue 3 Composition API, Vitest, Go service tests, Ent repository, existing workbench API contracts.

## Global Constraints

- No database schema migration for denormalized image counters in this change.
- No broad redesign of the workbench user page.
- No changes to upstream image model routing or account scheduling rules.
- No exposure of secrets, tokens, or raw provider response bodies in UI errors.
- Keep edits scoped to the workbench page/service/test files.

---

### Task 1: Admin Workbench Modal, Lightbox, and Non-Blocking Stats

**Files:**
- Modify: `frontend/src/views/admin/AdminWorkbenchView.vue`
- Modify: `frontend/src/views/admin/__tests__/AdminWorkbenchView.spec.ts`

**Interfaces:**
- Consumes: `adminAPI.workbench.getStats(retentionDays)`, `listConversations(params)`, `getConversation(id)`.
- Produces: `openDetail(conversationId: number): Promise<void>`, `closeDetail(): void`, `openLightbox(url: string): void`, `closeLightbox(): void`, `imageURL(image: WorkbenchImageOutput): string`.

- [ ] **Step 1: Write failing modal/lightbox frontend tests**

Add tests in `frontend/src/views/admin/__tests__/AdminWorkbenchView.spec.ts`:

```ts
it('opens conversation detail inside a modal and closes it without removing the list', async () => {
  getConversation.mockResolvedValueOnce({
    conversation: { id: 2, title: 'image', mode: 'image', user_email: 'b@example.com' },
    messages: [{ id: 20, role: 'assistant', content: 'done', status: 'success', image_outputs: [] }],
  })
  const wrapper = mount(AdminWorkbenchView, {
    attachTo: document.body,
    global: { stubs: { AppLayout: AppLayoutStub } },
  })
  await flushPromises()

  await wrapper.get('[data-testid="admin-workbench-open-2"]').trigger('click')
  await flushPromises()

  expect(getConversation).toHaveBeenCalledWith(2)
  expect(document.body.querySelector('[data-testid="admin-workbench-detail-modal"]')?.textContent).toContain('done')
  expect(wrapper.text()).toContain('a@example.com')

  const close = document.body.querySelector('[data-testid="admin-workbench-detail-close"]') as HTMLButtonElement
  close.click()
  await wrapper.vm.$nextTick()

  expect(document.body.querySelector('[data-testid="admin-workbench-detail-modal"]')).toBeNull()
  wrapper.unmount()
})

it('opens a lightbox when an admin detail image is clicked', async () => {
  getConversation.mockResolvedValueOnce({
    conversation: { id: 2, title: 'image', mode: 'image', user_email: 'b@example.com' },
    messages: [{
      id: 20,
      role: 'assistant',
      content: 'done',
      status: 'success',
      image_outputs: [{ b64_json: 'ZmFrZQ==', mime_type: 'image/png' }],
    }],
  })
  const wrapper = mount(AdminWorkbenchView, {
    attachTo: document.body,
    global: { stubs: { AppLayout: AppLayoutStub } },
  })
  await flushPromises()

  await wrapper.get('[data-testid="admin-workbench-open-2"]').trigger('click')
  await flushPromises()

  const thumbnail = document.body.querySelector('[data-testid="admin-workbench-detail-image-20-0"]') as HTMLImageElement
  thumbnail.click()
  await wrapper.vm.$nextTick()

  const lightbox = document.body.querySelector('[data-testid="admin-workbench-image-lightbox"]')
  expect(lightbox?.querySelector('img')?.getAttribute('src')).toBe('data:image/png;base64,ZmFrZQ==')
  wrapper.unmount()
})
```

- [ ] **Step 2: Run frontend test to verify it fails**

Run:

```bash
cd frontend && pnpm test:run src/views/admin/__tests__/AdminWorkbenchView.spec.ts
```

Expected: FAIL because `admin-workbench-detail-modal`, modal close, and image lightbox do not exist yet.

- [ ] **Step 3: Implement admin modal and image lightbox**

In `AdminWorkbenchView.vue`:

```ts
import type { WorkbenchImageOutput, WorkbenchMode } from '@/api/workbench'

const detail = ref<AdminWorkbenchConversationDetail | null>(null)
const detailOpen = ref(false)
const detailLoading = ref(false)
const lightboxImage = ref('')

function imageURL(image: WorkbenchImageOutput): string {
  if (image.url) return image.url
  if (image.b64_json) return `data:${image.mime_type || 'image/png'};base64,${image.b64_json}`
  return ''
}

function openLightbox(url: string): void {
  if (url) lightboxImage.value = url
}

function closeLightbox(): void {
  lightboxImage.value = ''
}

function closeDetail(): void {
  detailOpen.value = false
  detail.value = null
  detailLoading.value = false
}

async function openDetail(conversationId: number): Promise<void> {
  detailOpen.value = true
  detail.value = null
  detailLoading.value = true
  try {
    detail.value = await adminAPI.workbench.getConversation(conversationId)
  } catch (error) {
    console.error(error)
    appStore.showError(t('admin.workbench.detailLoadFailed'))
  } finally {
    detailLoading.value = false
  }
}
```

Replace inline detail markup with a `Teleport to="body"` modal using `data-testid="admin-workbench-detail-modal"`, close button `data-testid="admin-workbench-detail-close"`, and image buttons/images with `data-testid="admin-workbench-detail-image-${message.id}-${index}"`.

- [ ] **Step 4: Make stats loading non-blocking**

Change `reload()` and mount flow so list loading controls the table:

```ts
async function reload(): Promise<void> {
  loading.value = true
  const statsPromise = loadStats().catch((error) => {
    console.error(error)
    appStore.showError(t('admin.workbench.loadFailed'))
  })
  try {
    await loadConversations()
  } catch (error) {
    console.error(error)
    appStore.showError(t('admin.workbench.loadFailed'))
  } finally {
    loading.value = false
  }
  await statsPromise
}
```

- [ ] **Step 5: Run frontend test to verify it passes**

Run:

```bash
cd frontend && pnpm test:run src/views/admin/__tests__/AdminWorkbenchView.spec.ts
```

Expected: PASS for all `AdminWorkbenchView` tests.

### Task 2: Backend Image Edit Failure Semantics

**Files:**
- Modify: `backend/internal/service/workbench_service.go`
- Modify: `backend/internal/service/workbench_gateway_client.go`
- Modify: `backend/internal/service/workbench_service_test.go`

**Interfaces:**
- Consumes: `WorkbenchGatewayImageResponse{Images []WorkbenchImageOutput}` from the gateway client.
- Produces: `completeImageMessage` marks empty image results as `error`, and `postBodyWithHeaders` returns sanitized status plus upstream message.

- [ ] **Step 1: Write failing empty image response test**

Add to `backend/internal/service/workbench_service_test.go`:

```go
func TestWorkbenchServiceSendImageAsyncEmptyResponseMarksMessageError(t *testing.T) {
	ctx := context.Background()
	repo := newWorkbenchMemoryRepo()
	apiKeys := &workbenchAPIKeyLookupStub{keys: map[int64]*APIKey{
		7: {ID: 7, UserID: 42, Key: "sk-test", Status: StatusAPIKeyActive, Name: "main"},
	}}
	gateway := &workbenchGatewayStub{image: WorkbenchGatewayImageResponse{
		Images:   nil,
		Metadata: map[string]any{"image_count": float64(0)},
	}}
	svc := NewWorkbenchService(repo, apiKeys, gateway)
	svc.asyncRunner = func(fn func()) { fn() }

	conv, err := svc.CreateConversation(ctx, 42, CreateWorkbenchConversationRequest{Mode: WorkbenchModeImage})
	require.NoError(t, err)

	_, err = svc.Send(ctx, 42, conv.ID, WorkbenchSendRequest{
		Mode:     WorkbenchModeImage,
		APIKeyID: 7,
		Endpoint: WorkbenchEndpointImagesEdits,
		Model:    "gpt-image-2",
		Input:    "replace background",
		Options: map[string]any{
			"images": []any{map[string]any{"image_url": "data:image/png;base64,ZmFrZQ=="}},
		},
	})
	require.NoError(t, err)

	messages, err := repo.ListMessages(ctx, 42, conv.ID)
	require.NoError(t, err)
	require.Len(t, messages, 2)
	require.Equal(t, WorkbenchMessageStatusError, messages[1].Status)
	require.NotNil(t, messages[1].ErrorMessage)
	require.Equal(t, "未返回图片", *messages[1].ErrorMessage)
	require.Empty(t, messages[1].ImageOutputs)
}
```

- [ ] **Step 2: Write failing sanitized upstream message test**

Update `TestHTTPWorkbenchGatewayClientPostJSONDoesNotLeakRawBody` or add:

```go
func TestHTTPWorkbenchGatewayClientPostJSONReturnsSanitizedUpstreamMessage(t *testing.T) {
	httpClient := &http.Client{Transport: workbenchRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"image is required for edits with key sk-secret"}}`)),
		}, nil
	})}

	client := &HTTPWorkbenchGatewayClient{client: httpClient, baseURL: "https://workbench.local"}

	err := client.postJSON(context.Background(), "/v1/images/edits", "Bearer sk-test", map[string]any{"model": "gpt-image-2"}, &map[string]any{})

	require.EqualError(t, err, "gateway returned 400: image is required for edits with key [redacted]")
	require.NotContains(t, err.Error(), "sk-secret")
}
```

- [ ] **Step 3: Run backend test to verify it fails**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestWorkbenchServiceSendImageAsyncEmptyResponseMarksMessageError|TestHTTPWorkbenchGatewayClientPostJSONReturnsSanitizedUpstreamMessage|TestHTTPWorkbenchGatewayClientGenerateImageWithInputImagesUsesEditsEndpoint'
```

Expected: FAIL because empty image responses are currently success and upstream message extraction is not implemented.

- [ ] **Step 4: Implement backend failure semantics**

In `completeImageMessage`, after `GenerateImage` returns:

```go
	if sendErr == nil && len(resp.Images) == 0 {
		sendErr = fmt.Errorf("未返回图片")
	}
```

Import `fmt` in `workbench_service.go` if needed. Keep the existing error update block so empty output stores status `error`.

In `workbench_gateway_client.go`, add helpers:

```go
func workbenchGatewayError(statusCode int, body []byte) error {
	message := strings.TrimSpace(gjson.GetBytes(body, "error.message").String())
	message = sanitizeWorkbenchGatewayErrorMessage(message)
	if message == "" {
		return fmt.Errorf("gateway returned %d", statusCode)
	}
	return fmt.Errorf("gateway returned %d: %s", statusCode, message)
}

func sanitizeWorkbenchGatewayErrorMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	message = strings.ReplaceAll(message, "\n", " ")
	message = strings.ReplaceAll(message, "\r", " ")
	message = regexp.MustCompile(`\bsk-[A-Za-z0-9_-]+\b`).ReplaceAllString(message, "[redacted]")
	message = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._-]+\b`).ReplaceAllString(message, "Bearer [redacted]")
	return truncateWorkbenchText(message, workbenchErrorMessageMax)
}
```

Use it in `postBodyWithHeaders`:

```go
if resp.StatusCode < 200 || resp.StatusCode >= 300 {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	return workbenchGatewayError(resp.StatusCode, body)
}
```

- [ ] **Step 5: Run backend test to verify it passes**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestWorkbenchServiceSendImageAsyncEmptyResponseMarksMessageError|TestHTTPWorkbenchGatewayClientPostJSONReturnsSanitizedUpstreamMessage|TestHTTPWorkbenchGatewayClientGenerateImageWithInputImagesUsesEditsEndpoint'
```

Expected: PASS.

### Task 3: Focused Verification

**Files:**
- Verify: `frontend/src/views/admin/AdminWorkbenchView.vue`
- Verify: `backend/internal/service/workbench_service.go`
- Verify: `backend/internal/service/workbench_gateway_client.go`

**Interfaces:**
- Consumes: completed Task 1 and Task 2 changes.
- Produces: verified focused frontend and backend behavior.

- [ ] **Step 1: Run frontend workbench admin tests**

Run:

```bash
cd frontend && pnpm test:run src/views/admin/__tests__/AdminWorkbenchView.spec.ts
```

Expected: PASS.

- [ ] **Step 2: Run frontend typecheck if focused tests pass**

Run:

```bash
cd frontend && pnpm typecheck
```

Expected: PASS.

- [ ] **Step 3: Run backend workbench service tests**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'Workbench'
```

Expected: PASS.

- [ ] **Step 4: Check diff and formatting**

Run:

```bash
git diff --check
git status --short
```

Expected: `git diff --check` exits 0. `git status --short` lists only the intended workbench frontend, backend, tests, and the ignored plan file if force-added.
