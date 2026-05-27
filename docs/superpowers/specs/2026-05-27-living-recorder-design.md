# Living Recorder — 直播流录制工具设计文档

## 概述

独立 Web 应用，通过 FFmpeg 录制 RTSP/RTMP/FLV/HLS 等直播流到本地或对象存储，支持多路并发、录制参数自定义、计划录制。

## 技术栈

- **后端**: Go 1.22 + Gin + GORM + SQLite
- **前端**: React 18 + Vite + shadcn/ui + Tailwind CSS
- **录制**: FFmpeg 子进程管理
- **存储**: 本地磁盘 / S3 兼容对象存储（MinIO 等）
- **实时**: WebSocket (gorilla/websocket)

## 项目结构

```
living-recorder/
├── backend/
│   ├── main.go                  # 入口
│   ├── config/
│   │   ├── config.go            # Viper 配置加载
│   │   └── config.yaml          # 配置文件
│   ├── database/
│   │   └── database.go          # GORM 初始化 + AutoMigrate
│   ├── models/
│   │   ├── stream.go            # 直播流模型
│   │   ├── record_task.go       # 录制任务模型
│   │   └── record_log.go        # 录制日志模型
│   ├── services/
│   │   ├── recorder.go          # 录制引擎 — FFmpeg 进程管理
│   │   ├── scheduler.go         # 调度器 — cron 计划录制
│   │   ├── storage.go           # 存储后端接口 + 实现
│   │   └── monitor.go           # 监控 — 状态采集 + WebSocket 推送
│   ├── handlers/
│   │   ├── stream.go            # 流 CRUD 处理器
│   │   ├── task.go              # 任务 CRUD 处理器
│   │   ├── status.go            # 状态查询处理器
│   │   └── websocket.go         # WebSocket 处理器
│   └── routes/
│       └── routes.go            # 路由注册
├── frontend/
│   ├── src/
│   │   ├── pages/
│   │   │   ├── Dashboard.tsx
│   │   │   ├── Streams.tsx
│   │   │   ├── StreamDetail.tsx
│   │   │   ├── Tasks.tsx
│   │   │   └── Settings.tsx
│   │   ├── components/
│   │   ├── hooks/
│   │   └── lib/
│   └── ...
└── docker-compose.yml           # 可选(MinIO 等)
```

## 数据模型

### Stream

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint PK | |
| Name | string | 流名称 |
| URL | string | 流地址 |
| Protocol | string | 协议: rtsp/rtmp/flv/hls |
| Enabled | bool | 启用状态 |
| Status | string | idle/recording/error |
| CreatedAt | time | |
| UpdatedAt | time | |

### RecordTask

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint PK | |
| StreamID | uint FK | 关联流 |
| OutputTemplate | string | 文件名模板 |
| SegmentSec | int | 分段秒数, 0=不分段 |
| VideoCodec | string | 视频编码 copy/libx264/h264_nvenc |
| VideoBitrate | string | 视频码率 |
| Framerate | int | 帧率, 0=原帧率 |
| Resolution | string | 分辨率, 空=原分辨率 |
| AudioCodec | string | 音频编码 copy/aac |
| AudioBitrate | string | 音频码率 |
| StorageType | string | local/s3/minio |
| StorageConfig | json | 存储配置 |
| ScheduleCron | string | cron 表达式, 空=手动 |
| ScheduleDuration | int | 每次录制秒数, 0=持续 |
| LoopRecord | bool | 循环录制 |
| Enabled | bool | 启用 |
| CreatedAt | time | |

### RecordLog

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | uint PK | |
| StreamID | uint FK | |
| FilePath | string | 录制文件路径 |
| FileSize | int64 | 文件大小(字节) |
| Duration | int | 录制时长(秒) |
| Status | string | success/failed |
| ErrorMsg | string | 错误信息 |
| StartedAt | time | |
| EndedAt | time | |

## API 路由

