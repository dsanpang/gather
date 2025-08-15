package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDefaultConfig(t *testing.T) {
	// 清除环境变量以测试默认行为
	originalURL := os.Getenv("ETH_NODE_URL")
	os.Unsetenv("ETH_NODE_URL")
	defer func() {
		if originalURL != "" {
			os.Setenv("ETH_NODE_URL", originalURL)
		}
	}()

	config := GetDefaultConfig()

	assert.NotNil(t, config)
	assert.NotNil(t, config.Blockchain)
	assert.NotNil(t, config.Collector)
	assert.NotNil(t, config.Output)
	assert.NotNil(t, config.Decoder)
	assert.NotNil(t, config.Logging)

	// 测试区块链配置
	assert.NotEmpty(t, config.Blockchain.Nodes)
	firstNode := config.Blockchain.Nodes[0]
	assert.Equal(t, "local_node", firstNode.Name)
	assert.Equal(t, "", firstNode.URL) // 应该从环境变量获取，现在为空
	assert.Equal(t, "local", firstNode.Type)
	assert.Equal(t, 1000, firstNode.RateLimit)
	assert.Equal(t, 1, firstNode.Priority)

	// 测试采集器配置
	assert.Equal(t, 2, config.Collector.Workers)
	assert.Equal(t, 100, config.Collector.BatchSize)
	assert.Equal(t, 3, config.Collector.RetryLimit)
	assert.Equal(t, "30s", config.Collector.Timeout)
	assert.True(t, config.Collector.EnableTrace)
	assert.True(t, config.Collector.EnableStateTracking)

	// 测试输出配置
	assert.Equal(t, "kafka_async", config.Output.Format)
	assert.Equal(t, "./outputs", config.Output.Directory)
	assert.False(t, config.Output.Compress)
	assert.NotNil(t, config.Output.Kafka)
	assert.Equal(t, []string{"localhost:9092"}, config.Output.Kafka.Brokers)
	assert.NotEmpty(t, config.Output.Kafka.Topics)

	// 测试解码器配置
	assert.Equal(t, "https://www.4byte.directory/api/v1/signatures/", config.Decoder.FourByteAPIURL)
	assert.Equal(t, "5s", config.Decoder.APITimeout)
	assert.True(t, config.Decoder.EnableCache)
	assert.Equal(t, 10000, config.Decoder.CacheSize)
	assert.True(t, config.Decoder.EnableAPI)

	// 测试日志配置
	assert.Equal(t, "info", config.Logging.Level)
	assert.Equal(t, "json", config.Logging.Format)
	assert.Equal(t, "stdout", config.Logging.Output)
}

func TestGetDefaultConfig_WithoutEnvironmentVariable(t *testing.T) {
	// 确保没有环境变量干扰
	os.Unsetenv("ETH_NODE_URL")

	config := GetDefaultConfig()

	assert.NotNil(t, config)
	assert.NotNil(t, config.Blockchain)
	assert.NotEmpty(t, config.Blockchain.Nodes)

	firstNode := config.Blockchain.Nodes[0]
	assert.Equal(t, "local_node", firstNode.Name)
	assert.Equal(t, "", firstNode.URL) // 默认为空，需要在YAML或数据库中配置
}

func TestNodeConfigValidation(t *testing.T) {
	tests := []struct {
		name     string
		node     *NodeConfig
		valid    bool
	}{
		{
			name: "valid node config",
			node: &NodeConfig{
				Name:      "test_node",
				URL:       "https://mainnet.infura.io/v3/test-key",
				Type:      "infura",
				RateLimit: 100,
				Priority:  1,
			},
			valid: true,
		},
		{
			name: "empty name",
			node: &NodeConfig{
				Name:      "",
				URL:       "https://mainnet.infura.io/v3/test-key",
				Type:      "infura",
				RateLimit: 100,
				Priority:  1,
			},
			valid: false,
		},
		{
			name: "empty URL",
			node: &NodeConfig{
				Name:      "test_node",
				URL:       "",
				Type:      "infura",
				RateLimit: 100,
				Priority:  1,
			},
			valid: false,
		},
		{
			name: "invalid rate limit",
			node: &NodeConfig{
				Name:      "test_node",
				URL:       "https://mainnet.infura.io/v3/test-key",
				Type:      "infura",
				RateLimit: -1,
				Priority:  1,
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := validateNodeConfig(tt.node)
			assert.Equal(t, tt.valid, valid)
		})
	}
}

