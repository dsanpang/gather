-- Gather ETH链数据采集系统数据库配置表结构
-- 数据库类型：PostgreSQL (推荐) 或 MySQL
-- 用途：动态配置管理，支持热更新

-- ==================================================
-- 1. 区块链节点配置表
-- ==================================================
CREATE TABLE IF NOT EXISTS blockchain_nodes (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE COMMENT '节点名称，如 infura, alchemy',
    url TEXT NOT NULL COMMENT '节点RPC URL',
    node_type VARCHAR(50) NOT NULL DEFAULT 'rpc' COMMENT '节点类型：rpc, ws, ipc',
    provider VARCHAR(50) COMMENT '提供商：infura, alchemy, quicknode等',
    rate_limit INTEGER NOT NULL DEFAULT 100 COMMENT '每秒请求限制',
    priority INTEGER NOT NULL DEFAULT 1 COMMENT '优先级，数字越小越优先',
    is_active BOOLEAN NOT NULL DEFAULT true COMMENT '是否启用',
    health_check_url TEXT COMMENT '健康检查URL',
    timeout_seconds INTEGER DEFAULT 30 COMMENT '超时时间(秒)',
    max_connections INTEGER DEFAULT 10 COMMENT '最大连接数',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMP COMMENT '最后使用时间',
    error_count INTEGER DEFAULT 0 COMMENT '错误计数',
    success_count INTEGER DEFAULT 0 COMMENT '成功计数'
);

-- 创建索引
CREATE INDEX idx_blockchain_nodes_priority ON blockchain_nodes(priority, is_active);
CREATE INDEX idx_blockchain_nodes_type ON blockchain_nodes(node_type, is_active);

-- ==================================================
-- 2. 采集器配置表
-- ==================================================
CREATE TABLE IF NOT EXISTS collector_config (
    id SERIAL PRIMARY KEY,
    config_key VARCHAR(100) NOT NULL UNIQUE COMMENT '配置键',
    config_value TEXT NOT NULL COMMENT '配置值',
    value_type VARCHAR(20) NOT NULL DEFAULT 'string' COMMENT '值类型：string, int, bool, json',
    description TEXT COMMENT '配置描述',
    is_active BOOLEAN NOT NULL DEFAULT true COMMENT '是否启用',
    category VARCHAR(50) DEFAULT 'general' COMMENT '配置分类',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(100) COMMENT '更新人'
);

-- 创建索引
CREATE INDEX idx_collector_config_category ON collector_config(category, is_active);

