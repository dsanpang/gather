# Gather - 以太坊区块链数据采集系统

一个高性能、企业级的以太坊区块链数据采集系统，支持多种输出格式和动态配置管理。

## ✨ 功能特性

### 🔄 数据采集
- **完整数据采集**：区块、交易、日志、内部交易、状态变更、合约创建
- **上海升级支持**：提款(Withdrawals)数据采集
- **Dencun升级支持**：Blob交易数据采集  
- **重组检测**：自动检测和处理链重组
- **断点续传**：支持采集进度保存和恢复

### ⚡ 高性能架构
- **异步处理**：支持异步文件写入和Kafka消息发送
- **连接池管理**：智能节点负载均衡和故障转移
- **批量处理**：可配置的批次大小优化性能
- **内存优化**：高效的内存使用和垃圾回收策略

### 📤 多种输出方式
- **文件输出**：JSON格式，支持压缩
- **Kafka输出**：实时数据流，支持异步发送
- **数据验证**：完整的数据验证和错误处理

### 🛠️ 配置管理
- **YAML配置**：静态配置文件管理
- **数据库配置**：动态配置管理，支持热更新
- **多节点支持**：Infura、Alchemy、QuickNode等
- **智能解码**：4byte.directory API集成

### 🔧 运维友好
- **RESTful API**：完整的管理和监控接口
- **健康检查**：节点状态监控和故障恢复
- **错误统计**：详细的错误分析和重试机制
- **性能监控**：实时性能指标和统计信息

## 🚀 快速开始

### 1. 环境准备

```bash
# 确保Go版本 1.23+
go version

# 下载依赖
go mod tidy
```

### 2. 配置设置

```bash
# 复制配置模板
cp configs/config.example.yaml configs/config.yaml

# 编辑配置文件，设置你的节点API密钥
nano configs/config.yaml
```

### 3. 编译运行

```bash
# 编译
go build -o bin/gather-collector cmd/gather/main.go
go build -o bin/gather-api cmd/api/main.go

# 或使用Makefile
make build

# 运行采集器
./bin/gather-collector --config configs/config.yaml

# 运行API服务
./bin/gather-api --config configs/config.yaml --port 8080
```

## 📊 使用示例

### 基础数据采集
```bash
# 采集指定区块范围
./bin/gather-collector --config configs/config.yaml \
  --start-block 18000000 \
  --end-block 18001000

# 实时采集最新区块
./bin/gather-collector --config configs/config.yaml --stream
```

### API服务控制
```bash
# 启动API服务
./bin/gather-api --config configs/config.yaml --port 8080

# 检查服务状态
curl http://localhost:8080/health

# 获取采集状态
curl http://localhost:8080/api/v1/status

# 获取统计信息
curl http://localhost:8080/api/v1/stats
```

## ⚙️ 配置说明

### YAML配置示例
```yaml
# 区块链节点配置
blockchain:
  nodes:
    - name: "infura"
      url: "https://mainnet.infura.io/v3/YOUR_API_KEY"
      type: "infura"
      rate_limit: 100
      priority: 1

# 采集器配置
collector:
  workers: 10          # 工作协程数
  batch_size: 100      # 批处理大小
  retry_limit: 3       # 重试次数
  timeout: "30s"       # 超时时间

# 输出配置  
output:
  format: "kafka_async"  # 输出格式
  kafka:
    brokers: ["localhost:9092"]
    topics:
      blocks: "blockchain_blocks"
      transactions: "blockchain_transactions"
```

### 数据库配置（可选）
```yaml
# 启用数据库动态配置
database:
  enabled: true
  dsn: "postgres://user:password@localhost:5432/gather_config"
```

## 🗄️ 数据库配置

### 初始化数据库
```bash
# PostgreSQL
export DB_TYPE=postgresql DB_PASSWORD=your_password
./configs/init_database.sh

# MySQL  
export DB_TYPE=mysql DB_PASSWORD=your_password
./configs/init_database.sh
```

### 配置管理API
```bash
# 获取节点配置
curl http://localhost:8080/api/v1/config/nodes

# 更新节点配置
curl -X PUT http://localhost:8080/api/v1/config/nodes/infura \
  -H "Content-Type: application/json" \
  -d '{"rate_limit": 200}'
```

## 📋 API端点

