# 简化录制流程设计文档

## 概述

简化直播录制流程，去除"录制任务"概念，支持一键开始/停止录制，使操作更直观。

## 页面结构调整

- 删除导航栏中的「录制任务」入口
- 导航栏保留：**仪表盘 → 直播流 → 设置**
- 后端 `RecordTask` 模型和 API 保留（兼容性），前端的 Tasks 页面和路由删除

## 后端变更

### 配置新增 (`config/config.yaml`)

```yaml
recorder:
  max_parallel: 10
  restart_on_failure: 3
  health_check_interval: 30
  default_video_codec: "copy"       # 新增：无任务时的默认视频编码
  default_audio_codec: "copy"       # 新增：无任务时的默认音频编码
  default_output_template: "{name}/{date}_{time}.mp4"  # 新增
```

### Config 结构变更 (`config/config.go`)

`RecorderConfig` 新增字段：
- `DefaultVideoCodec string`
- `DefaultAudioCodec string`
- `DefaultOutputTemplate string`

### RecorderService 变更 (`services/recorder.go`)

**`Start()` 方法修改**：
- 当前逻辑：查找 `WHERE stream_id = ? AND enabled = true` 的 `RecordTask`，没找到返回错误
- 新逻辑：查找 `RecordTask`，没找到时使用默认参数构建一个临时 `RecordTask` 对象
- 不影响已有的定时任务功能

**新增 `StartAll()` 方法**：
- 遍历所有 `Enabled = true` 的 Stream
- 对每个非录制中的流调用 `Start()`
- 返回成功启动数量和失败列表

**新增 `StopAll()` 方法**：
- 遍历当前 `streams` map 中的所有活跃流
- 对每个调用 `Stop()`
- 返回成功停止数量

### 新增 API 路由 (`routes/routes.go`)

```go
streams.POST("/start-all", streamHandler.StartAll)
streams.POST("/stop-all", streamHandler.StopAll)
```

### StreamHandler 新增 (`handlers/stream.go`)

- `StartAll()` — 调用 `recorder.StartAll()`，返回结果
- `StopAll()` — 调用 `recorder.StopAll()`，返回结果

## 前端变更

### 导航更新 (`Layout.tsx`)

删除 `Tasks` 导航项，只保留 `Dashboard`、`Streams`、`Settings`。

### 页面路由更新 (`App.tsx`)

删除 `/tasks` 路由。

### Streams 页面 (`pages/Streams.tsx`)

**顶部工具栏新增**：

```tsx
<div className="flex gap-2">
  <Button onClick={handleStartAll} variant="default">
    ▶ 全部开始
  </Button>
  <Button onClick={handleStopAll} variant="destructive">
    ■ 全部停止
  </Button>
</div>
```

**`handleStartAll` / `handleStopAll`**：
- 分别调用 `api.streams.startAll()` / `api.streams.stopAll()`
- 成功后刷新流列表

**`api.ts` 新增**：

```ts
startAll: () => request<void>('/api/streams/start-all', { method: 'POST' }),
stopAll: () => request<void>('/api/streams/stop-all', { method: 'POST' }),
```

### StreamCard 简化 (`components/StreamCard.tsx`)

无需修改 — 已有开始/停止按钮，状态由 `stream.status` 控制。

## 数据流

```
用户点击「开始」→ POST /api/streams/:id/start
  → StreamHandler.Start()
    → RecorderService.Start()
      → 查找 RecordTask (无则用默认参数)
      → 启动 FFmpeg 进程
      → 更新 DB status = "recording"
      → 回调 MonitorService.NotifyStreamChange()
      → WebSocket 推送实时更新

用户点击「全部开始」→ POST /api/streams/start-all
  → StreamHandler.StartAll()
    → RecorderService.StartAll()
      → 遍历所有流，逐个 Start()
      → 返回结果统计

用户点击「全部停止」→ POST /api/streams/stop-all
  → StreamHandler.StopAll()
    → RecorderService.StopAll()
      → 遍历活跃流，逐个 Stop()
      → 返回结果统计
```

## 兼容性

- 已有的定时任务（cron）功能不受影响
- 已有 `RecordTask` 配置的流仍然按任务设置录制
- 未配置任务的流使用默认参数录制
- 后端 API 不做破坏性变更
