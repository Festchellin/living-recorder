# Living Recorder

自托管的 IP 摄像机 / 直播流录像管理 Web 应用。支持多路 RTSP/RTMP/FLV/HLS 流的实时预览、录像管理、硬件加速编码，以及灵活的存储后端。

## 功能特性

- **多协议支持** — RTSP、RTMP、FLV、HLS 流接入
- **实时预览** — 多路流同时预览（最多 32 路），支持 MPEG-TS over WebSocket 低延迟播放
- **录像管理** — 启停单路/批量流录制，FFmpeg 后端，支持硬件加速编码
- **预览质量控制** — 每路独立设置分辨率（360p~1080p）、帧率（5~30fps）、CRF 画质
- **分组管理** — 支持嵌套层级的分组，拖拽排序
- **存储后端** — 本地磁盘或 S3 兼容对象存储
- **录制日志** — 完整的录制事件记录，支持按流查询
- **实时监控** — WebSocket 推送流状态、存储用量
- **Docker 部署** — 多阶段构建，镜像体积小，开箱即用

## 下载

从 [GitHub Releases](https://github.com/Festchellin/living-recorder/releases) 下载预编译的二进制文件：

| 文件 | 平台 | 说明 |
|------|------|------|
| `living-recorder-linux-amd64` | Linux (amd64, 64-bit) | Linux x86-64 系统，直接运行 |
| `living-recorder-windows-amd64.exe` | Windows (amd64, 64-bit) | Windows x86-64 系统，双击运行 |

> 运行前请确保系统已安装 FFmpeg（见下方说明）。

### FFmpeg 处理

从 [GitHub Releases](https://github.com/Festchellin/living-recorder/releases) 下载的预编译二进制已**内置 FFmpeg**，启动时自动解压，无需额外安装。

**Docker 镜像**也已内置 FFmpeg：

```bash
docker pull ghcr.io/festchellin/living-recorder:1.0.0
docker run -p 8080:8080 ghcr.io/festchellin/living-recorder:1.0.0
```

**本地源码编译**需自行安装 FFmpeg：

| 系统 | 命令 |
|------|------|
| Linux | `sudo apt install ffmpeg` |
| macOS | `brew install ffmpeg` |
| Windows | 下载 [ffmpeg.org](https://ffmpeg.org/download.html) 并添加到 PATH |

## 技术栈

| 层 | 技术 |
|-------|-----------|
| 后端 | Go 1.25, Gin, GORM, SQLite |
| 前端 | React 18, TypeScript, Tailwind CSS, Vite |
| 视频处理 | FFmpeg, mpegts.js |
| 容器化 | Docker（多阶段构建） |

## 快速开始

### 使用 Docker（推荐）

```bash
docker compose up -d --build
```

访问 http://localhost:8080

### 手动构建

```bash
# 构建前端
cd frontend && npm install && npm run build

# 嵌入前端到后端
cp -r frontend/dist backend/embed/

# 启动后端
cd backend && go run .
```

## 配置

配置文件为 `config.yaml`，支持以下主要选项：

| 配置项 | 默认值 | 说明 |
|-----------|---------|-------------|
| `server.port` | `8080` | HTTP 监听端口 |
| `database.path` | `./data/living-recorder.db` | SQLite 数据库路径 |
| `ffmpeg.path` | `ffmpeg` | FFmpeg 可执行文件路径 |
| `recorder.max_parallel` | `10` | 最大并发录制数 |
| `recorder.restart_on_failure` | `3` | 失败自动重试次数 |
| `storage.default` | `local` | 存储后端：`local` 或 `s3` |
| `storage.local.path` | `./recordings` | 本地录像存储路径 |

环境变量通过 Viper 支持覆盖。

## API

所有接口以 `/api` 为前缀，返回 `{ "code": 0, "data": ... }` 格式。

### 流管理

| 方法 | 路径 | 说明 |
|--------|------|-------------|
| `GET` | `/api/streams` | 列出所有流 |
| `POST` | `/api/streams` | 创建流 |
| `PUT` | `/api/streams/:id` | 更新流 |
| `DELETE` | `/api/streams/:id` | 删除流 |
| `POST` | `/api/streams/:id/start` | 开始录制 |
| `POST` | `/api/streams/:id/stop` | 停止录制 |
| `GET` | `/api/streams/:id/probe` | 探测流连通性 |
| `GET` | `/api/streams/:id/probe-info` | 获取流元数据 |
| `POST` | `/api/streams/start-all` | 批量启动所有流 |
| `POST` | `/api/streams/stop-all` | 批量停止所有流 |

### 预览

| 方法 | 路径 | 说明 |
|--------|------|-------------|
| `GET` | `/api/preview/:id/ws` | WebSocket 实时预览 |

### 分组

| 方法 | 路径 | 说明 |
|--------|------|-------------|
| `GET` | `/api/groups` | 列出分组 |
| `POST` | `/api/groups` | 创建分组 |
| `PUT` | `/api/groups/reorder` | 排序分组 |
| `PUT` | `/api/groups/:id` | 更新分组 |
| `DELETE` | `/api/groups/:id` | 删除分组 |

### 系统

| 方法 | 路径 | 说明 |
|--------|------|-------------|
| `GET` | `/api/status` | 系统状态 |
| `GET` | `/api/config` | 获取配置 |
| `PUT` | `/api/config` | 更新配置 |
| `GET` | `/api/logs/recent` | 最近录像日志 |
| `GET` | `/api/ws` | WebSocket 实时状态推送 |

## Docker 镜像

镜像托管在 GitHub Container Registry：

```bash
docker pull ghcr.io/festchellin/living-recorder:1.0.0
```

### 多阶段构建

- **Frontend 阶段**: Node 20 Alpine — 安装依赖、构建前端
- **Backend 阶段**: Golang latest — 编译静态链接二进制，嵌入前端资源
- **Runtime 阶段**: Alpine 3.20 — 仅包含运行时最小依赖 + FFmpeg

### 目录挂载

| 主机路径 | 容器路径 | 用途 |
|-----------|----------|------|
| `./data` | `/app/data` | SQLite 数据库 |
| `./recordings` | `/app/recordings` | 录像文件存储 |
| `./build/config.yaml` | `/app/config.yaml` | 运行时配置 |

## 开发

```bash
git clone https://github.com/Festchellin/living-recorder.git
cd living-recorder

# 前端开发
cd frontend
npm install
npm run dev

# 后端开发
cd backend
go run .
```

## License

MIT
