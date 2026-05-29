# Nested Groups Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform flat groups into multi-level nested tree hierarchy with parent_id, tree UI, cross-level drag-and-drop, and hierarchical recording paths.

**Architecture:** Adjacency list (parent_id self-reference) for hierarchy. Flat API list from backend → frontend builds tree. Recording path walks parent chain at record time.

**Tech Stack:** Go/GORM (backend), React/Typescript/Radix UI (frontend), HTML5 Drag & Drop (tree reorder)

---

### File Structure

| File | Status | Responsibility |
|------|--------|---------------|
| `backend/models/group.go` | Modify | Add ParentID *uint field |
| `backend/handlers/group.go` | Rewrite | Tree-aware CRUD + per-parent reorder + cascade delete |
| `backend/services/recorder.go:buildOutputPath` | Modify | Walk parent chain for full group path |
| `frontend/src/lib/api.ts` | Modify | Group interface: add parent_id |
| `frontend/src/pages/Groups.tsx` | Rewrite | Tree view + expand/collapse + cross-level DnD |
| `frontend/src/pages/Streams.tsx` | Modify | Tree selector for group filter + dialog |
| `frontend/src/components/StreamCard.tsx` | Modify | Show full group ancestry path |
| `frontend/src/pages/StreamDetail.tsx` | Modify | Show full group ancestry in info |

---

### Task 1: Add ParentID to Group model

**Files:**
- Modify: `backend/models/group.go`

- [ ] **Step 1: Update Group model**

```go
package models

import "time"

type Group struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:255;not null" json:"name"`
	ParentID  *uint     `gorm:"index" json:"parent_id"`
	Children  []*Group  `gorm:"-" json:"children,omitempty"`
	SortOrder int       `gorm:"default:0" json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
```

- [ ] **Step 2: Verify build**

Run: `cd backend && go build ./...`
Expected: no errors

---

### Task 2: Update Group handlers for tree

**Files:**
- Modify: `backend/handlers/group.go` (full rewrite)

- [ ] **Step 1: Write new handlers**

```go
package handlers

import (
	"net/http"
	"strconv"

	"living-recorder/backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type GroupHandler struct {
	db *gorm.DB
}

func NewGroupHandler(db *gorm.DB) *GroupHandler {
	return &GroupHandler{db: db}
}

func (h *GroupHandler) List(c *gin.Context) {
	var groups []models.Group
	h.db.Order("sort_order asc").Find(&groups)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": groups})
}

func (h *GroupHandler) Create(c *gin.Context) {
	var input struct {
		Name     string `json:"name" binding:"required"`
		ParentID *uint  `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": err.Error()})
		return
	}

	var maxOrder int64
	query := h.db.Model(&models.Group{}).Select("COALESCE(MAX(sort_order), 0)")
	if input.ParentID != nil {
		query = query.Where("parent_id = ?", *input.ParentID)
	} else {
		query = query.Where("parent_id IS NULL")
	}
	query.Scan(&maxOrder)

	group := models.Group{
		Name:      input.Name,
		ParentID:  input.ParentID,
		SortOrder: int(maxOrder) + 1,
	}
	if err := h.db.Create(&group).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 0, "data": group})
}