| 方法 | 端点 | 描述 |
|------|------|------|
| GET | `/health` | 健康检查 |
| GET | `/api/v1/status` | 获取采集器状态 |
| GET | `/api/v1/stats` | 获取统计信息 |
| GET | `/api/v1/errors/stats` | 获取错误统计 |
| GET | `/api/v1/connection/stats` | 获取连接统计 |
| GET | `/api/v1/config/nodes` | 获取节点配置 |
| PUT | `/api/v1/config/nodes/{name}` | 更新节点配置 |
| GET | `/api/v1/config/collector` | 获取采集器配置 |

详细API文档：[docs/API.md](docs/API.md)

## 🏗️ 项目架构

```
gather/
├── cmd/                    # 入口程序
│   ├── gather/            # 采集器主程序  
│   └── api/               # API服务程序
├── internal/              # 内部模块
│   ├── collector/         # 数据采集器
│   ├── output/            # 输出处理
│   ├── config/            # 配置管理
│   ├── decoder/           # 数据解码
│   ├── validation/        # 数据验证
│   ├── errors/            # 错误处理
│   └── connection/        # 连接池管理
├── pkg/                   # 公共包
│   └── models/            # 数据模型
├── configs/               # 配置文件
├── docs/                  # 文档
└── bin/                   # 编译输出
```

## 🔧 命令行参数

### 采集器参数
```bash
gather-collector [flags]

Flags:
  --config string         配置文件路径 (默认: configs/config.yaml)
  --start-block uint      起始区块号
  --end-block uint        结束区块号  
  --stream               启用实时流处理
  --workers int          工作协程数
  --batch-size int       批处理大小
  --output string        输出目录
  --verbose              详细日志输出
```

### API服务参数
```bash
gather-api [flags]

Flags:
  --config string         配置文件路径 (默认: configs/config.yaml)
  --port int             API服务端口 (默认: 8080)
  --verbose              详细日志输出
```

## 📈 性能优化

### 硬件建议
- **CPU**: 8核心以上
- **内存**: 16GB以上  
- **存储**: SSD 500GB以上
- **网络**: 高速稳定连接

### 配置优化
```yaml
collector:
  workers: 20              # CPU核心数的2-3倍
  batch_size: 200          # 根据内存大小调整
  
output:
  format: "kafka_async"    # 异步输出提升性能

decoder:
  cache_size: 50000        # 增大缓存减少API请求
```

## 🚀 部署方式

### Docker部署
```bash
# 构建镜像
docker build -t gather:latest .

# 运行容器
docker-compose up -d
```

### Systemd服务
```bash
# 创建服务文件 (参考DEPLOYMENT.md文档中的完整配置)
sudo nano /etc/systemd/system/gather-collector.service
sudo nano /etc/systemd/system/gather-api.service

# 启用服务
sudo systemctl enable gather-collector gather-api
sudo systemctl start gather-collector gather-api
```

详细部署文档：[docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)

## 🧪 测试

```bash
# 运行所有测试
go test ./...

# 运行测试并生成覆盖率
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# 运行基准测试
go test -bench=. ./...
```

## 📚 文档

- [配置管理指南](docs/CONFIG.md)
- [数据库配置指南](docs/DATABASE_CONFIG.md)  
- [部署指南](docs/DEPLOYMENT.md)
- [API文档](docs/API.md)

## 🔍 故障排除

### 常见问题

1. **节点连接失败**
   ```bash
   # 检查节点状态
   curl -X POST -H "Content-Type: application/json" \
     --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
     https://mainnet.infura.io/v3/YOUR_KEY
   ```

2. **Kafka连接失败**  
   ```bash
   # 检查Kafka服务状态
   docker-compose ps kafka
   
   # 测试连接
   kafka-console-producer --bootstrap-server localhost:9092 --topic test
   ```

3. **内存使用过高**
   ```bash
   # 调整配置减少内存使用
   collector:
     workers: 5
     batch_size: 50
   ```

4. **数据库连接问题**
   ```bash
   # 检查数据库连接
   psql -h localhost -U postgres -d gather_config -c "SELECT 1;"
   ```

## 🤝 贡献

1. Fork 项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 打开 Pull Request

## 📄 许可证

本项目采用 MIT 许可证。

## 🙏 致谢

- [go-ethereum](https://github.com/ethereum/go-ethereum) - 以太坊Go实现
- [Kafka](https://kafka.apache.org/) - 分布式流处理平台
- [Viper](https://github.com/spf13/viper) - Go配置管理库

---

**Gather** - 高性能以太坊数据采集系统 🚀