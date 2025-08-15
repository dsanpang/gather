# 配置管理指南

## 配置系统架构

Gather项目支持两种配置方式：

1. **YAML配置文件** - 静态配置，适合大部分场景
2. **数据库配置** - 动态配置，适合运维管理

## 配置方式说明

### 方式1：YAML配置文件 (推荐)

#### 使用步骤
```bash
# 复制配置模板
cp configs/config.example.yaml configs/config.yaml

# 编辑配置文件
nano configs/config.yaml
```

#### 配置示例
```yaml
# configs/config.yaml
blockchain:
  nodes:
    - name: "infura"
      url: "https://mainnet.infura.io/v3/YOUR_INFURA_API_KEY"
      type: "infura"
      rate_limit: 100
      priority: 1
    
    - name: "alchemy"
      url: "https://eth-mainnet.alchemyapi.io/v2/YOUR_ALCHEMY_API_KEY"
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
    brokers: ["localhost:9092"]
    topics:
      blocks: "blockchain_blocks"
      transactions: "blockchain_transactions"

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

### 方式2：数据库配置

#### 使用场景
- 需要动态修改配置不重启
- 多实例集中管理
- 运维界面管理

#### 设置步骤
1. 创建数据库和表结构
2. 在配置中启用数据库配置
3. 通过API或数据库直接修改配置

#### 数据库表结构
```sql
-- 节点配置表
CREATE TABLE blockchain_nodes (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    url TEXT NOT NULL,
    node_type VARCHAR(50),
    rate_limit INTEGER DEFAULT 100,
    priority INTEGER DEFAULT 1,
    is_active BOOLEAN DEFAULT true
);

-- 采集器配置表  
CREATE TABLE collector_config (
    config_key VARCHAR(100) PRIMARY KEY,
    config_value TEXT NOT NULL,
    is_active BOOLEAN DEFAULT true
);

-- Kafka主题配置表
CREATE TABLE kafka_topics (
    data_type VARCHAR(50) PRIMARY KEY,
    topic_name VARCHAR(100) NOT NULL,
    is_active BOOLEAN DEFAULT true
);
```

#### 在配置文件中启用数据库
```yaml
# configs/config.yaml
database:
  enabled: true
  dsn: "postgres://user:password@localhost:5432/gather_config"
  
# 数据库配置会覆盖YAML中的对应设置
```

## 配置优先级

当启用数据库配置时：
**数据库配置 > YAML配置 > 默认值**

## 安全最佳实践

1. **保护配置文件**：`configs/config.yaml` 包含敏感信息，不要提交到代码库
2. **使用占位符**：示例配置文件使用占位符，不包含真实密钥
3. **数据库安全**：数据库配置表设置适当的访问权限
4. **配置验证**：系统启动时自动验证配置有效性

## 配置管理API

启用数据库配置后，可通过API管理配置：

```bash
# 获取节点配置
curl http://localhost:8080/api/v1/config/nodes

# 更新节点配置
curl -X PUT http://localhost:8080/api/v1/config/nodes/infura \
  -H "Content-Type: application/json" \
  -d '{"rate_limit": 200}'

# 获取采集器配置
curl http://localhost:8080/api/v1/config/collector
```