func (h *GroupHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var group models.Group
	if err := h.db.First(&group, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "message": "group not found"})
		return
	}

	var input struct {
		Name     *string `json:"name"`
		ParentID *uint   `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.ParentID != nil {
		// Moving to new parent: append to end of new siblings
		var maxOrder int64
		h.db.Model(&models.Group{}).Select("COALESCE(MAX(sort_order), 0)").
			Where("parent_id = ?", *input.ParentID).Scan(&maxOrder)
		updates["parent_id"] = *input.ParentID
		updates["sort_order"] = int(maxOrder) + 1
	} else if input.ParentID != nil || group.ParentID != nil {
		// Moving to root level
		var maxOrder int64
		h.db.Model(&models.Group{}).Select("COALESCE(MAX(sort_order), 0)").
			Where("parent_id IS NULL").Scan(&maxOrder)
		updates["parent_id"] = nil
		updates["sort_order"] = int(maxOrder) + 1
	}

	if len(updates) > 0 {
		if err := h.db.Model(&group).Updates(updates).Error; err != nil {
			c.JSON(http.StatusConflict, gin.H{"code": 1, "message": err.Error()})
			return
		}
	}
	h.db.First(&group, id)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": group})
}

func (h *GroupHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	// Collect all descendant IDs recursively
	var allIDs []uint
	h.collectDescendantIDs(uint(id), &allIDs)
	allIDs = append(allIDs, uint(id))

	// Clear group references on streams
	h.db.Model(&models.Stream{}).Where("group_id IN ?", allIDs).Update("group_id", nil)
	// Delete all groups in the subtree
	h.db.Delete(&models.Group{}, allIDs)

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}

func (h *GroupHandler) collectDescendantIDs(parentID uint, ids *[]uint) {
	var children []models.Group
	h.db.Where("parent_id = ?", parentID).Find(&children)
	for _, child := range children {
		*ids = append(*ids, child.ID)
		h.collectDescendantIDs(child.ID, ids)
	}
}

func (h *GroupHandler) Reorder(c *gin.Context) {
	var input struct {
		ParentID *uint  `json:"parent_id"`
		Order    []uint `json:"order" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": err.Error()})
		return
	}

	for i, id := range input.Order {
		h.db.Model(&models.Group{}).Where("id = ?", id).Update("sort_order", i+1)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "reordered"})
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd backend && go build ./...`
Expected: no errors

---

### Task 3: Update recording path for nested groups

**Files:**
- Modify: `backend/services/recorder.go`

- [ ] **Step 1: Update buildOutputPath to walk parent chain**

Find `func (s *RecorderService) buildOutputPath` and replace with:

```go
func (s *RecorderService) buildOutputPath(stream *models.Stream, task *models.RecordTask) string {
	tpl := task.OutputTemplate
	now := time.Now()
	name := stream.Name

	tpl = strings.ReplaceAll(tpl, "{name}", name)
	tpl = strings.ReplaceAll(tpl, "{date}", now.Format("20060102"))
	tpl = strings.ReplaceAll(tpl, "{time}", now.Format("150405"))
	tpl = strings.ReplaceAll(tpl, "{protocol}", stream.Protocol)
	tpl = strings.ReplaceAll(tpl, "{id}", fmt.Sprintf("%d", stream.ID))

	outputPath := filepath.Join(s.cfg.StorageLocalPath, tpl)

	if stream.Group != nil && stream.Group.Name != "" {
		groupPath := s.buildGroupPath(stream.Group)
		outputPath = filepath.Join(s.cfg.StorageLocalPath, groupPath, tpl)
	}

	return outputPath
}

