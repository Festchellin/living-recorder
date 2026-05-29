import { useEffect, useState, useRef } from 'react'
import { api, Group, buildTree } from '@/lib/api'
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
  const [parentId, setParentId] = useState('_root')
  const dragId = useRef<number | null>(null)
  const dragOverId = useRef<number | null>(null)
  const dropPosition = useRef<'before' | 'after' | 'child' | null>(null)

  const load = () => api.groups.list().then(setGroups)
  useEffect(() => { load() }, [])

  const tree = buildTree(groups)

  const handleSave = async () => {
    if (!name.trim()) return
    if (editing) {
      await api.groups.update(editing.id, { name: name.trim() })
    } else {
      await api.groups.create({ name: name.trim(), parent_id: parentId === '_root' ? null : Number(parentId) })
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
    const dragged = groups.find((g) => g.id === draggedId)
    if (!target || !dragged) return

    if (pos === 'child') {
      await api.groups.update(draggedId, { parent_id: targetId })
    } else {
      const newParentId = target.parent_id

      if (dragged.parent_id !== newParentId) {
        await api.groups.update(draggedId, { parent_id: newParentId ?? 0 })
        const updated = await api.groups.list()
        setGroups(updated)
        const siblings = updated.filter((g) => g.parent_id === newParentId && g.id !== draggedId)
        const targetIdx = siblings.findIndex((g) => g.id === targetId)
        const insertIdx = pos === 'before' ? targetIdx : targetIdx + 1
        const moved = updated.find((g) => g.id === draggedId)!
        siblings.splice(insertIdx, 0, moved)
        await api.groups.reorder(newParentId, siblings.map((g) => g.id))
      } else {
        const siblings = groups.filter((g) => g.parent_id === newParentId && g.id !== draggedId)
        const targetIdx = siblings.findIndex((g) => g.id === targetId)
        const insertIdx = pos === 'before' ? targetIdx : targetIdx + 1
        siblings.splice(insertIdx, 0, dragged)
        await api.groups.reorder(newParentId, siblings.map((g) => g.id))
      }
    }
    load()
  }

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
          <Button size="sm" variant="ghost" onClick={() => { setEditing(node); setName(node.name); setOpen(true) }}>
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
