# Gather API 文档

Gather 提供了完整的 RESTful API 用于管理和监控以太坊数据采集系统。

## 基础信息

- **Base URL**: `http://localhost:8080`
- **API 版本**: v1
- **内容类型**: `application/json`

## 认证

当前版本暂不需要认证，建议在生产环境中部署时添加适当的认证机制。

## 错误响应格式

所有错误响应都遵循以下格式：

```json
{
  "error": "错误信息",
  "code": "ERROR_CODE",
  "timestamp": "2024-01-01T12:00:00Z"
}
```

## API 端点

### 1. 健康检查

检查服务是否正常运行。

**请求**
```
GET /health
```

**响应**
```json
{
  "status": "healthy",
  "timestamp": 1704067200,
  "service": "gather-api"
}
```

### 2. 获取采集器状态

获取当前采集器的运行状态。

**请求**
```
GET /api/v1/status
```

**响应**
```json
{
  "running": true,
  "paused": false,
  "status": "running"
}
```

**状态说明**
- `stopped`: 采集器已停止
- `running`: 采集器正在运行
- `paused`: 采集器已暂停

### 3. 启动采集任务

启动数据采集任务。

**请求**
```
POST /api/v1/start
Content-Type: application/json

{
  "start_block": 19000000,
  "end_block": 19001000,
  "workers": 5,
  "batch_size": 50,
  "stream": false
}
```

**参数说明**
- `start_block` (uint64): 起始区块号
- `end_block` (uint64): 结束区块号，0表示持续采集
- `workers` (int): 工作协程数量，默认为配置中的值
- `batch_size` (int): 批次大小，默认为配置中的值
- `stream` (bool): 是否为流式采集模式

**响应**
```json
{
  "message": "采集任务已启动",
  "status": "started"
}
```

### 4. 停止采集任务

停止当前运行的采集任务。

**请求**
```
POST /api/v1/stop
```

**响应**
```json
{
  "message": "采集任务已停止",
  "status": "stopped"
}
```

### 5. 暂停采集任务

暂停当前运行的采集任务。

**请求**
```
POST /api/v1/pause
```

**响应**
```json
{
  "message": "采集任务已暂停",
  "status": "paused"
}
```

### 6. 恢复采集任务

恢复已暂停的采集任务。

**请求**
```
POST /api/v1/resume
```

**响应**
```json
{
  "message": "采集任务已恢复",
  "status": "resumed"
}
```

### 7. 获取配置信息

获取当前系统配置。

**请求**
```
GET /api/v1/config
```

**响应**
```json
{
  "config": {
    "blockchain": {
      "nodes": [
        {
          "name": "infura",
          "url": "https://mainnet.infura.io/v3/...",
          "type": "infura",
          "rate_limit": 100,
          "priority": 1
        }
      ]
    },
    "collector": {
      "workers": 10,
      "batch_size": 100,
      "retry_limit": 3,
      "timeout": "30s"
    },
    "output": {
      "format": "kafka_async",
      "directory": "./outputs"
    }
  }
}
```

### 8. 更新配置

动态更新区块链节点配置。

**请求**
```
PUT /api/v1/config
Content-Type: application/json

{
  "nodes": [
    {
      "name": "new_node",
      "url": "https://eth-mainnet.alchemyapi.io/v2/...",
      "type": "alchemy",
      "rate_limit": 100,
      "priority": 1
    }
  ]
}
```

**响应**
```json
{
  "message": "配置已更新",
  "config": {
    // 更新后的完整配置
  }
}
```

### 9. 获取统计信息

获取采集系统的统计信息。

**请求**
```
GET /api/v1/stats
```

**响应**
```json
{
  "running": true,
  "paused": false,
  "status": "running",
  "uptime": "2h30m15s",
  "processed_blocks": 1500,
  "processed_transactions": 45000,
  "processing_rate": 12.5,
  "error_count": 3,
  "last_block": 19001500
}
```

### 10. 获取日志

获取系统日志信息。

**请求**
```
GET /api/v1/logs?level=error&page=1&pageSize=20
```

**查询参数**
- `level` (string): 日志级别过滤 (debug, info, warn, error)
- `page` (int): 页码，默认为1
- `pageSize` (int): 每页条数，默认为20

**响应**
```json
{
  "logs": [
    {
      "timestamp": "2024-01-01T12:00:00Z",
      "level": "error",
      "message": "网络连接失败",
      "component": "collector",
      "context": {
        "block_number": 19001000,
        "error_code": "CONNECTION_FAILED"
      }
    }
  ],
  "total": 150,
  "page": 1,
  "pageSize": 20,
  "level": "error"
}
```

