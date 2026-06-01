# 存储设置在网页中的配置

## 目标

在 Web UI 的设置页面中，支持全局切换存储后端（本地存储 / S3 兼容对象存储），并配置 S3 连接参数，无需手动编辑 config.yaml。

## 现状

- `S3Storage` 后端代码已实现（minio-go），但录制流程从未调用 `store.Save()`
- `config.yaml` 已有 `storage.default` 和 `storage.s3.*` 配置字段
- `GET/PUT /api/config` 已存在，但只暴露部分配置，未包含 S3 参数
- 设置页面已有基础框架

## 后端改动

### 1. RecorderService — 加入 SetStore 方法

```go
func (s *RecorderService) SetStore(store StorageBackend)
```

运行时切换存储后端，新录制使用新后端。

### 2. Config — 扩展 Save() 持久化 S3 字段

`config.Save()` 新增保存 `storage.s3.*` 字段（endpoint, access_key, secret_key, bucket, region）。

### 3. S3Storage — 修复 Secure 配置

`NewS3Storage` 将 `Secure: false` 改为 `!strings.HasPrefix(cfg.Endpoint, "localhost") && !strings.HasPrefix(cfg.Endpoint, "127.")`（自动判断），或增加 `secure` 配置项。

### 4. StatusHandler — 扩展 GetConfig / UpdateConfig

**GetConfig** 新增返回字段：

```json
{
  "storage_default": "local",
  "storage_local_path": "./recordings",
  "storage_s3_endpoint": "",
  "storage_s3_bucket": "",
  "storage_s3_region": "",
  // 不返回 access_key / secret_key（安全考虑）
  "storage_s3_configured": true  // 是否配置了完整 S3
}
```

**UpdateConfig** 新增接收字段：

```json
{
  "storage_default": "s3",
  "storage_s3_endpoint": "s3.amazonaws.com",
  "storage_s3_access_key": "xxx",
  "storage_s3_secret_key": "yyy",
  "storage_s3_bucket": "living-recorder",
  "storage_s3_region": "us-east-1"
}
```

UpdateConfig 处理逻辑：

1. 更新 `h.cfg.Storage.*` 字段
2. 如果 `storage_default` 或 S3 配置有变化，调用 `h.recorder.SetStore()` 创建新后端
3. 调用 `h.cfg.Save()` 持久化到 config.yaml

### 5. StatusHandler — 新增 TestS3Connection 接口

```
POST /api/config/test-s3
Body: { endpoint, access_key, secret_key, bucket, region }
Response: { code: 0, message: "ok" } 或 { code: 1, message: "连接失败: ..." }
```

内部调用 minio-go 的 `HealthCheck` 或 `BucketExists` 验证连通性。

### 6. 录制流程 — 接入 store.Save()

在 `watchProcess()` 中录制成功结束时：

```
如果 cfg.Storage.Default == "s3"
  → 调用 store.Save(ctx, outputPath, destPath)
  → 如果成功，删除本地文件
  → 如果失败，标记录制失败但保留本地文件
```

destPath 使用相对于 storage_local_path 的路径。

## 前端改动

### Settings.tsx — 新增存储设置区块

```
┌─────────────────────────────────────┐
│  存储设置                            │
│                                     │
│  ○ 本地存储                          │
│  ○ S3 对象存储                       │
│                                     │
│  ── S3 配置（选中 S3 时显示） ─────── │
│  Endpoint:  [________________]       │
│  Access Key: [________________]       │
│  Secret Key: [________________]       │
│  Bucket:    [________________]       │
│  Region:    [________________]       │
│                                     │
│  [测试连接]  [保存更改]               │
└─────────────────────────────────────┘
```

### API 扩展（api.ts）

```typescript
config: {
  get: () => request<ConfigData>('/api/config'),
  update: (data) => request<void>('/api/config', { method: 'PUT', body: JSON.stringify(data) }),
  testS3: (data) => request<{ message: string }>('/api/config/test-s3', {
    method: 'POST', body: JSON.stringify(data)
  }),
}
```

## 存储类型

| 类型 | 录制写路径 | 录制后处理 |
|------|-----------|-----------|
| local | `storage_local_path/{template}` | 不做额外操作 |
| s3 | `storage_local_path/{template}` | 上传到 S3，删除本地文件 |

## 不涉及

- 按流设置存储（本次只做全局）
- 存储用量统计（已有）
- 回放时从 S3 读取（后续迭代）
