-- Gather ETH链数据采集系统数据库配置表结构（MySQL版本）
-- 数据库类型：MySQL 8.0+
-- 用途：动态配置管理，支持热更新

-- 设置字符集
SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ==================================================
-- 1. 区块链节点配置表
-- ==================================================
CREATE TABLE IF NOT EXISTS `blockchain_nodes` (
    `id` int(11) NOT NULL AUTO_INCREMENT,
    `name` varchar(100) NOT NULL COMMENT '节点名称，如 infura, alchemy',
    `url` text NOT NULL COMMENT '节点RPC URL',
    `node_type` varchar(50) NOT NULL DEFAULT 'rpc' COMMENT '节点类型：rpc, ws, ipc',
    `provider` varchar(50) DEFAULT NULL COMMENT '提供商：infura, alchemy, quicknode等',
    `rate_limit` int(11) NOT NULL DEFAULT 100 COMMENT '每秒请求限制',
    `priority` int(11) NOT NULL DEFAULT 1 COMMENT '优先级，数字越小越优先',
    `is_active` tinyint(1) NOT NULL DEFAULT 1 COMMENT '是否启用',
    `health_check_url` text COMMENT '健康检查URL',
    `timeout_seconds` int(11) DEFAULT 30 COMMENT '超时时间(秒)',
    `max_connections` int(11) DEFAULT 10 COMMENT '最大连接数',
    `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    `last_used_at` timestamp NULL DEFAULT NULL COMMENT '最后使用时间',
    `error_count` int(11) DEFAULT 0 COMMENT '错误计数',
    `success_count` int(11) DEFAULT 0 COMMENT '成功计数',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_name` (`name`),
    KEY `idx_priority_active` (`priority`, `is_active`),
    KEY `idx_type_active` (`node_type`, `is_active`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='区块链节点配置表';

-- ==================================================
-- 2. 采集器配置表
-- ==================================================
CREATE TABLE IF NOT EXISTS `collector_config` (
    `id` int(11) NOT NULL AUTO_INCREMENT,
    `config_key` varchar(100) NOT NULL COMMENT '配置键',
    `config_value` text NOT NULL COMMENT '配置值',
    `value_type` varchar(20) NOT NULL DEFAULT 'string' COMMENT '值类型：string, int, bool, json',
    `description` text COMMENT '配置描述',
    `is_active` tinyint(1) NOT NULL DEFAULT 1 COMMENT '是否启用',
    `category` varchar(50) DEFAULT 'general' COMMENT '配置分类',
    `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    `updated_by` varchar(100) DEFAULT NULL COMMENT '更新人',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_config_key` (`config_key`),
    KEY `idx_category_active` (`category`, `is_active`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='采集器配置表';

-- ==================================================
-- 3. 输出配置表
-- ==================================================
CREATE TABLE IF NOT EXISTS `output_config` (
    `id` int(11) NOT NULL AUTO_INCREMENT,
    `output_type` varchar(50) NOT NULL COMMENT '输出类型：kafka, file, database',
    `config_key` varchar(100) NOT NULL COMMENT '配置键',
    `config_value` text NOT NULL COMMENT '配置值',
    `is_active` tinyint(1) NOT NULL DEFAULT 1 COMMENT '是否启用',
    `priority` int(11) DEFAULT 1 COMMENT '优先级',
    `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_type_active` (`output_type`, `is_active`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='输出配置表';

-- ==================================================
-- 4. Kafka主题配置表
-- ==================================================
CREATE TABLE IF NOT EXISTS `kafka_topics` (
    `id` int(11) NOT NULL AUTO_INCREMENT,
    `data_type` varchar(50) NOT NULL COMMENT '数据类型：blocks, transactions, logs等',
    `topic_name` varchar(200) NOT NULL COMMENT 'Kafka主题名称',
    `partitions` int(11) DEFAULT 3 COMMENT '分区数',
    `replication_factor` int(11) DEFAULT 1 COMMENT '副本因子',
    `is_active` tinyint(1) NOT NULL DEFAULT 1 COMMENT '是否启用',
    `compression_type` varchar(20) DEFAULT 'gzip' COMMENT '压缩类型',
    `retention_ms` bigint(20) DEFAULT NULL COMMENT '保留时间(毫秒)',
    `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_data_type` (`data_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Kafka主题配置表';

-- ==================================================
-- 5. Kafka Broker配置表
-- ==================================================
CREATE TABLE IF NOT EXISTS `kafka_brokers` (
    `id` int(11) NOT NULL AUTO_INCREMENT,
    `broker_address` varchar(255) NOT NULL COMMENT 'Broker地址',
    `broker_port` int(11) NOT NULL DEFAULT 9092 COMMENT 'Broker端口',
    `is_active` tinyint(1) NOT NULL DEFAULT 1 COMMENT '是否启用',
    `priority` int(11) DEFAULT 1 COMMENT '优先级',
    `max_connections` int(11) DEFAULT 100 COMMENT '最大连接数',
    `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    `last_check_at` timestamp NULL DEFAULT NULL COMMENT '最后检查时间',
    `is_healthy` tinyint(1) DEFAULT 1 COMMENT '健康状态',
    PRIMARY KEY (`id`),
    KEY `idx_active_priority` (`is_active`, `priority`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Kafka Broker配置表';

-- ==================================================
-- 6. 解码器配置表
-- ==================================================
CREATE TABLE IF NOT EXISTS `decoder_config` (
    `id` int(11) NOT NULL AUTO_INCREMENT,
    `decoder_type` varchar(50) NOT NULL COMMENT '解码器类型：4byte, abi, custom',
    `config_key` varchar(100) NOT NULL COMMENT '配置键',
    `config_value` text NOT NULL COMMENT '配置值',
    `is_active` tinyint(1) NOT NULL DEFAULT 1 COMMENT '是否启用',
    `cache_enabled` tinyint(1) DEFAULT 1 COMMENT '是否启用缓存',
    `cache_size` int(11) DEFAULT 10000 COMMENT '缓存大小',
    `api_timeout_seconds` int(11) DEFAULT 5 COMMENT 'API超时时间',
    `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_type_active` (`decoder_type`, `is_active`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='解码器配置表';

-- ==================================================
-- 7. 日志配置表
-- ==================================================
CREATE TABLE IF NOT EXISTS `logging_config` (
    `id` int(11) NOT NULL AUTO_INCREMENT,
    `log_level` varchar(20) NOT NULL DEFAULT 'info' COMMENT '日志级别',
    `log_format` varchar(20) NOT NULL DEFAULT 'json' COMMENT '日志格式：json, text',
    `output_type` varchar(50) NOT NULL DEFAULT 'stdout' COMMENT '输出类型：stdout, file, syslog',
    `output_path` text COMMENT '输出路径（文件输出时）',
    `rotation_enabled` tinyint(1) DEFAULT 0 COMMENT '是否启用轮转',
    `max_size_mb` int(11) DEFAULT 100 COMMENT '最大文件大小(MB)',
    `max_age_days` int(11) DEFAULT 30 COMMENT '保留天数',
    `max_backups` int(11) DEFAULT 3 COMMENT '备份文件数',
    `compression_enabled` tinyint(1) DEFAULT 1 COMMENT '是否压缩旧文件',
    `is_active` tinyint(1) NOT NULL DEFAULT 1 COMMENT '是否启用',
    `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='日志配置表';

-- ==================================================
-- 8. 系统配置表（全局配置）
-- ==================================================
CREATE TABLE IF NOT EXISTS `system_config` (
    `id` int(11) NOT NULL AUTO_INCREMENT,
    `config_section` varchar(50) NOT NULL COMMENT '配置段：system, security, performance',
    `config_key` varchar(100) NOT NULL COMMENT '配置键',
    `config_value` text NOT NULL COMMENT '配置值',
    `value_type` varchar(20) DEFAULT 'string' COMMENT '值类型',
    `description` text COMMENT '配置描述',
    `is_active` tinyint(1) NOT NULL DEFAULT 1 COMMENT '是否启用',
    `is_readonly` tinyint(1) DEFAULT 0 COMMENT '是否只读',
    `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_section_key` (`config_section`, `config_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='系统配置表';

-- ==================================================
-- 9. 配置变更历史表
-- ==================================================
CREATE TABLE IF NOT EXISTS `config_history` (
    `id` int(11) NOT NULL AUTO_INCREMENT,
    `table_name` varchar(50) NOT NULL COMMENT '配置表名',
    `record_id` int(11) NOT NULL COMMENT '记录ID',
    `action` varchar(20) NOT NULL COMMENT '操作：INSERT, UPDATE, DELETE',
    `old_values` json COMMENT '旧值（JSON格式）',
    `new_values` json COMMENT '新值（JSON格式）',
    `changed_by` varchar(100) DEFAULT NULL COMMENT '操作人',
    `changed_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `reason` text COMMENT '变更原因',
    PRIMARY KEY (`id`),
    KEY `idx_table_record` (`table_name`, `record_id`),
    KEY `idx_changed_at` (`changed_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='配置变更历史表';

-- ==================================================
-- 10. 配置模板表（预定义配置）
-- ==================================================
CREATE TABLE IF NOT EXISTS `config_templates` (
    `id` int(11) NOT NULL AUTO_INCREMENT,
    `template_name` varchar(100) NOT NULL COMMENT '模板名称',
    `template_type` varchar(50) NOT NULL COMMENT '模板类型：node, output, decoder',
    `template_data` json NOT NULL COMMENT '模板数据（JSON格式）',
    `description` text COMMENT '模板描述',
    `is_active` tinyint(1) NOT NULL DEFAULT 1 COMMENT '是否启用',
    `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_template_name` (`template_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='配置模板表';

-- ==================================================
-- 插入默认配置数据
-- ==================================================

-- 默认节点配置
INSERT IGNORE INTO `blockchain_nodes` (`name`, `url`, `node_type`, `provider`, `rate_limit`, `priority`) VALUES
('infura_mainnet', 'https://mainnet.infura.io/v3/YOUR_API_KEY', 'rpc', 'infura', 100, 1),
('alchemy_mainnet', 'https://eth-mainnet.alchemyapi.io/v2/YOUR_API_KEY', 'rpc', 'alchemy', 100, 2),
('quicknode_mainnet', 'https://YOUR_ENDPOINT.quiknode.pro/YOUR_API_KEY/', 'rpc', 'quicknode', 100, 3);

-- 默认采集器配置
INSERT IGNORE INTO `collector_config` (`config_key`, `config_value`, `value_type`, `description`, `category`) VALUES
('workers', '10', 'int', '工作协程数', 'performance'),
('batch_size', '100', 'int', '批处理大小', 'performance'),
('retry_limit', '3', 'int', '重试次数', 'reliability'),
('timeout', '30s', 'string', '超时时间', 'reliability'),
('enable_trace', 'true', 'bool', '启用交易追踪', 'features'),
('enable_state_tracking', 'true', 'bool', '启用状态跟踪', 'features'),
('start_block', 'latest', 'string', '起始区块：latest, number, hash', 'sync'),
('sync_mode', 'realtime', 'string', '同步模式：realtime, historical', 'sync');

-- 默认Kafka主题配置
INSERT IGNORE INTO `kafka_topics` (`data_type`, `topic_name`, `partitions`, `replication_factor`) VALUES
('blocks', 'blockchain_blocks', 3, 1),
('transactions', 'blockchain_transactions', 6, 1),
('logs', 'blockchain_logs', 6, 1),
('internal_transactions', 'blockchain_internal_transactions', 3, 1),
('state_changes', 'blockchain_state_changes', 3, 1),
('contract_creations', 'blockchain_contract_creations', 3, 1),
('reorg_notifications', 'blockchain_reorg_notifications', 1, 1),
('withdrawals', 'blockchain_withdrawals', 3, 1),
('blob_transactions', 'blockchain_blob_transactions', 3, 1);

-- 默认Kafka Broker配置
INSERT IGNORE INTO `kafka_brokers` (`broker_address`, `broker_port`, `priority`) VALUES
('localhost', 9092, 1),
('kafka-broker-1', 9092, 2),
('kafka-broker-2', 9092, 3);

-- 默认解码器配置
INSERT IGNORE INTO `decoder_config` (`decoder_type`, `config_key`, `config_value`) VALUES
('4byte', 'api_url', 'https://www.4byte.directory/api/v1/signatures/'),
('4byte', 'api_timeout', '5'),
('4byte', 'cache_enabled', 'true'),
('4byte', 'cache_size', '10000'),
('abi', 'enable_local_abi', 'true'),
('abi', 'abi_cache_size', '5000');

-- 默认系统配置
INSERT IGNORE INTO `system_config` (`config_section`, `config_key`, `config_value`, `description`) VALUES
('system', 'service_name', 'gather-collector', '服务名称'),
('system', 'version', '1.0.0', '版本号'),
('security', 'enable_auth', 'false', '是否启用认证'),
('security', 'api_rate_limit', '1000', 'API速率限制'),
('performance', 'max_memory_mb', '2048', '最大内存使用(MB)'),
('performance', 'gc_target_percent', '75', 'GC目标百分比'),
('monitoring', 'metrics_enabled', 'true', '是否启用指标收集'),
('monitoring', 'health_check_interval', '30s', '健康检查间隔');

-- ==================================================
-- 常用查询视图
-- ==================================================

-- 活跃节点视图
CREATE OR REPLACE VIEW `v_active_nodes` AS
SELECT 
    `id`, `name`, `url`, `node_type`, `provider`, `rate_limit`, `priority`,
    `timeout_seconds`, `max_connections`, `last_used_at`,
    `error_count`, `success_count`,
    CASE 
        WHEN `error_count` = 0 THEN 100.0
        ELSE (`success_count` / (`success_count` + `error_count`) * 100)
    END as `success_rate`
FROM `blockchain_nodes` 
WHERE `is_active` = 1 
ORDER BY `priority`;

-- 配置概览视图
CREATE OR REPLACE VIEW `v_config_overview` AS
SELECT 
    'nodes' as `config_type`, 
    COUNT(*) as `total_count`, 
    SUM(CASE WHEN `is_active` = 1 THEN 1 ELSE 0 END) as `active_count`
FROM `blockchain_nodes`
UNION ALL
SELECT 
    'collector_configs', 
    COUNT(*), 
    SUM(CASE WHEN `is_active` = 1 THEN 1 ELSE 0 END)
FROM `collector_config`
UNION ALL
SELECT 
    'kafka_topics', 
    COUNT(*), 
    SUM(CASE WHEN `is_active` = 1 THEN 1 ELSE 0 END)
FROM `kafka_topics`;

-- ==================================================
-- 存储过程：获取有效配置
-- ==================================================
DELIMITER //
CREATE PROCEDURE `GetActiveNodeConfig`()
BEGIN
    SELECT * FROM `blockchain_nodes` 
    WHERE `is_active` = 1 
    ORDER BY `priority` ASC, `success_count` DESC;
END //

CREATE PROCEDURE `GetCollectorConfig`()
BEGIN
    SELECT `config_key`, `config_value`, `value_type` 
    FROM `collector_config` 
    WHERE `is_active` = 1;
END //

CREATE PROCEDURE `UpdateNodeStats`(
    IN `node_id` INT,
    IN `is_success` BOOLEAN,
    IN `response_time_ms` INT
)
BEGIN
    IF `is_success` THEN
        UPDATE `blockchain_nodes` 
        SET `success_count` = `success_count` + 1,
            `last_used_at` = NOW()
        WHERE `id` = `node_id`;
    ELSE
        UPDATE `blockchain_nodes` 
        SET `error_count` = `error_count` + 1,
            `last_used_at` = NOW()
        WHERE `id` = `node_id`;
    END IF;
END //
DELIMITER ;

SET FOREIGN_KEY_CHECKS = 1;