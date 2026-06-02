# 录制可靠性改进设计方案

## 概述

针对 Living Recorder 项目的录制与预览功能进行多项可靠性改进，核心变更包括：

1. 预览共享 FFmpeg 进程（Ring Buffer 扇出）
2. 录制中健康检测（静帧/僵死探测）
3. 磁盘空间预检
4. 重试计数器持久化
5. 预览 WebSocket 读超时
6. S3 上传异步化

---

## 1. 预览共享 FFmpeg 进程

### 问题

当前每个 WebSocket 预览窗口独立启动一个 `ffmpeg` 进程进行转码，32 格多画面预览 = 32 个 ffmpeg 进程，CPU 和带宽浪费严重。

### 设计

新增 `PreviewManager` 服务，每条流维护一个 `PreviewStream` 实例。

#### PreviewManager

```
PreviewManager
├── streams map[uint]*PreviewStream
├── Subscribe(streamID, conn, config) → error
├── Unsubscribe(streamID, conn) → error
└── Stop()
```

#### PreviewStream（单条流）

```
PreviewStream
├── cmd *exec.Cmd          ← 一个 ffmpeg 进程
├── stdout io.ReadCloser   ← ffmpeg stdout pipe
├── cancel context.CancelFunc
├── refCount int32          ← 引用计数
├── subscribers map[*websocket.Conn]chan []byte
├── readBuf []byte          ← 共享读缓冲 64KB
└── mu sync.RWMutex
```

#### 生命周期

1. **首次订阅**：`refCount` 从 0 → 1，启动 ffmpeg，启动读 goroutine
2. **读 goroutine**：循环读 stdout（64KB 块），非阻塞广播到所有 subscriber channel
   - `subscriber.ch <- chunk`，写不进去的 drop
3. **订阅者 goroutine**：从 channel 读数据 → `conn.WriteMessage(BinaryMessage)`
4. **取消订阅**：关闭 channel，从 map 移除，`refCount--`
   - 若回到 0，kill ffmpeg 进程，清理资源
5. **WS 断连**：read deadline 超时 → 自动触发取消订阅

#### 缓冲策略

- 每个 subscriber channel 容量为 **8**（约 512KB 缓冲）
- 慢客户端自动丢包，不影响其他订阅者
- 新加入订阅者从当前时间点开始接收（下一个 chunk 开始）

#### WebSocket 读超时

- `conn.SetReadDeadline(time.Now().Add(60 * time.Second))`
- `SetPongHandler` 重置 deadline
- 服务端每 30s 发 Ping

### 修改文件

| 文件 | 改动 |
|------|------|
| `backend/services/preview.go` | 新增 PreviewManager + PreviewStream |
| `backend/services/preview_test.go` | 单元测试 |
| `backend/handlers/stream.go` | PreviewWS 改为调用 PreviewManager.Subscribe/Unsubscribe |

---

## 2. 录制中健康检测

### 问题

当前只在 FFmpeg 进程退出时才感知录制失败。网络闪断或摄像头推送停止时 FFmpeg 不一定退出，导致"录制中"状态但实际无数据写入。

### 设计

`RecorderService` 启动一个后台健康检查 goroutine，周期检查正在录制的流。

#### 检测逻辑

```
每 RecorderConfig.HealthCheckInterval（默认 60s）：
  遍历所有活跃的 StreamProcess：
    忽略 userStopped 的
    检查输出文件最近修改时间
    若超过 HealthCheckTimeout（默认 120s）无写入：
      → 日志警告
      → Kill ffmpeg 进程
      → watchProcess 捕获退出 → 自动触发重试
```

#### 文件修改

| 文件 | 改动 |
|------|------|
| `backend/config/config.go` | RecorderConfig 加 HealthCheckInterval、HealthCheckTimeout |
| `backend/config/config.yaml` | 加默认值 |
| `backend/services/recorder.go` | 加 StartHealthCheck()、checkHealth() |
| `backend/main.go` | 初始化后调用 StartHealthCheck() |

---

## 3. 磁盘空间预检

### 问题

磁盘满时 FFmpeg 写入失败但前期无提示，用户发现时已丢失数据。

