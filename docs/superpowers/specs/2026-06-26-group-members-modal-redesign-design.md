# Group Members Modal Redesign Design

## Goal

Redesign the admin group members modal so member usage is easier to scan, and restore the "view members" entry for every group type.

## Scope

- `GroupsView.vue` always shows the "view members" action for group rows.
- `GroupMembersModal.vue` replaces the dense usage table layout with stacked member panels.
- Exclusive groups with fixed members show each member's today/yesterday usage in a two-row comparison.
- Public groups continue to open the modal and show the existing no-fixed-members empty state when the backend returns no fixed members.
- Backend usage aggregation and endpoint contracts stay unchanged.

## UI Direction

Each member is rendered as a bordered row panel instead of a table row. The top line contains identity, email, notes, status, and the remove action when available. The usage section is a separate comparison area with two rows, today above yesterday. Each row shows requests, tokens, account cost (`A $...`), and user cost (`U $...`) in stable metric blocks.

## Verification

- Component tests cover the restored public-group entry behavior indirectly through the modal empty state and the redesigned exclusive usage display.
- Frontend typecheck validates the Vue template changes.
