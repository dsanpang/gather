package config

import (
	"fmt"
	"os"

	"gather/internal/logging"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Config 主配置
type Config struct {
	Blockchain *BlockchainConfig  `mapstructure:"blockchain"`
	Collector  *CollectorConfig   `mapstructure:"collector"`
	Output     *OutputConfig      `mapstructure:"output"`
	Decoder    *DecoderConfig     `mapstructure:"decoder"`
	Logging    *logging.LogConfig `mapstructure:"logging"`
}

// BlockchainConfig 区块链配置
type BlockchainConfig struct {
	Nodes []*NodeConfig `mapstructure:"nodes"`
}

// NodeConfig 节点配置
type NodeConfig struct {
	Name      string `mapstructure:"name"`
	URL       string `mapstructure:"url"`
	Type      string `mapstructure:"type"`
	RateLimit int    `mapstructure:"rate_limit"`
	Priority  int    `mapstructure:"priority"`
}

// CollectorConfig 采集器配置
type CollectorConfig struct {
	Workers             int    `mapstructure:"workers"`
	BatchSize           int    `mapstructure:"batch_size"`
	RetryLimit          int    `mapstructure:"retry_limit"`
	Timeout             string `mapstructure:"timeout"`
	EnableTrace         bool   `mapstructure:"enable_trace"`
	EnableStateTracking bool   `mapstructure:"enable_state_tracking"`
}

// KafkaConfig Kafka配置
type KafkaConfig struct {
	Brokers []string          `mapstructure:"brokers"`
	Topics  map[string]string `mapstructure:"topics"`
}

// DecoderConfig 解码器配置
type DecoderConfig struct {
	FourByteAPIURL string `mapstructure:"fourbyte_api_url"`
	APITimeout     string `mapstructure:"api_timeout"`
	EnableCache    bool   `mapstructure:"enable_cache"`
	CacheSize      int    `mapstructure:"cache_size"`
	EnableAPI      bool   `mapstructure:"enable_api"`
}

// OutputConfig 输出配置
type OutputConfig struct {
	Format    string       `mapstructure:"format"`
	Directory string       `mapstructure:"directory"`
	Compress  bool         `mapstructure:"compress"`
	Kafka     *KafkaConfig `mapstructure:"kafka"`
}

// LoadConfig 加载配置（自动检测配置源）
func LoadConfig(configPath string) (*Config, error) {
	// 首先尝试从环境变量获取数据库配置
	dbDSN := os.Getenv("GATHER_DB_DSN")
	if dbDSN != "" {
		logger := logrus.New()
		dbConfig, err := NewDatabaseConfig(dbDSN, logger)
		if err != nil {
			return nil, fmt.Errorf("连接数据库失败: %w", err)
		}
		defer dbConfig.Close()

		config, err := dbConfig.LoadConfig()
		if err != nil {
			return nil, fmt.Errorf("从数据库加载配置失败: %w", err)
		}

		logger.Info("已从数据库加载配置")
		return config, nil
	}

	// 检查是否存在数据库配置文件
	dbConfigFile := "configs/database.yaml"
	if _, err := os.Stat(dbConfigFile); err == nil {
		// 读取数据库配置文件
		dbViper := viper.New()
		dbViper.SetConfigFile(dbConfigFile)
		dbViper.SetConfigType("yaml")

		if err := dbViper.ReadInConfig(); err == nil {
			dbDSN := dbViper.GetString("database.dsn")
			if dbDSN != "" {
				logger := logrus.New()
				dbConfig, err := NewDatabaseConfig(dbDSN, logger)
				if err == nil {
					defer dbConfig.Close()

					config, err := dbConfig.LoadConfig()
					if err == nil {
						logger.Info("已从数据库加载配置")
						return config, nil
					}
				}
			}
		}
	}

	// 如果数据库配置不可用，回退到YAML文件
	return LoadConfigFromFile(configPath)
}

// LoadConfigFromFile 从文件加载配置
func LoadConfigFromFile(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return &config, nil
}

// GetDefaultConfig 获取默认配置
func GetDefaultConfig() *Config {
	return &Config{
		Blockchain: &BlockchainConfig{
			Nodes: []*NodeConfig{
				{
					Name:      "local_node",
					URL:       "", // 需要在YAML配置或数据库中指定
					Type:      "local",
					RateLimit: 1000,
					Priority:  1,
				},
			},
		},
		Collector: &CollectorConfig{
			Workers:             2,
			BatchSize:           100,
			RetryLimit:          3,
			Timeout:             "30s",
			EnableTrace:         true,
			EnableStateTracking: true,
		},
		Output: &OutputConfig{
			Format:    "kafka_async",
			Directory: "./outputs",
			Compress:  false,
			Kafka: &KafkaConfig{
				Brokers: []string{"localhost:9092"},
				Topics: map[string]string{
					"blocks":                "blockchain_blocks",
					"transactions":          "blockchain_transactions",
					"logs":                  "blockchain_logs",
					"internal_transactions": "blockchain_internal_transactions",
					"state_changes":         "blockchain_state_changes",
					"contract_creations":    "blockchain_contract_creations",
					"reorg_notifications":   "blockchain_reorg_notifications",
				},
			},
		},
		Decoder: &DecoderConfig{
			FourByteAPIURL: "https://www.4byte.directory/api/v1/signatures/",
			APITimeout:     "5s",
			EnableCache:    true,
			CacheSize:      10000,
			EnableAPI:      true,
		},
		Logging: &logging.LogConfig{
			Level:      "info",
			Format:     "json",
			Output:     "stdout",
			Rotation:   false,
			MaxSize:    100,
			MaxAge:     30,
			MaxBackups: 3,
			Compress:   true,
		},
	}
}