func (s *RecorderService) buildGroupPath(group *models.Group) string {
	names := []string{group.Name}
	currentID := group.ParentID
	for currentID != nil {
		var parent models.Group
		if err := s.db.First(&parent, *currentID).Error; err != nil {
			break
		}
		names = append([]string{parent.Name}, names...)
		currentID = parent.ParentID
	}
	return filepath.Join(names...)
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd backend && go build ./...`
Expected: no errors

---

### Task 4: Update Group interface in API client

**Files:**
- Modify: `frontend/src/lib/api.ts`

- [ ] **Step 1: Update Group interface**

Wait for it to prove it exists...

Current:
```typescript
export interface Group {
  id: number
  name: string
  sort_order: number
  created_at: string
  updated_at: string
}
```

Change to:
```typescript
export interface Group {
  id: number
  name: string
  parent_id: number | null
  children?: Group[]
  sort_order: number
  created_at: string
  updated_at: string
}
```

- [ ] **Step 2: Add tree-building utility**

At the bottom of `lib/api.ts`, add (or in a new file `lib/group-tree.ts`):

```typescript
export function buildTree(groups: Group[]): Group[] {
  const sorted = [...groups].sort((a, b) => a.sort_order - b.sort_order)
  const map = new Map<number, Group>()
  const roots: Group[] = []

  for (const g of sorted) {
    map.set(g.id, { ...g, children: [] })
  }

  for (const g of sorted) {
    const node = map.get(g.id)!
    if (g.parent_id != null && map.has(g.parent_id)) {
      map.get(g.parent_id)!.children!.push(node)
    } else {
      roots.push(node)
    }
  }

  return roots
}

export function flattenTree(tree: Group[], depth = 0): (Group & { depth: number })[] {
  const result: (Group & { depth: number })[] = []
  for (const node of tree) {
    result.push({ ...node, depth })
    if (node.children && node.children.length > 0) {
      result.push(...flattenTree(node.children, depth + 1))
    }
  }
  return result
}
```

- [ ] **Step 3: Verify TypeScript**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors

---

### Task 5: Rewrite Groups page with tree view and cross-level DnD

**Files:**
- Rewrite: `frontend/src/pages/Groups.tsx`

- [ ] **Step 1: Write the new Groups page**

```typescript
import { useEffect, useState, useRef, useCallback } from 'react'
import { api, Group, buildTree, flattenTree } from '@/lib/api'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select'
import {
  GripVertical, Plus, Pencil, Trash2, Layers,
  ChevronRight, ChevronDown,
} from 'lucide-react'

export default function Groups() {
  const [groups, setGroups] = useState<Group[]>([])
  const [expanded, setExpanded] = useState<Set<number>>(new Set())
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<Group | null>(null)
  const [name, setName] = useState('')
  const [parentId, setParentId] = useState<string>('_root')
  const dragId = useRef<number | null>(null)
  const dragOverId = useRef<number | null>(null)
  const dropPosition = useRef<'before' | 'after' | 'child' | null>(null)

  const load = () => api.groups.list().then(setGroups)
  useEffect(() => { load() }, [])

  const tree = buildTree(groups)
  const flatTree = flattenTree(tree)

  const getParentIdForNewGroup = useCallback(() => {
    return parentId === '_root' ? null : Number(parentId)
  }, [parentId])

  const handleSave = async () => {
    if (!name.trim()) return
    if (editing) {
      await api.groups.update(editing.id, { name: name.trim() })
    } else {
      await api.groups.create({ name: name.trim(), parent_id: getParentIdForNewGroup() })
    }
    setOpen(false)
    setEditing(null)
    setName('')
    setParentId('_root')
    load()
  }

  const handleDelete = async (id: number) => {
    await api.groups.delete(id)
    load()
  }

  const toggleExpand = (id: number) => {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  // Drag & Drop handlers
  const handleDragStart = (id: number) => {
    dragId.current = id
  }

  const handleDragOver = (e: React.DragEvent, id: number) => {
    e.preventDefault()
    dragOverId.current = id
    const rect = e.currentTarget.getBoundingClientRect()
    const y = e.clientY - rect.top
    const h = rect.height
    if (y < h * 0.25) {
      dropPosition.current = 'before'
    } else if (y > h * 0.75) {
      dropPosition.current = 'after'
    } else {
      dropPosition.current = 'child'
    }
  }

  const handleDrop = async () => {
    const draggedId = dragId.current
    const targetId = dragOverId.current
    const pos = dropPosition.current
    dragId.current = null
    dragOverId.current = null
    dropPosition.current = null

    if (draggedId == null || targetId == null || pos == null || draggedId === targetId) return

    const target = groups.find((g) => g.id === targetId)
    if (!target) return

    if (pos === 'child') {
      // Move as child of target
      await api.groups.update(draggedId, { parent_id: targetId })
    } else {
      // Move as sibling of target (same parent)
      const parentGroups = groups.filter((g) => g.parent_id === target.parent_id)
      const siblings = [...parentGroups].filter((g) => g.id !== draggedId)
      const targetIdx = siblings.findIndex((g) => g.id === targetId)
      const insertIdx = pos === 'before' ? targetIdx : targetIdx + 1
      siblings.splice(insertIdx, 0, groups.find((g) => g.id === draggedId)!)
      const order = siblings.map((g) => g.id)

      // Update parent_id if needed
      if (target.parent_id !== groups.find((g) => g.id === draggedId)?.parent_id) {
        await api.groups.update(draggedId, { parent_id: target.parent_id ?? undefined })
      }

      await api.groups.reorder(target.parent_id ?? undefined, order)
    }
    load()
  }

  // Render a single tree node and its children (recursive)
  const renderNode = (node: Group, depth: number): JSX.Element => {
    const hasChildren = node.children && node.children.length > 0
    const isExpanded = expanded.has(node.id)

    return (
      <div key={node.id}>
        <div
          className="flex items-center gap-2 px-2 py-2 rounded-lg glass-hover cursor-default transition-colors"
          style={{ paddingLeft: `${depth * 24 + 8}px` }}
          draggable
          onDragStart={() => handleDragStart(node.id)}
          onDragOver={(e) => handleDragOver(e, node.id)}
          onDrop={handleDrop}
        >
          <button
            className={`p-0.5 text-white/30 hover:text-white/60 transition-colors ${!hasChildren && 'invisible'}`}
            onClick={() => toggleExpand(node.id)}
          >
            {isExpanded ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
          </button>
          <div className="cursor-grab active:cursor-grabbing text-white/30 hover:text-white/60 transition-colors">
            <GripVertical className="h-4 w-4" />
          </div>
          <span className="flex-1 text-sm font-medium text-white/80">{node.name}</span>
          <Button
            size="sm"
            variant="ghost"
            onClick={() => { setEditing(node); setName(node.name); setParentId(String(node.parent_id ?? '_root')); setOpen(true) }}
          >
            <Pencil className="h-3 w-3 mr-1" />
            编辑
          </Button>
          <Button size="sm" variant="ghost" onClick={() => handleDelete(node.id)}>
            <Trash2 className="h-3 w-3 mr-1" />
            删除
          </Button>
        </div>
        {hasChildren && isExpanded && node.children!.map((child) => renderNode(child, depth + 1))}
      </div>
    )
  }

  // Build parent options for the dialog (tree indentation)
  const renderParentOption = (node: Group, depth: number): JSX.Element[] => {
    const prefix = '\u00A0\u00A0'.repeat(depth)
    const label = depth > 0 ? `${prefix}${node.name}` : node.name
    const result: JSX.Element[] = [
      <SelectItem key={node.id} value={String(node.id)}>{label}</SelectItem>,
    ]
    if (node.children) {
      for (const child of node.children) {
        result.push(...renderParentOption(child, depth + 1))
      }
    }
    return result
  }

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <div className="flex items-center gap-3">
          <div className="p-2 rounded-lg bg-iridescent-purple/10 border border-iridescent-purple/20">
            <Layers className="h-5 w-5 text-iridescent-purple" />
          </div>
          <h2 className="text-2xl font-bold text-white/90">分组管理</h2>
        </div>
        <Dialog open={open} onOpenChange={(v) => { setOpen(v); if (!v) { setEditing(null); setName(''); setParentId('_root') } }}>
          <DialogTrigger asChild>
            <Button onClick={() => { setEditing(null); setName(''); setParentId('_root') }}>
              <Plus className="h-4 w-4 mr-1" />
              添加分组
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{editing ? '编辑分组' : '添加分组'}</DialogTitle>
            </DialogHeader>
            <div className="space-y-4">
              <div className="space-y-2">
                <Label>分组名称</Label>
                <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="例：摄像头A" />
              </div>
              {!editing && (
                <div className="space-y-2">
                  <Label>父分组</Label>
                  <Select value={parentId} onValueChange={setParentId}>
                    <SelectTrigger>
                      <SelectValue placeholder="根级分组" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="_root">根级分组</SelectItem>
                      {tree.flatMap((node) => renderParentOption(node, 0))}
                    </SelectContent>
                  </Select>
                </div>
              )}
              <Button onClick={handleSave} className="w-full">
                {editing ? '更新' : '创建'}
              </Button>
            </div>
          </DialogContent>
        </Dialog>
      </div>

      {groups.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center">
            <Layers className="h-10 w-10 text-white/20 mx-auto mb-3" />
            <p className="text-white/40">暂无分组，点击上方按钮添加</p>
          </CardContent>
        </Card>
      ) : (
        <div className="glass rounded-xl p-2 space-y-0.5">
          {tree.map((node) => renderNode(node, 0))}
        </div>
      )}
    </div>
  )
}
```

Wait — the `api.groups.reorder` signature changed. Let me update that too.

In `api.ts`:
```typescript
reorder: (parentId: number | undefined, order: number[]) =>
  request<void>('/api/groups/reorder', {
    method: 'PUT',
    body: JSON.stringify({ parent_id: parentId ?? null, order }),
  }),
```

- [ ] **Step 2: Update api.groups.reorder signature**

Edit `frontend/src/lib/api.ts`:

```typescript
  groups: {
    list: () => request<Group[]>('/api/groups'),
    create: (data: { name: string; parent_id?: number | null }) =>
      request<Group>('/api/groups', { method: 'POST', body: JSON.stringify(data) }),
    update: (id: number, data: { name?: string; parent_id?: number | null | undefined }) =>
      request<Group>(`/api/groups/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
    delete: (id: number) => request<void>(`/api/groups/${id}`, { method: 'DELETE' }),
    reorder: (parentId: number | undefined | null, order: number[]) =>
      request<void>('/api/groups/reorder', {
        method: 'PUT',
        body: JSON.stringify({ parent_id: parentId ?? null, order }),
      }),
  },
```

Note: When `parent_id` is undefined/null, the body sends `null` to the backend, which means root-level sorting.

- [ ] **Step 3: Verify TypeScript**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors

---

### Task 6: Update Streams page with tree selector

**Files:**
- Modify: `frontend/src/pages/Streams.tsx`

- [ ] **Step 1: Import buildTree and update group selector**

Add import at top:
```typescript
import { api, Group, Stream, buildTree, flattenTree } from '@/lib/api'
```

Replace the group filter dropdown:
```typescript
{/* Group filter */}
<Select value={filterGroupId} onValueChange={setFilterGroupId}>
  <SelectTrigger className="w-36">
    <SelectValue placeholder="全部分组" />
  </SelectTrigger>
  <SelectContent>
    <SelectItem value="all">全部分组</SelectItem>
    {buildTree(groups).flatMap((node) =>
      flattenTree([node]).map((g) => (
        <SelectItem key={g.id} value={String(g.id)}>
          {'\u00A0\u00A0'.repeat(g.depth)}{g.name}
        </SelectItem>
      ))
    )}
  </SelectContent>
</Select>
```

Replace the group selector inside the dialog:
```typescript
<div className="space-y-2">
  <Label>分组</Label>
  <Select
    value={editing.group_id ? String(editing.group_id) : '_none'}
    onValueChange={(v) => setEditing({ ...editing, group_id: v === '_none' ? null : Number(v) })}
  >
    <SelectTrigger>
      <SelectValue placeholder="未分组" />
    </SelectTrigger>
    <SelectContent>
      <SelectItem value="_none">未分组</SelectItem>
      {buildTree(groups).flatMap((node) =>
        flattenTree([node]).map((g) => (
          <SelectItem key={g.id} value={String(g.id)}>
            {'\u00A0\u00A0'.repeat(g.depth)}{g.name}
          </SelectItem>
        ))
      )}
    </SelectContent>
  </Select>
</div>
```

- [ ] **Step 2: Verify TypeScript**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors

---

### Task 7: Update StreamCard and StreamDetail for full group path

**Files:**
- Modify: `frontend/src/components/StreamCard.tsx`
- Modify: `frontend/src/pages/StreamDetail.tsx`

- [ ] **Step 1: Add full group path helper**

In `lib/api.ts`, add:
```typescript
// Build the full ancestry string from a group (e.g. "摄像头A > 1F > 大厅")
export function getGroupPath(groups: Group[], groupId: number | null): string {
  if (!groupId) return ''
  const group = groups.find((g) => g.id === groupId)
  if (!group) return ''
  const parts: string[] = [group.name]
  let pid = group.parent_id
  while (pid != null) {
    const parent = groups.find((g) => g.id === pid)
    if (!parent) break
    parts.unshift(parent.name)
    pid = parent.parent_id
  }
  return parts.join(' > ')
}
```

- [ ] **Step 2: Update StreamCard to show full path**

In `StreamCard.tsx`, the group badge section changes to show full path. The component needs `groups` prop or we pass the path string. Since StreamCard doesn't have access to groups, pass the path as a prop.

Actually, simpler: show the full path in the StreamCard. The Stream has `group` preloaded from backend, but the Group only has immediate parent_id, not the full chain. So we need either:
- Load groups in StreamCard (use api.groups.list())
- Or just show the immediate group name for now, and the full path is visible on StreamDetail

Let me keep it simple: StreamCard shows the immediate group name badge. StreamDetail loads all groups and computes the full path.

In `StreamCard.tsx`, the code already shows `stream.group?.name`. That's fine for the card. The card is compact, showing full path might be too long.

In `StreamDetail.tsx`, add a full path display:

```typescript
// Add state
const [allGroups, setAllGroups] = useState<Group[]>([])

// In useEffect
api.groups.list().then(setAllGroups)

// In render, after the protocol field:
<div>
  <div className="text-xs text-white/40 mb-1">分组路径</div>
  <div className="text-sm text-white/70">
    {stream.group ? (
      <span className="rounded-md bg-iridescent-purple/10 text-iridescent-purple border border-iridescent-purple/20 px-2 py-0.5 text-xs">
        {getGroupPath(allGroups, stream.group_id)}
      </span>
    ) : (
      <span className="text-white/40">未分组</span>
    )}
  </div>
</div>
```

- [ ] **Step 3: Verify TypeScript**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors

---

### Task 8: Build and verify

**Files:**
- `frontend/dist/` (generated)
- `backend/embed/dist/` (synced)
- `build/living-recorder-linux`
- `build/living-recorder.exe`

- [ ] **Step 1: Build frontend**

Run:
```bash
cd frontend && npm run build
```
Expected: tsc + vite succeed, dist/ generated

- [ ] **Step 2: Copy dist to embed**

Run:
```bash
rm -rf backend/embed/dist && cp -r frontend/dist backend/embed/dist
```

- [ ] **Step 3: Cross-compile backend**

Run:
```bash
cd backend && GOOS=linux GOARCH=amd64 go build -o ../build/living-recorder-linux .
GOOS=windows GOARCH=amd64 go build -o ../build/living-recorder.exe .
```
Expected: binaries in build/

- [ ] **Step 4: Integration smoke test**

Run:
```bash
cd build && ./living-recorder-linux &
sleep 3
# Create root group
curl -s -X POST http://localhost:8080/api/groups -H 'Content-Type: application/json' -d '{"name":"摄像头A"}'
echo ""
# Create child group
curl -s -X POST http://localhost:8080/api/groups -H 'Content-Type: application/json' -d '{"name":"1F","parent_id":1}'
echo ""
# Create grandchild group
curl -s -X POST http://localhost:8080/api/groups -H 'Content-Type: application/json' -d '{"name":"大厅","parent_id":2}'
echo ""
# List all (flat with parent_id)
curl -s http://localhost:8080/api/groups
echo ""
# Reorder children of group 1
curl -s -X PUT http://localhost:8080/api/groups/reorder -H 'Content-Type: application/json' -d '{"parent_id":1,"order":[2]}'
echo ""
# Move group 3 to root
curl -s -X PUT http://localhost:8080/api/groups/3 -H 'Content-Type: application/json' -d '{"parent_id":null}'
echo ""
# Delete group 1 (cascade)
curl -s -X DELETE http://localhost:8080/api/groups/1
echo ""
kill %1 2>/dev/null; wait 2>/dev/null
```
Expected: All operations succeed, no errors

- [ ] **Step 5: Clean up test DB**

Run: `rm -f build/living-recorder.db`