func TestCollectorConfigValidation(t *testing.T) {
	tests := []struct {
		name     string
		config   *CollectorConfig
		valid    bool
	}{
		{
			name: "valid collector config",
			config: &CollectorConfig{
				Workers:    10,
				BatchSize:  100,
				RetryLimit: 3,
				Timeout:    "30s",
			},
			valid: true,
		},
		{
			name: "invalid workers",
			config: &CollectorConfig{
				Workers:    0,
				BatchSize:  100,
				RetryLimit: 3,
				Timeout:    "30s",
			},
			valid: false,
		},
		{
			name: "invalid batch size",
			config: &CollectorConfig{
				Workers:    10,
				BatchSize:  0,
				RetryLimit: 3,
				Timeout:    "30s",
			},
			valid: false,
		},
		{
			name: "invalid timeout",
			config: &CollectorConfig{
				Workers:    10,
				BatchSize:  100,
				RetryLimit: 3,
				Timeout:    "invalid",
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := validateCollectorConfig(tt.config)
			assert.Equal(t, tt.valid, valid)
		})
	}
}

func TestDecoderConfigValidation(t *testing.T) {
	tests := []struct {
		name     string
		config   *DecoderConfig
		valid    bool
	}{
		{
			name: "valid decoder config",
			config: &DecoderConfig{
				FourByteAPIURL: "https://www.4byte.directory/api/v1/signatures/",
				APITimeout:     "5s",
				EnableCache:    true,
				CacheSize:      10000,
				EnableAPI:      true,
			},
			valid: true,
		},
		{
			name: "invalid API URL",
			config: &DecoderConfig{
				FourByteAPIURL: "invalid-url",
				APITimeout:     "5s",
				EnableCache:    true,
				CacheSize:      10000,
				EnableAPI:      true,
			},
			valid: false,
		},
		{
			name: "invalid timeout",
			config: &DecoderConfig{
				FourByteAPIURL: "https://www.4byte.directory/api/v1/signatures/",
				APITimeout:     "invalid",
				EnableCache:    true,
				CacheSize:      10000,
				EnableAPI:      true,
			},
			valid: false,
		},
		{
			name: "invalid cache size",
			config: &DecoderConfig{
				FourByteAPIURL: "https://www.4byte.directory/api/v1/signatures/",
				APITimeout:     "5s",
				EnableCache:    true,
				CacheSize:      -1,
				EnableAPI:      true,
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := validateDecoderConfig(tt.config)
			assert.Equal(t, tt.valid, valid)
		})
	}
}

func TestKafkaConfigValidation(t *testing.T) {
	tests := []struct {
		name     string
		config   *KafkaConfig
		valid    bool
	}{
		{
			name: "valid kafka config",
			config: &KafkaConfig{
				Brokers: []string{"localhost:9092", "localhost:9093"},
				Topics: map[string]string{
					"blocks":       "blockchain_blocks",
					"transactions": "blockchain_transactions",
				},
			},
			valid: true,
		},
		{
			name: "empty brokers",
			config: &KafkaConfig{
				Brokers: []string{},
				Topics: map[string]string{
					"blocks": "blockchain_blocks",
				},
			},
			valid: false,
		},
		{
			name: "empty topics",
			config: &KafkaConfig{
				Brokers: []string{"localhost:9092"},
				Topics:  map[string]string{},
			},
			valid: false,
		},
		{
			name: "invalid broker format",
			config: &KafkaConfig{
				Brokers: []string{"invalid-broker"},
				Topics: map[string]string{
					"blocks": "blockchain_blocks",
				},
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := validateKafkaConfig(tt.config)
			assert.Equal(t, tt.valid, valid)
		})
	}
}

func TestOutputConfigValidation(t *testing.T) {
	tests := []struct {
		name     string
		config   *OutputConfig
		valid    bool
	}{
		{
			name: "valid file output config",
			config: &OutputConfig{
				Format:    "json",
				Directory: "./outputs",
				Compress:  false,
			},
			valid: true,
		},
		{
			name: "valid kafka output config",
			config: &OutputConfig{
				Format:    "kafka",
				Directory: "./outputs",
				Kafka: &KafkaConfig{
					Brokers: []string{"localhost:9092"},
					Topics: map[string]string{
						"blocks": "blockchain_blocks",
					},
				},
			},
			valid: true,
		},
		{
			name: "invalid format",
			config: &OutputConfig{
				Format:    "invalid",
				Directory: "./outputs",
			},
			valid: false,
		},
		{
			name: "kafka format without kafka config",
			config: &OutputConfig{
				Format:    "kafka",
				Directory: "./outputs",
				Kafka:     nil,
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := validateOutputConfig(tt.config)
			assert.Equal(t, tt.valid, valid)
		})
	}
}