```
GET    /api/streams               # 流列表
POST   /api/streams               # 添加流
GET    /api/streams/:id           # 流详情
PUT    /api/streams/:id           # 编辑流
DELETE /api/streams/:id           # 删除流

POST   /api/streams/:id/start     # 开始录制
POST   /api/streams/:id/stop      # 停止录制
GET    /api/streams/:id/logs      # 录制日志

GET    /api/tasks                 # 录制任务列表
POST   /api/tasks                 # 创建录制任务
PUT    /api/tasks/:id             # 编辑任务
DELETE /api/tasks/:id             # 删除任务

GET    /api/config                # 获取全局配置
PUT    /api/config                # 更新全局配置
GET    /api/status                # 全局状态概览
GET    /api/ws                    # WebSocket 实时推送
```

## 核心服务

### Recorder Service

- 维护 `map[uint]*FFmpegProcess` 管理所有录制进程
- `Start(task)` → 构建 FFmpeg 命令参数 → `exec.CommandContext` 启动
- `Stop(streamID)` → 向 FFmpeg 发送 `q` → 等待退出
- 自动重启：进程异常退出时按配置次数自动重试
- 状态变更时通过 monitor 广播

### Scheduler Service

- 启动时加载所有 `enabled=true` + 有 cron 表达式的 RecordTask
- 使用 `robfig/cron` 注册定时任务
- 触发时调用 Recorder.Start()，到达 duration 后调用 Stop()
- 支持 loop_record 模式：分段录制不间断

### Storage Backend

```go
type StorageBackend interface {
    Save(ctx, srcPath, destPath string) error
    Delete(ctx, path string) error
    List(ctx, prefix string) ([]FileInfo, error)
    GetURL(ctx, path string) (string, error)
}
```

内置实现：LocalStorage（本地文件移动）、S3Storage（minio-go s3 客户端）

### Monitor Service

- 每 `health_check_interval` 秒检查所有运行中进程
- 通过 WebSocket push 实时状态（单流状态、磁盘/存储空间、录制进度）
- 录制完成/失败时生成 RecordLog

## FFmpeg 命令构造

```bash
# 通用模板
ffmpeg [协议参数] -i [URL] [视频参数] [音频参数] [分段参数] [输出]

# RTSP 示例
ffmpeg -rtsp_transport tcp -i rtsp://...
  -c:v libx264 -preset medium -crf 23
  -b:v 2000k -r 25 -s 1920x1080
  -c:a aac -b:a 128k
  -f segment -segment_time 600 -reset_timestamps 1
  -strftime 1 "/output/{name}/%Y%m%d_%H%M%S.mp4"

# 纯转发（不转码，性能最优）
ffmpeg -rtsp_transport tcp -i rtsp://...
  -c copy
  -f segment -segment_time 600
  -strftime 1 "/output/{name}/%Y%m%d_%H%M%S.mp4"
```

## 前端页面

| 路由 | 页面 | 内容 |
|------|------|------|
| / | Dashboard | 运行中流数、存储空间、最近录制列表 |
| /streams | Streams | 流列表 + 添加/编辑/启停操作 |
| /streams/:id | StreamDetail | 录制日志、状态趋势图 |
| /tasks | Tasks | 录制任务管理 + 参数配置表单 |
| /settings | Settings | ffmpeg 路径、默认存储、最大并行数 |

每个流卡片显示：协议标签、状态指示灯（绿/黄/红）、操作按钮。

## 录制计划流程

1. 用户创建 RecordTask，配置 cron + duration
2. Scheduler 启动时加载所有启用的计划任务
3. 到点时 Scheduler 调用 Recorder.Start()
4. 到达 duration 后自动 Stop()，若 loop_record=true 则继续下一轮
5. 生成 RecordLog 记录结果

## 配置

```yaml
server:
  port: "8080"
  mode: debug

database:
  driver: sqlite
  path: ./data/living-recorder.db

ffmpeg:
  path: ffmpeg

storage:
  default: local
  local:
    path: ./recordings
  s3:
    endpoint: ""
    access_key: ""
    secret_key: ""
    bucket: ""
    region: ""

recorder:
  max_parallel: 10
  restart_on_failure: 3
  health_check_interval: 30
```

## 非功能性需求

- 单二进制部署，含内嵌前端
- 录制进程崩溃自动恢复
- FFmpeg 输出实时日志记录
- 支持 10+ 路并发录制
