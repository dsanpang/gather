# Gather 部署指南

本文档介绍如何在不同环境中部署 Gather 以太坊数据采集系统。

## 系统要求

### 最低配置
- **CPU**: 2核心
- **内存**: 4GB RAM
- **存储**: 100GB 可用空间
- **网络**: 稳定的互联网连接

### 推荐配置
- **CPU**: 8核心或更多
- **内存**: 16GB RAM 或更多
- **存储**: 500GB SSD
- **网络**: 高速稳定连接

### 软件依赖
- Go 1.23.0 或更高版本
- Kafka（如使用 Kafka 输出）
- PostgreSQL（可选，用于配置管理）

## 预备步骤

### 1. 获取以太坊节点访问

你需要至少一个以太坊节点的访问权限：

#### Infura
1. 注册 [Infura](https://infura.io/) 账户
2. 创建新项目
3. 获取项目ID

#### Alchemy
1. 注册 [Alchemy](https://alchemy.com/) 账户
2. 创建新应用
3. 获取API密钥

#### 自建节点
如果你有自己的以太坊节点，确保它支持标准的JSON-RPC接口。

### 2. 设置 Kafka（可选）

如果使用 Kafka 作为输出方式：

#### 使用 Docker 快速设置
```bash
# 创建 docker-compose.yml
version: '3'
services:
  zookeeper:
    image: confluentinc/cp-zookeeper:latest
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181
      ZOOKEEPER_TICK_TIME: 2000

  kafka:
    image: confluentinc/cp-kafka:latest
    depends_on:
      - zookeeper
    ports:
      - "9092:9092"
    environment:
      KAFKA_BROKER_ID: 1
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://localhost:9092
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1

# 启动服务
docker-compose up -d
```

#### 创建主题
```bash
# 创建必要的 Kafka 主题
docker exec -it <kafka-container> kafka-topics --create --topic blockchain_blocks --bootstrap-server localhost:9092 --partitions 3 --replication-factor 1
docker exec -it <kafka-container> kafka-topics --create --topic blockchain_transactions --bootstrap-server localhost:9092 --partitions 3 --replication-factor 1
docker exec -it <kafka-container> kafka-topics --create --topic blockchain_logs --bootstrap-server localhost:9092 --partitions 3 --replication-factor 1
```

### 3. 设置 PostgreSQL（可选）

如果使用数据库进行配置管理：

```sql
-- 创建数据库
CREATE DATABASE gather_config;

-- 创建配置表
\c gather_config;

CREATE TABLE blockchain_nodes (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    url TEXT NOT NULL,
    node_type VARCHAR(50) NOT NULL,
    rate_limit INTEGER DEFAULT 100,
    priority INTEGER DEFAULT 1,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE collector_config (
    id SERIAL PRIMARY KEY,
    config_key VARCHAR(100) NOT NULL UNIQUE,
    config_value TEXT NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE output_config (
    id SERIAL PRIMARY KEY,
    config_key VARCHAR(100) NOT NULL UNIQUE,
    config_value TEXT NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE kafka_topics (
    id SERIAL PRIMARY KEY,
    data_type VARCHAR(50) NOT NULL,
    topic_name VARCHAR(100) NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 插入示例配置
INSERT INTO blockchain_nodes (name, url, node_type, rate_limit, priority) VALUES
('infura', 'https://mainnet.infura.io/v3/YOUR_PROJECT_ID', 'infura', 100, 1),
('alchemy', 'https://eth-mainnet.alchemyapi.io/v2/YOUR_API_KEY', 'alchemy', 100, 2);

INSERT INTO kafka_topics (data_type, topic_name) VALUES
('blocks', 'blockchain_blocks'),
('transactions', 'blockchain_transactions'),
('logs', 'blockchain_logs'),
('internal_transactions', 'blockchain_internal_transactions'),
('state_changes', 'blockchain_state_changes'),
('contract_creations', 'blockchain_contract_creations'),
('reorg_notifications', 'blockchain_reorg_notifications');
```

## 安装和配置

### 1. 下载和编译

```bash
# 克隆代码仓库
git clone <repository-url>
cd gather

# 编译主程序
go build -o gather-collector cmd/gather/main.go
go build -o gather-api cmd/api/main.go

# 或者使用 Makefile（如果有）
make build
```

### 2. 配置设置

Gather支持两种配置方式：**YAML配置文件** 和 **数据库配置**

#### YAML配置（推荐）
```bash
# 复制配置模板  
cp configs/config.example.yaml configs/config.yaml

# 编辑配置文件，设置你的API密钥和配置
nano configs/config.yaml
```

#### 配置示例
```yaml
# configs/config.yaml
blockchain:
  nodes:
    - name: "infura"
      url: "https://mainnet.infura.io/v3/YOUR_PROJECT_ID"
      type: "infura"
      rate_limit: 100
      priority: 1
    - name: "alchemy"
      url: "https://eth-mainnet.alchemyapi.io/v2/YOUR_API_KEY"
      type: "alchemy"
      rate_limit: 100
      priority: 2

collector:
  workers: 10
  batch_size: 100
  retry_limit: 3
  timeout: "30s"

output:
  format: "kafka_async"
  directory: "./outputs"
  kafka:
    brokers:
      - "localhost:9092"
    topics:
      blocks: "blockchain_blocks"
      transactions: "blockchain_transactions"
      logs: "blockchain_logs"

decoder:
  fourbyte_api_url: "https://www.4byte.directory/api/v1/signatures/"
  api_timeout: "5s"
  enable_cache: true
  cache_size: 10000

logging:
  level: "info"
  format: "json"
  output: "stdout"
```

#### 数据库配置（可选）
```yaml
# 启用数据库配置，实现动态配置管理
database:
  enabled: true
  dsn: "postgres://user:password@localhost:5432/gather_config"
```

**配置优势：**
- ✅ **YAML简单直接**：所有配置在一个文件中
- ✅ **数据库动态**：运维友好，支持热更新配置  
- ✅ **安全**：敏感信息不入代码库

### 3. 权限设置

```bash
# 确保二进制文件可执行
chmod +x gather-collector gather-api

# 创建必要的目录
mkdir -p outputs data logs

# 设置适当的权限
chmod 755 outputs data logs
```

## 部署方式

### 方式一：直接运行

#### 启动数据采集器
```bash
# 前台运行
./gather-collector --config configs/config.yaml

# 后台运行
nohup ./gather-collector --config configs/config.yaml > logs/collector.log 2>&1 &
```

#### 启动 API 服务器
```bash
# 前台运行
./gather-api --config configs/config.yaml --port 8080

# 后台运行
nohup ./gather-api --config configs/config.yaml --port 8080 > logs/api.log 2>&1 &
```

### 方式二：使用 systemd

#### 创建服务文件

**数据采集器服务**
```ini
# /etc/systemd/system/gather-collector.service
[Unit]
Description=Gather Ethereum Data Collector
After=network.target

[Service]
Type=simple
User=gather
Group=gather
WorkingDirectory=/opt/gather
ExecStart=/opt/gather/gather-collector --config /opt/gather/configs/config.yaml
Restart=always
RestartSec=10
Environment=PATH=/usr/local/bin:/usr/bin:/bin
EnvironmentFile=/opt/gather/.env

# 资源限制
LimitNOFILE=65536
LimitMEMLOCK=infinity

[Install]
WantedBy=multi-user.target
```

**API 服务器服务**
```ini
# /etc/systemd/system/gather-api.service
[Unit]
Description=Gather API Server
After=network.target

[Service]
Type=simple
User=gather
Group=gather
WorkingDirectory=/opt/gather
ExecStart=/opt/gather/gather-api --config /opt/gather/configs/config.yaml --port 8080
Restart=always
RestartSec=5
Environment=PATH=/usr/local/bin:/usr/bin:/bin
EnvironmentFile=/opt/gather/.env

[Install]
WantedBy=multi-user.target
```

#### 启动服务
```bash
# 重新加载 systemd
sudo systemctl daemon-reload

# 启用服务
sudo systemctl enable gather-collector gather-api

# 启动服务
sudo systemctl start gather-collector gather-api

# 检查状态
sudo systemctl status gather-collector gather-api

# 查看日志
sudo journalctl -u gather-collector -f
sudo journalctl -u gather-api -f
```

### 方式三：使用 Docker

#### 创建 Dockerfile
```dockerfile
# Dockerfile
FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o gather-collector cmd/gather/main.go
RUN go build -o gather-api cmd/api/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/

COPY --from=builder /app/gather-collector .
COPY --from=builder /app/gather-api .
COPY --from=builder /app/configs configs/

EXPOSE 8080
```

#### 创建 docker-compose.yml
```yaml
version: '3.8'

services:
  gather-collector:
    build: .
    command: ./gather-collector --config configs/config.yaml
    volumes:
      - ./configs:/root/configs
      - ./outputs:/root/outputs
      - ./data:/root/data
    environment:
      - ETH_NODE_URL=${ETH_NODE_URL}
      - KAFKA_BROKERS=kafka:9092
    depends_on:
      - kafka
    restart: unless-stopped

  gather-api:
    build: .
    command: ./gather-api --config configs/config.yaml --port 8080
    ports:
      - "8080:8080"
    volumes:
      - ./configs:/root/configs
      - ./data:/root/data
    environment:
      - ETH_NODE_URL=${ETH_NODE_URL}
      - KAFKA_BROKERS=kafka:9092
    depends_on:
      - kafka
    restart: unless-stopped

  zookeeper:
    image: confluentinc/cp-zookeeper:latest
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181
      ZOOKEEPER_TICK_TIME: 2000

  kafka:
    image: confluentinc/cp-kafka:latest
    depends_on:
      - zookeeper
    ports:
      - "9092:9092"
    environment:
      KAFKA_BROKER_ID: 1
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:9092
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
```

#### 启动 Docker 部署
```bash
# 构建并启动所有服务
docker-compose up -d

# 查看日志
docker-compose logs -f gather-collector
docker-compose logs -f gather-api

# 停止服务
docker-compose down
```

## 监控和维护

### 1. 健康检查

```bash
# 检查 API 服务状态
curl http://localhost:8080/health

# 检查采集器状态
curl http://localhost:8080/api/v1/status

# 获取统计信息
curl http://localhost:8080/api/v1/stats
```

### 2. 日志监控

```bash
# 查看实时日志
tail -f logs/collector.log
tail -f logs/api.log

# 使用 systemd 查看日志
sudo journalctl -u gather-collector -f --since "1 hour ago"
```

### 3. 性能监控

#### 系统资源监控
```bash
# CPU 和内存使用情况
top -p $(pgrep gather)

# 磁盘使用情况
df -h
du -sh outputs/ data/ logs/

# 网络连接
netstat -tulpn | grep gather
```

#### 应用监控
```bash
# 获取详细统计信息
curl http://localhost:8080/api/v1/stats
curl http://localhost:8080/api/v1/errors/stats
curl http://localhost:8080/api/v1/connection/stats
```

### 4. 备份和恢复

#### 配置备份
```bash
# 备份配置
tar -czf gather-config-$(date +%Y%m%d).tar.gz configs/ .env

# 备份进度数据
cp data/progress.db data/progress.db.backup-$(date +%Y%m%d)
```

#### 数据恢复
```bash
# 恢复配置
tar -xzf gather-config-20240101.tar.gz

# 恢复进度数据
cp data/progress.db.backup-20240101 data/progress.db
```

## 故障排除

### 常见问题

#### 1. 连接以太坊节点失败
```bash
# 检查网络连通性
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  https://mainnet.infura.io/v3/YOUR_PROJECT_ID

# 检查 API 密钥是否正确
# 检查节点URL格式是否正确
```

#### 2. Kafka 连接失败
```bash
# 检查 Kafka 服务状态
sudo systemctl status kafka
docker-compose ps kafka

# 测试 Kafka 连接
kafka-console-producer --bootstrap-server localhost:9092 --topic test
```

#### 3. 内存不足
```bash
# 增加 swap 空间
sudo fallocate -l 4G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile

# 调整采集器配置
# 减少 workers 数量
# 减少 batch_size
```

#### 4. 磁盘空间不足
```bash
# 清理日志文件
find logs/ -name "*.log" -mtime +7 -delete

# 压缩输出文件
gzip outputs/*.json

# 清理临时文件
rm -rf /tmp/gather-*
```

### 日志分析

#### 查找错误
```bash
# 查找错误信息
grep -i error logs/collector.log | tail -20

# 查找特定错误类型
grep "CONNECTION_FAILED" logs/collector.log

# 统计错误频率
grep -c "ERROR" logs/collector.log
```

#### 性能分析
```bash
# 分析处理速度
grep "blocks/sec" logs/collector.log | tail -10

# 查找慢请求
grep "timeout" logs/collector.log
```

## 安全建议

### 1. 网络安全
- 使用防火墙限制不必要的端口访问
- 仅对信任的IP开放API端口
- 使用 HTTPS 部署 API 服务

### 2. 访问控制
- 为 gather 创建专用用户账户
- 限制文件和目录权限
- 定期更新系统和依赖包

### 3. 监控和审计
- 启用系统审计日志
- 监控异常网络活动
- 定期检查系统日志

### 4. 数据保护
- 加密敏感配置信息
- 定期备份重要数据
- 使用安全的传输协议

## 性能优化

### 1. 硬件优化
- 使用 SSD 存储提高 I/O 性能
- 增加内存减少磁盘交换
- 使用高速网络连接

### 2. 软件优化
- 调整 Go 运行时参数
- 优化 Kafka 配置
- 使用连接池减少连接开销

### 3. 配置优化
```yaml
# 高性能配置示例
collector:
  workers: 20              # 增加工作协程
  batch_size: 200          # 增加批次大小
  timeout: "60s"           # 增加超时时间

output:
  format: "kafka_async"    # 使用异步输出
  
decoder:
  enable_cache: true       # 启用缓存
  cache_size: 50000        # 增大缓存大小
```

通过遵循本部署指南，你应该能够成功部署和运行 Gather 以太坊数据采集系统。如遇问题，请查看日志文件或联系技术支持。