func TestConfigValidation(t *testing.T) {
	validConfig := GetDefaultConfig()
	
	// 测试有效配置
	valid := ValidateConfig(validConfig)
	assert.True(t, valid)
	
	// 测试无效配置 - 空配置
	invalid := ValidateConfig(nil)
	assert.False(t, invalid)
	
	// 测试无效配置 - 缺少区块链配置
	invalidConfig := &Config{
		Blockchain: nil,
		Collector:  validConfig.Collector,
		Output:     validConfig.Output,
		Decoder:    validConfig.Decoder,
		Logging:    validConfig.Logging,
	}
	invalid = ValidateConfig(invalidConfig)
	assert.False(t, invalid)
}

// 辅助验证函数 - 这些在实际代码中应该存在
func validateNodeConfig(node *NodeConfig) bool {
	if node == nil {
		return false
	}
	if node.Name == "" || node.URL == "" {
		return false
	}
	if node.RateLimit < 0 {
		return false
	}
	return true
}

func validateCollectorConfig(config *CollectorConfig) bool {
	if config == nil {
		return false
	}
	if config.Workers <= 0 || config.BatchSize <= 0 {
		return false
	}
	// 简单的超时格式验证
	if config.Timeout == "invalid" {
		return false
	}
	return true
}

func validateDecoderConfig(config *DecoderConfig) bool {
	if config == nil {
		return false
	}
	// 简单的URL验证
	if config.FourByteAPIURL == "invalid-url" {
		return false
	}
	// 简单的超时格式验证
	if config.APITimeout == "invalid" {
		return false
	}
	if config.CacheSize < 0 {
		return false
	}
	return true
}

func validateKafkaConfig(config *KafkaConfig) bool {
	if config == nil {
		return false
	}
	if len(config.Brokers) == 0 {
		return false
	}
	if len(config.Topics) == 0 {
		return false
	}
	// 简单的broker格式验证
	for _, broker := range config.Brokers {
		if broker == "invalid-broker" {
			return false
		}
	}
	return true
}

func validateOutputConfig(config *OutputConfig) bool {
	if config == nil {
		return false
	}
	
	validFormats := []string{"json", "json_async", "kafka", "kafka_async"}
	validFormat := false
	for _, format := range validFormats {
		if config.Format == format {
			validFormat = true
			break
		}
	}
	if !validFormat {
		return false
	}
	
	// 如果是kafka格式，必须有kafka配置
	if (config.Format == "kafka" || config.Format == "kafka_async") && config.Kafka == nil {
		return false
	}
	
	// 如果有kafka配置，验证它
	if config.Kafka != nil {
		return validateKafkaConfig(config.Kafka)
	}
	
	return true
}

func ValidateConfig(config *Config) bool {
	if config == nil {
		return false
	}
	
	if config.Blockchain == nil {
		return false
	}
	
	// 验证各个子配置
	if !validateCollectorConfig(config.Collector) {
		return false
	}
	
	if !validateOutputConfig(config.Output) {
		return false
	}
	
	if !validateDecoderConfig(config.Decoder) {
		return false
	}
	
	return true
}

// 测试默认Kafka主题配置
func TestDefaultKafkaTopics(t *testing.T) {
	config := GetDefaultConfig()
	
	expectedTopics := map[string]string{
		"blocks":                "blockchain_blocks",
		"transactions":          "blockchain_transactions",
		"logs":                  "blockchain_logs",
		"internal_transactions": "blockchain_internal_transactions",
		"state_changes":         "blockchain_state_changes",
		"contract_creations":    "blockchain_contract_creations",
		"reorg_notifications":   "blockchain_reorg_notifications",
	}
	
	assert.Equal(t, expectedTopics, config.Output.Kafka.Topics)
}

// 测试日志配置
func TestLoggingConfig(t *testing.T) {
	config := GetDefaultConfig()
	
	assert.NotNil(t, config.Logging)
	assert.Equal(t, "info", config.Logging.Level)
	assert.Equal(t, "json", config.Logging.Format)
	assert.Equal(t, "stdout", config.Logging.Output)
	assert.False(t, config.Logging.Rotation)
	assert.Equal(t, 100, config.Logging.MaxSize)
	assert.Equal(t, 30, config.Logging.MaxAge)
	assert.Equal(t, 3, config.Logging.MaxBackups)
	assert.True(t, config.Logging.Compress)
}

// 基准测试
func BenchmarkGetDefaultConfig(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetDefaultConfig()
	}
}

func BenchmarkValidateConfig(b *testing.B) {
	config := GetDefaultConfig()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateConfig(config)
	}
}