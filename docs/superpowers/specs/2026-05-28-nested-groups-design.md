# Nested Groups — Design Spec

## Overview
Support multi-level nested groups (parent/child hierarchy) replacing the current flat Group model. Recording output paths reflect the full group ancestry: `recordings/{group_a}/{group_b}/.../{stream_name}/...`.

---

## Data Model

### Group
| Field        | Type      | Notes                                      |
|--------------|-----------|--------------------------------------------|
| id           | uint      | Primary key                                |
| name         | string    | Unique per level (no cross-level constraint) |
| parent_id    | *uint     | Self-referencing FK, nullable (root group)  |
| sort_order   | int       | Sibling-only sort, default 0               |
| created_at   | time.Time |                                            |
| updated_at   | time.Time |                                            |

- `parent_id` FK → `groups.id` ON DELETE SET NULL (prevent orphaned children when parent deleted — handled in app logic instead, cascading delete)
- Children are not a GORM field; frontend builds tree from flat list via parent_id

### Group Tree (API response shape, not a DB model)
```json
{
  "id": 1,
  "name": "摄像头A",
  "parent_id": null,
  "sort_order": 1,
  "children": [
    {
      "id": 2,
      "name": "大门",
      "parent_id": 1,
      "sort_order": 1,
      "children": []
    }
  ]
}
```

### Stream (changes)
- `group_id` continues to reference a group at any level. No change to the Stream model itself.

---

## Backend API

### Group CRUD

**GET /api/groups** — Returns flat list with parent_id. Frontend builds tree client-side.

**POST /api/groups**
```json
{ "name": "大门", "parent_id": 1 }
```
- parent_id is optional (null → root group)
- sort_order auto-assigned to max+1 among siblings

**PUT /api/groups/:id**
```json
{ "name": "新名称", "parent_id": 2 }
```
- parent_id change = move to another parent
- O(1) operation — no cascading path updates needed

**DELETE /api/groups/:id**
- Deletes the group AND all descendant groups recursively (app-level cascade)
- Sets `group_id = NULL` on all streams belonging to any deleted group

**PUT /api/groups/reorder**
```json
{ "parent_id": 1, "order": [5, 3, 7] }
```
- parent_id: which parent's children to reorder (null for root-level)
- order: array of child group IDs in desired order
- Only affects direct children of the specified parent

### Recording Path Builder
In `RecorderService.buildOutputPath`:
1. If stream has no group, path = `{StorageLocalPath}/{stream_name}/...`
2. If stream has group, walk up the parent chain (recursive LOAD or loop) to get full ancestry from root to leaf
3. Path = `{StorageLocalPath}/{parent}/{child}/.../{stream_name}/...`

Example: stream "大门" in group "大厅" (parent="1F", grandparent="摄像头A")
→ `recordings/摄像头A/1F/大厅/大门/20260528_120000.mp4`

---

## Frontend

### Groups Page (`/groups`)
- Tree view with expand/collapse per node (ChevronRight/ChevronDown icons)
- Each node: indent level + `GripVertical` drag handle + name + edit/delete buttons
- **Drag & Drop**: HTML5 native DnD
  - Drop zone detection: top half = insert before, bottom half = insert after (same-level reorder), middle area = drop as child (reparent)
  - On drop: calls either `/api/groups/reorder` (same-level) or `/api/groups/:id` update parent_id (reparent)
- Add dialog: parent selector shows tree indentation (e.g., `摄像头A > 1F > 大厅`)

### Streams Page
- Group filter dropdown: tree indentation format
- Stream add/edit dialog group selector: same tree indentation
- StreamCard: show full group path badge (e.g., `摄像头A > 1F > 大厅`)

### StreamDetail Page
- Info section shows full group ancestry path

---

## Backend Changes Summary

| File | Changes |
|------|---------|
| `models/group.go` | Add ParentID *uint, remove unique on name |
| `database/database.go` | No change (already in AutoMigrate) |
| `handlers/group.go` | Update List (flat), Create (parent_id), Update (move), Delete (cascade), Reorder (per-parent) |
| `routes/routes.go` | Add PUT /groups/:id/move (optional, can use Update) |
| `services/recorder.go` | buildOutputPath: walk parent chain for full path |

## Frontend Changes Summary

| File | Changes |
|------|---------|
| `lib/api.ts` | Group interface: add parent_id, children? |
| `pages/Groups.tsx` | Full rewrite: tree view, expand/collapse, cross-level DnD |
| `pages/Streams.tsx` | Group filter + selector with tree indentation |
| `components/StreamCard.tsx` | Full group path badge |
| `pages/StreamDetail.tsx` | Full group path in info |