### 设计

`Start()` 方法开头增加磁盘空间检查，使用 `syscall.Statfs` 获取当前分区可用空间。

```
const minDiskSpaceGB = 1.0

func (s *RecorderService) getDiskFreeGB(path string) float64

func (s *RecorderService) Start(...) error {
    free := getDiskFreeGB(storageLocalPath)
    if free >= 0 && free < minDiskSpaceGB {
        return fmt.Errorf("磁盘空间不足: %.1fGB 可用，需至少 %.1fGB", free, minDiskSpaceGB)
    }
    // ... 原有逻辑
}
```

### 文件修改

| 文件 | 改动 |
|------|------|
| `backend/services/recorder.go` | Start() 开头加磁盘检查 |

---

## 4. 重试计数器持久化

### 问题

当前 `retryCounts sync.Map` 存储在内存中，进程重启后丢失。一条已耗尽重试次数的流在重启后会重新尝试，反复失败。

### 设计

Stream 模型加 `RetryCount` 字段，直接读写 DB。

```go
// models/stream.go
type Stream struct {
    // ... 原有字段
    RetryCount int `gorm:"default:0" json:"retry_count"`
}
```

去掉 `RecorderService.retryCounts sync.Map`，改用 DB 操作：

```go
func (s *RecorderService) getRetryCount(streamID uint) int
func (s *RecorderService) setRetryCount(streamID uint, count int)
```

### 文件修改

| 文件 | 改动 |
|------|------|
| `backend/models/stream.go` | 加 RetryCount 字段 |
| `backend/services/recorder.go` | 替换 retryCounts sync.Map 为 DB 操作 |

---

## 5. 预览 WebSocket 读超时

### 问题

客户端断连不干净时，`PreviewWS` 的 `conn.ReadMessage()` 一直阻塞，FFmpeg 进程和 goroutine 泄漏。

### 设计

- `conn.SetReadDeadline(time.Now().Add(60 * time.Second))`
- 注册 `PongHandler` 重置 deadline
- 服务端 goroutine 每 30s 发 `PingMessage`
- 超时后 `ReadMessage()` 返回错误 → 触发清理流程

### 文件修改

| 文件 | 改动 |
|------|------|
| `backend/handlers/stream.go` | PreviewWS 加 ping/pong + read deadline |

---

## 6. S3 上传异步化

### 问题

`watchProcess` 中 S3 上传是同步阻塞的。大文件（数 GB）上传耗时数分钟，阻塞后续日志写入和重试逻辑。

### 设计

将 S3 上传改为 `go goroutine` 异步执行。主流程直接走成功路径，上传失败时只打印日志（文件仍保留在本地）。

```go
if status == "success" && s.storageType == "s3" && fileSize > 0 {
    destPath := ...
    go func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
        defer cancel()
        if err := s.store.Save(ctx, sp.OutputPath, destPath); err != nil {
            log.Printf("[recorder] stream %d: s3 upload failed: %v", sp.StreamID, err)
            return
        }
        os.Remove(sp.OutputPath)
    }()
}
```

### 文件修改

| 文件 | 改动 |
|------|------|
| `backend/services/recorder.go` | watchProcess 中 S3 上传改为 goroutine |

---

## 影响范围

### 向后兼容性

- 所有改动均为新增行为，不破坏现有 API 接口
- Stream 模型加字段，gorm auto migrate 会自动增加列
- PreviewWebSocket URL 不变，前端无需修改
- 配置项增加使用默认值，不影响现有配置

### 前端影响

- **无**—— 预览的 WS URL 和数据结构不变

### 后端测试

- `backend/services/preview_test.go`：PreviewManager 的订阅/取消订阅/多客户端/进程生命周期
- 健康检测测试：mock StreamProcess，验证 kill 触发重试
- 磁盘检查测试：验证空间不足时 Start 返回错误

---

## 实施顺序

1. PreviewManager（最大改动，核心基础）
2. 重试计数器持久化（小改，独立）
3. 磁盘空间预检（小改，独立）
4. 预览 WebSocket 超时（小改，独立）
5. 录制健康检测（依赖重试机制）
6. S3 上传异步化（小改，独立）
