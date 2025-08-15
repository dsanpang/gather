# 数据库配置使用指南

## 概述

Gather支持使用数据库进行动态配置管理，这样可以在不重启服务的情况下修改配置，非常适合生产环境运维。

## 支持的数据库

- **PostgreSQL** (推荐) - 完整功能支持
- **MySQL 8.0+** - 完整功能支持

## 快速开始

### 1. 数据库初始化

#### 方法一：自动初始化脚本（推荐）

```bash
# PostgreSQL
export DB_TYPE=postgresql
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=gather_config
export DB_USER=postgres
export DB_PASSWORD=your_password

./configs/init_database.sh
```

```bash
# MySQL
export DB_TYPE=mysql
export DB_HOST=localhost
export DB_PORT=3306
export DB_NAME=gather_config
export DB_USER=root
export DB_PASSWORD=your_password

./configs/init_database.sh
```

#### 方法二：手动执行SQL脚本

```bash
# PostgreSQL
psql -h localhost -U postgres -d gather_config -f configs/database_schema.sql

# MySQL
mysql -h localhost -u root -p gather_config < configs/database_schema_mysql.sql
```

### 2. 启用数据库配置

编辑 `configs/config.yaml`：

```yaml
# 启用数据库配置
database:
  enabled: true
  # PostgreSQL
  dsn: "postgres://postgres:password@localhost:5432/gather_config?sslmode=disable"
  # 或者 MySQL
  # dsn: "root:password@tcp(localhost:3306)/gather_config?charset=utf8mb4&parseTime=True&loc=Local"
```

## 数据库表结构说明

### 核心配置表

| 表名 | 用途 | 重要字段 |
|------|------|----------|
| `blockchain_nodes` | 节点配置 | name, url, rate_limit, priority |
| `collector_config` | 采集器配置 | config_key, config_value, category |
| `kafka_topics` | Kafka主题配置 | data_type, topic_name, partitions |
| `kafka_brokers` | Kafka代理配置 | broker_address, broker_port |
| `decoder_config` | 解码器配置 | decoder_type, config_key, config_value |
| `logging_config` | 日志配置 | log_level, log_format, output_type |
| `system_config` | 系统配置 | config_section, config_key, config_value |

### 管理表

| 表名 | 用途 |
|------|------|
| `config_history` | 配置变更历史记录 |
| `config_templates` | 配置模板 |

## 配置管理

### 1. 节点配置管理

```sql
-- 查看所有节点
SELECT * FROM blockchain_nodes WHERE is_active = true ORDER BY priority;

-- 添加新节点
INSERT INTO blockchain_nodes (name, url, node_type, provider, rate_limit, priority) 
VALUES ('my_node', 'https://my.node.com', 'rpc', 'custom', 200, 4);

-- 更新节点配置
UPDATE blockchain_nodes 
SET rate_limit = 150, priority = 2 
WHERE name = 'infura_mainnet';

-- 禁用节点
UPDATE blockchain_nodes SET is_active = false WHERE name = 'old_node';
```

### 2. 采集器配置管理

```sql
-- 查看采集器配置
SELECT config_key, config_value, description 
FROM collector_config 
WHERE is_active = true;

-- 更新工作协程数
UPDATE collector_config 
SET config_value = '20' 
WHERE config_key = 'workers';

-- 添加新配置
INSERT INTO collector_config (config_key, config_value, value_type, description, category)
VALUES ('max_block_lag', '10', 'int', '最大区块滞后数', 'sync');
```

### 3. Kafka配置管理

```sql
-- 查看Kafka主题配置
SELECT data_type, topic_name, partitions, replication_factor 
FROM kafka_topics 
WHERE is_active = true;

-- 添加新主题
INSERT INTO kafka_topics (data_type, topic_name, partitions, replication_factor)
VALUES ('blob_data', 'blockchain_blob_data', 6, 2);

-- 更新主题分区数（注意：需要在Kafka集群中手动执行）
UPDATE kafka_topics 
SET partitions = 9 
WHERE data_type = 'transactions';
```

### 4. 系统配置管理

```sql
-- 查看系统配置
SELECT config_section, config_key, config_value, description 
FROM system_config 
WHERE is_active = true
ORDER BY config_section, config_key;

-- 更新性能配置
UPDATE system_config 
SET config_value = '4096' 
WHERE config_section = 'performance' AND config_key = 'max_memory_mb';
```

