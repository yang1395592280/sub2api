# Workbench Records Detail, Performance, and Image Edit Design

## Background

The admin workbench records page currently renders conversation detail below the list after clicking "view". This makes the page jump and keeps the detail coupled to the table layout. Images in the admin detail are static thumbnails and cannot be enlarged.

The page also feels slow because the first screen loads both list data and global stats. The backend stats path scans workbench image outputs to calculate image bytes, and the list path performs extra message reads for image counts and bytes.

The user workbench image edit flow already sends reference images through `options.images`, and the backend converts data URLs to multipart uploads for `/v1/images/edits`. However, the internal workbench gateway client discards non-2xx response bodies and returns only generic `gateway returned <status>` errors. Empty image responses are also treated as a success with "未返回图片", which makes failures hard to diagnose.

## Goals

- Show admin workbench conversation detail in a modal dialog instead of inline below the list.
- Allow images inside workbench content, especially admin detail images, to be enlarged.
- Improve perceived load speed of the admin workbench records page without a risky schema migration.
- Make web workbench image edits easier to diagnose and prevent empty image responses from appearing as successful results.

## Non-Goals

- No database schema migration for denormalized image counters in this change.
- No broad redesign of the workbench user page.
- No changes to upstream image model routing or account scheduling rules.
- No exposure of secrets, tokens, or raw provider response bodies in UI errors.

## Proposed Approach

Use a focused middle path:

1. Change `frontend/src/views/admin/AdminWorkbenchView.vue` so "view" opens a teleported modal. The list remains stable while detail loads inside the modal.
2. Add a lightbox state to the admin page. Detail images become clickable thumbnails, using the same data URL fallback already used elsewhere.
3. Load admin list and stats separately. The list should render as soon as its own request finishes; stats failures should not block the table. Keep the existing stats API but remove it from the critical path.
4. Keep backend list image stats limited to the current page. Avoid larger refactors unless tests show a clear need.
5. Update the workbench gateway client so non-2xx image/edit errors preserve a short, sanitized upstream error message when available.
6. Treat image responses with no returned images as an error for the pending assistant image message, so the UI stops polling and shows a clear message.

## Components

### Admin Workbench View

- `detail` remains the selected conversation detail payload.
- Add `detailOpen`, `detailLoading`, and `lightboxImage` state.
- `openDetail(id)` opens the modal immediately, loads detail, and shows a loading state.
- Close actions clear the modal state without affecting list filters or selection.
- Thumbnail images call `openLightbox(imageURL(image))`.

### User Workbench Image Flow

- Keep the existing optimistic send and pending polling behavior.
- Existing polling will pick up the backend error state after async image edit completion fails or returns no images.
- No UI copy overhaul is required beyond any missing translation key needed for clearer errors.

### Backend Workbench Service

- In `completeImageMessage`, if gateway returns no error but `resp.Images` is empty, mark the message as `error` with a user-facing "未返回图片" style message.
- Keep success behavior unchanged for one or more images.

### Backend Workbench Gateway Client

- When a non-2xx response is returned, read a bounded body and extract `error.message` when JSON is present.
- Sanitize secrets before returning the message.
- Preserve status information in the error string.
- Keep the current generic fallback when no safe message exists.

## Data Flow

Admin records:

1. Page mounts.
2. Start list request and stats request separately.
3. Render table when list returns.
4. Update stat cards when stats returns.
5. Clicking "view" opens the detail modal and requests `/admin/workbench/conversations/:id`.
6. Clicking an image in the modal opens a full-screen lightbox.

Image edit:

1. User selects image edit mode and uploads reference images.
2. Frontend sends `endpoint: images_edits` and `options.images` containing data URLs.
3. Workbench service creates user and pending assistant messages.
4. Async gateway client sends multipart request to `/v1/images/edits`.
5. Success with images updates the assistant message to `success`.
6. Non-2xx or empty image response updates the assistant message to `error`.
7. Frontend polling refreshes the message and stops once status is no longer `pending`.

## Error Handling

- Admin detail load failures close or keep the modal with an error toast, matching existing page behavior.
- Stats load failures show an error toast but do not blank the list.
- Gateway errors are sanitized before persistence and UI display.
- Empty image output is treated as a failed generation/edit, not a successful response.

## Testing

Frontend:

- Update `frontend/src/views/admin/__tests__/AdminWorkbenchView.spec.ts` to assert detail opens in a modal.
- Add coverage for image thumbnail click opening the lightbox.
- Add coverage that list rendering does not depend on stats completion if practical within existing test structure.

Backend:

- Update `backend/internal/service/workbench_service_test.go` for empty image response becoming an error message.
- Add or update gateway client tests for sanitized upstream error extraction.
- Keep existing multipart image edit test passing.

Verification:

- Run focused frontend tests for `AdminWorkbenchView`.
- Run focused backend service tests for workbench.
- Run formatting where required.

## Risks

- The global stats endpoint may still be expensive on very large datasets because this design avoids a schema migration. It is removed from the first-render critical path but not made O(1).
- Surfacing sanitized upstream messages improves diagnostics but depends on upstream response shape.
- Modal layout must handle long messages and large images without overflowing small screens.