### 11. 清空日志

清空系统日志记录。

**请求**
```
DELETE /api/v1/logs
```

**响应**
```json
{
  "message": "日志已清空"
}
```

### 12. 获取节点状态

获取所有配置节点的状态信息。

**请求**
```
GET /api/v1/nodes
```

**响应**
```json
{
  "nodes": [
    {
      "name": "infura",
      "type": "infura",
      "url": "https://mainnet.infura.io/v3/...",
      "available": true,
      "priority": 1,
      "last_check": "2024-01-01T12:00:00Z",
      "response_time": "250ms",
      "error_rate": 0.02
    }
  ],
  "total": 2,
  "healthy": 2,
  "message": "多节点支持已启用"
}
```

## 扩展端点

### 验证器状态

获取数据验证器的状态和统计信息。

**请求**
```
GET /api/v1/validator/stats
```

**响应**
```json
{
  "strict_mode": true,
  "total_validations": 50000,
  "validation_errors": 25,
  "error_rate": 0.0005,
  "rules": {
    "block": {
      "validations": 1500,
      "errors": 2
    },
    "transaction": {
      "validations": 45000,
      "errors": 15
    }
  }
}
```

### 连接池状态

获取以太坊节点连接池状态。

**请求**
```
GET /api/v1/connection/stats
```

**响应**
```json
{
  "pools": {
    "infura": {
      "max_size": 10,
      "current_size": 8,
      "available": 5,
      "in_use": 3,
      "is_healthy": true,
      "last_check": "2024-01-01T12:00:00Z"
    }
  },
  "total_connections": 15,
  "available_connections": 8
}
```

### 错误统计

获取详细的错误统计信息。

**请求**
```
GET /api/v1/errors/stats
```

**响应**
```json
{
  "total_errors": 150,
  "errors_by_type": {
    "Network": 80,
    "Validation": 25,
    "Kafka": 15
  },
  "errors_by_severity": {
    "Low": 100,
    "Medium": 35,
    "High": 15,
    "Critical": 0
  },
  "error_rate_per_hour": 5.2,
  "last_error": {
    "timestamp": "2024-01-01T12:00:00Z",
    "type": "Network",
    "message": "连接超时",
    "component": "collector"
  }
}
```

## 使用示例

### 启动批量采集

```bash
curl -X POST http://localhost:8080/api/v1/start \
  -H "Content-Type: application/json" \
  -d '{
    "start_block": 19000000,
    "end_block": 19001000,
    "workers": 5,
    "batch_size": 100,
    "stream": false
  }'
```

### 启动流式采集

```bash
curl -X POST http://localhost:8080/api/v1/start \
  -H "Content-Type: application/json" \
  -d '{
    "start_block": 0,
    "end_block": 0,
    "stream": true
  }'
```

### 获取实时状态

```bash
curl http://localhost:8080/api/v1/status
```

### 监控日志

```bash
# 获取错误日志
curl "http://localhost:8080/api/v1/logs?level=error&pageSize=50"

# 获取最新日志
curl "http://localhost:8080/api/v1/logs?page=1&pageSize=10"
```

## 错误码参考

| 错误码 | HTTP状态码 | 描述 |
|--------|------------|------|
| `COLLECTOR_ALREADY_RUNNING` | 409 | 采集器已在运行 |
| `COLLECTOR_NOT_RUNNING` | 409 | 采集器未在运行 |
| `INVALID_BLOCK_RANGE` | 400 | 无效的区块范围 |
| `CONFIG_VALIDATION_FAILED` | 400 | 配置验证失败 |
| `INTERNAL_SERVER_ERROR` | 500 | 内部服务器错误 |

## 注意事项

1. **并发控制**: API 会自动处理并发请求，但同时只能运行一个采集任务
2. **资源监控**: 建议定期检查 `/api/v1/stats` 端点监控系统资源使用情况
3. **错误处理**: 所有API调用都应该包含适当的错误处理逻辑
4. **日志管理**: 系统日志会自动轮转，但建议定期清理以避免占用过多存储空间
5. **配置热更新**: 部分配置更新需要重启采集任务才能生效

## 开发指南

如需扩展API功能，请参考 `internal/api/server.go` 文件，所有新的端点都应该遵循现有的模式和错误处理机制。