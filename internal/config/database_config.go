package config

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

// DatabaseConfig 数据库配置管理器
type DatabaseConfig struct {
	DB     *sql.DB
	logger *logrus.Logger
}

// NewDatabaseConfig 创建数据库配置管理器
func NewDatabaseConfig(dsn string, logger *logrus.Logger) (*DatabaseConfig, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	// 测试连接
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("数据库连接测试失败: %w", err)
	}

	return &DatabaseConfig{
		DB:     db,
		logger: logger,
	}, nil
}

// LoadConfig 从数据库加载完整配置
func (dc *DatabaseConfig) LoadConfig() (*Config, error) {
	config := &Config{}

	// 加载区块链节点配置
	blockchainConfig, err := dc.loadBlockchainConfig()
	if err != nil {
		return nil, fmt.Errorf("加载区块链配置失败: %w", err)
	}
	config.Blockchain = blockchainConfig

	// 加载采集器配置
	collectorConfig, err := dc.loadCollectorConfig()
	if err != nil {
		return nil, fmt.Errorf("加载采集器配置失败: %w", err)
	}
	config.Collector = collectorConfig

	// 加载输出配置
	outputConfig, err := dc.loadOutputConfig()
	if err != nil {
		return nil, fmt.Errorf("加载输出配置失败: %w", err)
	}
	config.Output = outputConfig

	return config, nil
}

// loadBlockchainConfig 加载区块链配置
func (dc *DatabaseConfig) loadBlockchainConfig() (*BlockchainConfig, error) {
	// 加载节点配置
	query := `SELECT name, url, node_type, rate_limit, priority FROM blockchain_nodes WHERE is_active = true ORDER BY priority`
	rows, err := dc.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []*NodeConfig
	for rows.Next() {
		var node NodeConfig
		err := rows.Scan(&node.Name, &node.URL, &node.Type, &node.RateLimit, &node.Priority)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, &node)
	}

	return &BlockchainConfig{
		Nodes: nodes,
	}, nil
}

// loadCollectorConfig 加载采集器配置
func (dc *DatabaseConfig) loadCollectorConfig() (*CollectorConfig, error) {
	query := `SELECT config_key, config_value FROM collector_config WHERE is_active = true`
	rows, err := dc.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	config := &CollectorConfig{
		Workers:             2,
		BatchSize:           100,
		RetryLimit:          3,
		Timeout:             "30s",
		EnableTrace:         false,
		EnableStateTracking: false,
	}

	for rows.Next() {
		var key, value string
		err := rows.Scan(&key, &value)
		if err != nil {
			return nil, err
		}

		switch key {
		case "workers":
			if v, err := strconv.Atoi(value); err == nil {
				config.Workers = v
			}
		case "batch_size":
			if v, err := strconv.Atoi(value); err == nil {
				config.BatchSize = v
			}
		case "retry_limit":
			if v, err := strconv.Atoi(value); err == nil {
				config.RetryLimit = v
			}
		case "timeout":
			config.Timeout = value
		case "enable_trace":
			config.EnableTrace = strings.ToLower(value) == "true"
		case "enable_state_tracking":
			config.EnableStateTracking = strings.ToLower(value) == "true"
		}
	}

	return config, nil
}

// loadOutputConfig 加载输出配置
func (dc *DatabaseConfig) loadOutputConfig() (*OutputConfig, error) {
	query := `SELECT config_key, config_value FROM output_config WHERE is_active = true`
	rows, err := dc.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	config := &OutputConfig{
		Format:    "json",
		Directory: "./outputs",
		Compress:  false,
	}

	for rows.Next() {
		var key, value string
		err := rows.Scan(&key, &value)
		if err != nil {
			return nil, err
		}

		switch key {
		case "format":
			config.Format = value
		case "compress":
			config.Compress = strings.ToLower(value) == "true"
		case "kafka_brokers":
			var brokers []string
			if err := json.Unmarshal([]byte(value), &brokers); err == nil {
				config.Kafka = &KafkaConfig{
					Brokers: brokers,
				}
			}
		}
	}

	// 加载Kafka主题配置
	if config.Format == "kafka" {
		topics, err := dc.loadKafkaTopics()
		if err != nil {
			return nil, err
		}
		if config.Kafka == nil {
			config.Kafka = &KafkaConfig{}
		}
		config.Kafka.Topics = topics
	}

	return config, nil
}

// loadKafkaTopics 加载Kafka主题配置
func (dc *DatabaseConfig) loadKafkaTopics() (map[string]string, error) {
	query := `SELECT data_type, topic_name FROM kafka_topics WHERE is_active = true`
	rows, err := dc.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	topics := make(map[string]string)
	for rows.Next() {
		var dataType, topicName string
		err := rows.Scan(&dataType, &topicName)
		if err != nil {
			return nil, err
		}
		topics[dataType] = topicName
	}

	return topics, nil
}

// UpdateConfig 更新配置
func (dc *DatabaseConfig) UpdateConfig(configType, key, value string) error {
	var tableName string
	switch configType {
	case "collector":
		tableName = "collector_config"
	case "output":
		tableName = "output_config"
	case "system":
		tableName = "system_config"
	default:
		return fmt.Errorf("不支持的配置类型: %s", configType)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (config_key, config_value, updated_at) 
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (config_key) 
		DO UPDATE SET config_value = $2, updated_at = CURRENT_TIMESTAMP
	`, tableName)

	_, err := dc.DB.Exec(query, key, value)
	return err
}

// GetConfig 获取配置值
func (dc *DatabaseConfig) GetConfig(configType, key string) (string, error) {
	var tableName string
	switch configType {
	case "collector":
		tableName = "collector_config"
	case "output":
		tableName = "output_config"
	case "system":
		tableName = "system_config"
	default:
		return "", fmt.Errorf("不支持的配置类型: %s", configType)
	}

	query := fmt.Sprintf(`SELECT config_value FROM %s WHERE config_key = $1 AND is_active = true`, tableName)
	var value string
	err := dc.DB.QueryRow(query, key).Scan(&value)
	return value, err
}

// ListConfigs 列出所有配置
func (dc *DatabaseConfig) ListConfigs(configType string) (map[string]string, error) {
	var tableName string
	switch configType {
	case "collector":
		tableName = "collector_config"
	case "output":
		tableName = "output_config"
	case "system":
		tableName = "system_config"
	default:
		return nil, fmt.Errorf("不支持的配置类型: %s", configType)
	}

	query := fmt.Sprintf(`SELECT config_key, config_value FROM %s WHERE is_active = true`, tableName)
	rows, err := dc.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	configs := make(map[string]string)
	for rows.Next() {
		var key, value string
		err := rows.Scan(&key, &value)
		if err != nil {
			return nil, err
		}
		configs[key] = value
	}

	return configs, nil
}

// Close 关闭数据库连接
func (dc *DatabaseConfig) Close() error {
	if dc.DB != nil {
		return dc.DB.Close()
	}
	return nil
}