## API管理（推荐）

### 节点管理API

```bash
# 获取所有节点配置
curl http://localhost:8080/api/v1/config/nodes

# 获取特定节点配置
curl http://localhost:8080/api/v1/config/nodes/infura_mainnet

# 更新节点配置
curl -X PUT http://localhost:8080/api/v1/config/nodes/infura_mainnet \
  -H "Content-Type: application/json" \
  -d '{"rate_limit": 200, "priority": 1}'

# 添加新节点
curl -X POST http://localhost:8080/api/v1/config/nodes \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my_custom_node",
    "url": "https://my.node.com",
    "node_type": "rpc",
    "rate_limit": 150,
    "priority": 5
  }'
```

### 采集器配置API

```bash
# 获取采集器配置
curl http://localhost:8080/api/v1/config/collector

# 更新配置
curl -X PUT http://localhost:8080/api/v1/config/collector \
  -H "Content-Type: application/json" \
  -d '{"workers": 20, "batch_size": 200}'
```

### 系统配置API

```bash
# 获取系统配置
curl http://localhost:8080/api/v1/config/system

# 更新系统配置
curl -X PUT http://localhost:8080/api/v1/config/system/performance \
  -H "Content-Type: application/json" \
  -d '{"max_memory_mb": "4096"}'
```

## 配置热更新

数据库配置的最大优势是支持热更新，无需重启服务：

1. **实时生效的配置**：
   - 节点启用/禁用
   - 速率限制调整
   - 日志级别变更

2. **需要重启的配置**：
   - 工作协程数变更
   - 输出格式变更
   - 数据库连接配置

3. **配置检查间隔**：
   - 默认每30秒检查一次配置变更
   - 可通过 `config_check_interval` 配置项调整

## 配置模板

系统提供了预定义的配置模板，便于快速部署：

```sql
-- 查看可用模板
SELECT template_name, template_type, description 
FROM config_templates 
WHERE is_active = true;

-- 使用模板创建配置
-- 例如：从模板创建高性能节点配置
```

## 备份和恢复

### 备份配置

```bash
# PostgreSQL
pg_dump -h localhost -U postgres -d gather_config --data-only > config_backup.sql

# MySQL
mysqldump -h localhost -u root -p gather_config > config_backup.sql
```

### 恢复配置

```bash
# PostgreSQL
psql -h localhost -U postgres -d gather_config < config_backup.sql

# MySQL
mysql -h localhost -u root -p gather_config < config_backup.sql
```

## 监控和维护

### 配置变更审计

```sql
-- 查看最近的配置变更
SELECT table_name, action, changed_by, changed_at, reason
FROM config_history 
ORDER BY changed_at DESC 
LIMIT 20;

-- 查看特定表的变更历史
SELECT * FROM config_history 
WHERE table_name = 'blockchain_nodes' 
ORDER BY changed_at DESC;
```

### 性能监控

```sql
-- 节点性能统计
SELECT 
    name,
    success_count,
    error_count,
    (success_count::float / NULLIF(success_count + error_count, 0) * 100) as success_rate,
    last_used_at
FROM blockchain_nodes 
WHERE is_active = true
ORDER BY success_rate DESC;
```

## 安全建议

1. **数据库访问控制**
   - 创建专用数据库用户
   - 限制网络访问
   - 定期更新密码

2. **配置变更管理**
   - 记录变更原因
   - 实施变更审批流程
   - 定期备份配置

3. **监控告警**
   - 监控配置变更频率
   - 异常配置变更告警
   - 数据库连接状态监控

## 故障排除

### 常见问题

1. **数据库连接失败**
   ```bash
   # 检查DSN配置
   # 检查数据库服务状态
   # 验证网络连通性
   ```

2. **配置未生效**
   ```bash
   # 检查 is_active 字段
   # 查看配置更新时间
   # 检查服务日志
   ```

3. **性能问题**
   ```sql
   -- 检查配置查询性能
   EXPLAIN SELECT * FROM blockchain_nodes WHERE is_active = true;
   ```

通过数据库配置管理，你可以实现更灵活、更强大的配置管理能力，特别适合生产环境的运维需求。