-- ==================================================
-- 3. 输出配置表
-- ==================================================
CREATE TABLE IF NOT EXISTS output_config (
    id SERIAL PRIMARY KEY,
    output_type VARCHAR(50) NOT NULL COMMENT '输出类型：kafka, file, database',
    config_key VARCHAR(100) NOT NULL COMMENT '配置键',
    config_value TEXT NOT NULL COMMENT '配置值',
    is_active BOOLEAN NOT NULL DEFAULT true COMMENT '是否启用',
    priority INTEGER DEFAULT 1 COMMENT '优先级',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 复合索引
CREATE INDEX idx_output_config_type ON output_config(output_type, is_active);

-- ==================================================
-- 4. Kafka主题配置表
-- ==================================================
CREATE TABLE IF NOT EXISTS kafka_topics (
    id SERIAL PRIMARY KEY,
    data_type VARCHAR(50) NOT NULL UNIQUE COMMENT '数据类型：blocks, transactions, logs等',
    topic_name VARCHAR(200) NOT NULL COMMENT 'Kafka主题名称',
    partitions INTEGER DEFAULT 3 COMMENT '分区数',
    replication_factor INTEGER DEFAULT 1 COMMENT '副本因子',
    is_active BOOLEAN NOT NULL DEFAULT true COMMENT '是否启用',
    compression_type VARCHAR(20) DEFAULT 'gzip' COMMENT '压缩类型',
    retention_ms BIGINT COMMENT '保留时间(毫秒)',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ==================================================
-- 5. Kafka Broker配置表
-- ==================================================
CREATE TABLE IF NOT EXISTS kafka_brokers (
    id SERIAL PRIMARY KEY,
    broker_address VARCHAR(255) NOT NULL COMMENT 'Broker地址',
    broker_port INTEGER NOT NULL DEFAULT 9092 COMMENT 'Broker端口',
    is_active BOOLEAN NOT NULL DEFAULT true COMMENT '是否启用',
    priority INTEGER DEFAULT 1 COMMENT '优先级',
    max_connections INTEGER DEFAULT 100 COMMENT '最大连接数',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_check_at TIMESTAMP COMMENT '最后检查时间',
    is_healthy BOOLEAN DEFAULT true COMMENT '健康状态'
);

-- ==================================================
-- 6. 解码器配置表
-- ==================================================
CREATE TABLE IF NOT EXISTS decoder_config (
    id SERIAL PRIMARY KEY,
    decoder_type VARCHAR(50) NOT NULL COMMENT '解码器类型：4byte, abi, custom',
    config_key VARCHAR(100) NOT NULL COMMENT '配置键',
    config_value TEXT NOT NULL COMMENT '配置值',
    is_active BOOLEAN NOT NULL DEFAULT true COMMENT '是否启用',
    cache_enabled BOOLEAN DEFAULT true COMMENT '是否启用缓存',
    cache_size INTEGER DEFAULT 10000 COMMENT '缓存大小',
    api_timeout_seconds INTEGER DEFAULT 5 COMMENT 'API超时时间',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ==================================================
-- 7. 日志配置表
-- ==================================================
CREATE TABLE IF NOT EXISTS logging_config (
    id SERIAL PRIMARY KEY,
    log_level VARCHAR(20) NOT NULL DEFAULT 'info' COMMENT '日志级别',
    log_format VARCHAR(20) NOT NULL DEFAULT 'json' COMMENT '日志格式：json, text',
    output_type VARCHAR(50) NOT NULL DEFAULT 'stdout' COMMENT '输出类型：stdout, file, syslog',
    output_path TEXT COMMENT '输出路径（文件输出时）',
    rotation_enabled BOOLEAN DEFAULT false COMMENT '是否启用轮转',
    max_size_mb INTEGER DEFAULT 100 COMMENT '最大文件大小(MB)',
    max_age_days INTEGER DEFAULT 30 COMMENT '保留天数',
    max_backups INTEGER DEFAULT 3 COMMENT '备份文件数',
    compression_enabled BOOLEAN DEFAULT true COMMENT '是否压缩旧文件',
    is_active BOOLEAN NOT NULL DEFAULT true COMMENT '是否启用',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ==================================================
-- 8. 系统配置表（全局配置）
-- ==================================================
CREATE TABLE IF NOT EXISTS system_config (
    id SERIAL PRIMARY KEY,
    config_section VARCHAR(50) NOT NULL COMMENT '配置段：system, security, performance',
    config_key VARCHAR(100) NOT NULL COMMENT '配置键',
    config_value TEXT NOT NULL COMMENT '配置值',
    value_type VARCHAR(20) DEFAULT 'string' COMMENT '值类型',
    description TEXT COMMENT '配置描述',
    is_active BOOLEAN NOT NULL DEFAULT true COMMENT '是否启用',
    is_readonly BOOLEAN DEFAULT false COMMENT '是否只读',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 复合唯一索引
CREATE UNIQUE INDEX idx_system_config_section_key ON system_config(config_section, config_key);

-- ==================================================
-- 9. 配置变更历史表
-- ==================================================
CREATE TABLE IF NOT EXISTS config_history (
    id SERIAL PRIMARY KEY,
    table_name VARCHAR(50) NOT NULL COMMENT '配置表名',
    record_id INTEGER NOT NULL COMMENT '记录ID',
    action VARCHAR(20) NOT NULL COMMENT '操作：INSERT, UPDATE, DELETE',
    old_values JSONB COMMENT '旧值（JSON格式）',
    new_values JSONB COMMENT '新值（JSON格式）',
    changed_by VARCHAR(100) COMMENT '操作人',
    changed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    reason TEXT COMMENT '变更原因'
);

-- 创建索引
CREATE INDEX idx_config_history_table ON config_history(table_name, record_id);
CREATE INDEX idx_config_history_time ON config_history(changed_at);

-- ==================================================
-- 10. 配置模板表（预定义配置）
-- ==================================================
CREATE TABLE IF NOT EXISTS config_templates (
    id SERIAL PRIMARY KEY,
    template_name VARCHAR(100) NOT NULL UNIQUE COMMENT '模板名称',
    template_type VARCHAR(50) NOT NULL COMMENT '模板类型：node, output, decoder',
    template_data JSONB NOT NULL COMMENT '模板数据（JSON格式）',
    description TEXT COMMENT '模板描述',
    is_active BOOLEAN NOT NULL DEFAULT true COMMENT '是否启用',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ==================================================
-- 插入默认配置数据
-- ==================================================

-- 默认节点配置
INSERT INTO blockchain_nodes (name, url, node_type, provider, rate_limit, priority) VALUES
('infura_mainnet', 'https://mainnet.infura.io/v3/YOUR_API_KEY', 'rpc', 'infura', 100, 1),
('alchemy_mainnet', 'https://eth-mainnet.alchemyapi.io/v2/YOUR_API_KEY', 'rpc', 'alchemy', 100, 2),
('quicknode_mainnet', 'https://YOUR_ENDPOINT.quiknode.pro/YOUR_API_KEY/', 'rpc', 'quicknode', 100, 3)
ON CONFLICT (name) DO NOTHING;

-- 默认采集器配置
INSERT INTO collector_config (config_key, config_value, value_type, description, category) VALUES
('workers', '10', 'int', '工作协程数', 'performance'),
('batch_size', '100', 'int', '批处理大小', 'performance'),
('retry_limit', '3', 'int', '重试次数', 'reliability'),
('timeout', '30s', 'string', '超时时间', 'reliability'),
('enable_trace', 'true', 'bool', '启用交易追踪', 'features'),
('enable_state_tracking', 'true', 'bool', '启用状态跟踪', 'features'),
('start_block', 'latest', 'string', '起始区块：latest, number, hash', 'sync'),
('sync_mode', 'realtime', 'string', '同步模式：realtime, historical', 'sync')
ON CONFLICT (config_key) DO NOTHING;

-- 默认Kafka主题配置
INSERT INTO kafka_topics (data_type, topic_name, partitions, replication_factor) VALUES
('blocks', 'blockchain_blocks', 3, 1),
('transactions', 'blockchain_transactions', 6, 1),
('logs', 'blockchain_logs', 6, 1),
('internal_transactions', 'blockchain_internal_transactions', 3, 1),
('state_changes', 'blockchain_state_changes', 3, 1),
('contract_creations', 'blockchain_contract_creations', 3, 1),
('reorg_notifications', 'blockchain_reorg_notifications', 1, 1),
('withdrawals', 'blockchain_withdrawals', 3, 1),
('blob_transactions', 'blockchain_blob_transactions', 3, 1)
ON CONFLICT (data_type) DO NOTHING;

-- 默认Kafka Broker配置
INSERT INTO kafka_brokers (broker_address, broker_port, priority) VALUES
('localhost', 9092, 1),
('kafka-broker-1', 9092, 2),
('kafka-broker-2', 9092, 3)
ON CONFLICT DO NOTHING;

-- 默认解码器配置
INSERT INTO decoder_config (decoder_type, config_key, config_value) VALUES
('4byte', 'api_url', 'https://www.4byte.directory/api/v1/signatures/'),
('4byte', 'api_timeout', '5'),
('4byte', 'cache_enabled', 'true'),
('4byte', 'cache_size', '10000'),
('abi', 'enable_local_abi', 'true'),
('abi', 'abi_cache_size', '5000')
ON CONFLICT DO NOTHING;

-- 默认系统配置
INSERT INTO system_config (config_section, config_key, config_value, description) VALUES
('system', 'service_name', 'gather-collector', '服务名称'),
('system', 'version', '1.0.0', '版本号'),
('security', 'enable_auth', 'false', '是否启用认证'),
('security', 'api_rate_limit', '1000', 'API速率限制'),
('performance', 'max_memory_mb', '2048', '最大内存使用(MB)'),
('performance', 'gc_target_percent', '75', 'GC目标百分比'),
('monitoring', 'metrics_enabled', 'true', '是否启用指标收集'),
('monitoring', 'health_check_interval', '30s', '健康检查间隔')
ON CONFLICT (config_section, config_key) DO NOTHING;

-- ==================================================
-- 创建更新时间触发器（PostgreSQL）
-- ==================================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- 为各表创建更新时间触发器
CREATE TRIGGER update_blockchain_nodes_updated_at BEFORE UPDATE ON blockchain_nodes FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_collector_config_updated_at BEFORE UPDATE ON collector_config FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_output_config_updated_at BEFORE UPDATE ON output_config FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_kafka_topics_updated_at BEFORE UPDATE ON kafka_topics FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_kafka_brokers_updated_at BEFORE UPDATE ON kafka_brokers FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_decoder_config_updated_at BEFORE UPDATE ON decoder_config FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_logging_config_updated_at BEFORE UPDATE ON logging_config FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_system_config_updated_at BEFORE UPDATE ON system_config FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ==================================================
-- 创建配置历史记录触发器
-- ==================================================
CREATE OR REPLACE FUNCTION log_config_changes()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'UPDATE' THEN
        INSERT INTO config_history (table_name, record_id, action, old_values, new_values)
        VALUES (TG_TABLE_NAME, NEW.id, 'UPDATE', to_jsonb(OLD), to_jsonb(NEW));
        RETURN NEW;
    ELSIF TG_OP = 'INSERT' THEN
        INSERT INTO config_history (table_name, record_id, action, new_values)
        VALUES (TG_TABLE_NAME, NEW.id, 'INSERT', to_jsonb(NEW));
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        INSERT INTO config_history (table_name, record_id, action, old_values)
        VALUES (TG_TABLE_NAME, OLD.id, 'DELETE', to_jsonb(OLD));
        RETURN OLD;
    END IF;
    RETURN NULL;
END;
$$ language 'plpgsql';

-- 为配置表创建历史记录触发器
CREATE TRIGGER log_blockchain_nodes_changes AFTER INSERT OR UPDATE OR DELETE ON blockchain_nodes FOR EACH ROW EXECUTE FUNCTION log_config_changes();
CREATE TRIGGER log_collector_config_changes AFTER INSERT OR UPDATE OR DELETE ON collector_config FOR EACH ROW EXECUTE FUNCTION log_config_changes();
CREATE TRIGGER log_output_config_changes AFTER INSERT OR UPDATE OR DELETE ON output_config FOR EACH ROW EXECUTE FUNCTION log_config_changes();

-- ==================================================
-- 常用查询视图
-- ==================================================

-- 活跃节点视图
CREATE OR REPLACE VIEW v_active_nodes AS
SELECT 
    id, name, url, node_type, provider, rate_limit, priority,
    timeout_seconds, max_connections, last_used_at,
    error_count, success_count,
    CASE 
        WHEN error_count = 0 THEN 100.0
        ELSE (success_count::float / (success_count + error_count) * 100)
    END as success_rate
FROM blockchain_nodes 
WHERE is_active = true 
ORDER BY priority;

-- 配置概览视图
CREATE OR REPLACE VIEW v_config_overview AS
SELECT 
    'nodes' as config_type, COUNT(*) as total_count, COUNT(*) FILTER (WHERE is_active) as active_count
FROM blockchain_nodes
UNION ALL
SELECT 
    'collector_configs', COUNT(*), COUNT(*) FILTER (WHERE is_active)
FROM collector_config
UNION ALL
SELECT 
    'kafka_topics', COUNT(*), COUNT(*) FILTER (WHERE is_active)
FROM kafka_topics;

-- ==================================================
-- 权限设置建议
-- ==================================================

-- 创建只读用户（用于监控）
-- CREATE USER gather_readonly WITH PASSWORD 'your_secure_password';
-- GRANT SELECT ON ALL TABLES IN SCHEMA public TO gather_readonly;

-- 创建应用用户（用于正常操作）
-- CREATE USER gather_app WITH PASSWORD 'your_secure_password';
-- GRANT SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA public TO gather_app;
-- GRANT USAGE ON ALL SEQUENCES IN SCHEMA public TO gather_app;

COMMIT;