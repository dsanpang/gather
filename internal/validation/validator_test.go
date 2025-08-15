package validation

import (
	"math/big"
	"strings"
	"testing"
	"time"

	"gather/pkg/models"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewValidator(t *testing.T) {
	logger := logrus.New()
	validator := NewValidator(logger, true)
	
	assert.NotNil(t, validator)
	assert.True(t, validator.strictMode)
	assert.NotNil(t, validator.rules)
	assert.Equal(t, 5, len(validator.rules)) // 默认注册的规则数量
}

func TestValidateBlock_ValidBlock(t *testing.T) {
	logger := logrus.New()
	validator := NewValidator(logger, false)
	
	block := &models.Block{
		Number:           1000,
		Hash:             "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		ParentHash:       "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		Timestamp:        time.Now(),
		Miner:            "0x1234567890abcdef1234567890abcdef12345678",
		GasLimit:         21000000,
		GasUsed:          15000000,
		Difficulty:       big.NewInt(1000000),
		TotalDifficulty:  big.NewInt(1000000000),
		BaseFeePerGas:    big.NewInt(20000000000),
		Size:             1024,
		TransactionCount: 150,
		LogCount:         50,
	}
	
	result := validator.ValidateBlock(block)
	
	assert.NotNil(t, result)
	assert.True(t, result.Valid)
	assert.Equal(t, "block", result.DataType)
	assert.Empty(t, result.Errors)
}

func TestValidateBlock_InvalidHash(t *testing.T) {
	logger := logrus.New()
	validator := NewValidator(logger, false)
	
	block := &models.Block{
		Number:     1000,
		Hash:       "invalid_hash", // 无效哈希
		ParentHash: "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		Timestamp:  time.Now(),
		Miner:      "0x1234567890abcdef1234567890abcdef12345678",
		GasLimit:   21000000,
		GasUsed:    15000000,
	}
	
	result := validator.ValidateBlock(block)
	
	assert.NotNil(t, result)
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
	assert.Equal(t, "INVALID_BLOCK_HASH", result.Errors[0].Code)
}

func TestValidateBlock_GasExceedsLimit(t *testing.T) {
	logger := logrus.New()
	validator := NewValidator(logger, false)
	
	block := &models.Block{
		Number:     1000,
		Hash:       "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		ParentHash: "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		Timestamp:  time.Now(),
		Miner:      "0x1234567890abcdef1234567890abcdef12345678",
		GasLimit:   21000000,
		GasUsed:    25000000, // 超过限制
	}
	
	result := validator.ValidateBlock(block)
	
	assert.NotNil(t, result)
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
	assert.Equal(t, "GAS_USED_EXCEEDS_LIMIT", result.Errors[0].Code)
}

func TestValidateBlock_NilBlock(t *testing.T) {
	logger := logrus.New()
	validator := NewValidator(logger, false)
	
	result := validator.ValidateBlock(nil)
	
	assert.NotNil(t, result)
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
	assert.Equal(t, "block", result.DataType)
}

func TestValidateTransaction_ValidTransaction(t *testing.T) {
	logger := logrus.New()
	validator := NewValidator(logger, false)
	
	tx := &models.Transaction{
		Hash:        "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		BlockNumber: 1000,
		From:        "0x1234567890abcdef1234567890abcdef12345678",
		To:          "0xabcdef1234567890abcdef1234567890abcdef12",
		Value:       big.NewInt(1000000000000000000), // 1 ETH
		Gas:         21000,
		GasUsed:     21000,
		GasPrice:    big.NewInt(20000000000), // 20 Gwei
		Nonce:       1,
		Status:      1,
		Type:        0,
		Timestamp:   time.Now(),
	}
	
	result := validator.ValidateTransaction(tx)
	
	assert.NotNil(t, result)
	assert.True(t, result.Valid)
	assert.Equal(t, "transaction", result.DataType)
	assert.Empty(t, result.Errors)
}

func TestValidateTransaction_InvalidAddress(t *testing.T) {
	logger := logrus.New()
	validator := NewValidator(logger, false)
	
	tx := &models.Transaction{
		Hash:      "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		From:      "invalid_address", // 无效地址
		To:        "0xabcdef1234567890abcdef1234567890abcdef12",
		Value:     big.NewInt(1000000000000000000),
		Gas:       21000,
		GasUsed:   21000,
		Timestamp: time.Now(),
	}
	
	result := validator.ValidateTransaction(tx)
	
	assert.NotNil(t, result)
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
	assert.Equal(t, "INVALID_FROM_ADDRESS", result.Errors[0].Code)
}

func TestValidateTransaction_NegativeValue(t *testing.T) {
	logger := logrus.New()
	validator := NewValidator(logger, false)
	
	tx := &models.Transaction{
		Hash:      "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		From:      "0x1234567890abcdef1234567890abcdef12345678",
		To:        "0xabcdef1234567890abcdef1234567890abcdef12",
		Value:     big.NewInt(-1000), // 负值
		Gas:       21000,
		GasUsed:   21000,
		Timestamp: time.Now(),
	}
	
	result := validator.ValidateTransaction(tx)
	
	assert.NotNil(t, result)
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
	assert.Equal(t, "NEGATIVE_VALUE", result.Errors[0].Code)
}

func TestValidateLog_ValidLog(t *testing.T) {
	logger := logrus.New()
	validator := NewValidator(logger, false)
	
	log := &models.TransactionLog{
		TransactionHash: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		BlockNumber:     1000,
		Address:         "0x1234567890abcdef1234567890abcdef12345678",
		Topics: []string{
			"0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			"0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		},
		Data:      "0x123456",
		LogIndex:  0,
		Removed:   false,
		Timestamp: time.Now(),
	}
	
	result := validator.ValidateLog(log)
	
	assert.NotNil(t, result)
	assert.True(t, result.Valid)
	assert.Equal(t, "log", result.DataType)
	assert.Empty(t, result.Errors)
}

func TestValidateLog_InvalidTopic(t *testing.T) {
	logger := logrus.New()
	validator := NewValidator(logger, false)
	
	log := &models.TransactionLog{
		TransactionHash: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		BlockNumber:     1000,
		Address:         "0x1234567890abcdef1234567890abcdef12345678",
		Topics: []string{
			"invalid_topic", // 无效主题
		},
		Data:      "0x123456",
		LogIndex:  0,
		Timestamp: time.Now(),
	}
	
	result := validator.ValidateLog(log)
	
	assert.NotNil(t, result)
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
	assert.Equal(t, "INVALID_TOPIC", result.Errors[0].Code)
}

func TestIsValidHash(t *testing.T) {
	tests := []struct {
		name     string
		hash     string
		expected bool
	}{
		{
			name:     "valid hash",
			hash:     "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			expected: true,
		},
		{
			name:     "invalid hash - no 0x prefix",
			hash:     "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			expected: false,
		},
		{
			name:     "invalid hash - too short",
			hash:     "0x123456",
			expected: false,
		},
		{
			name:     "invalid hash - too long",
			hash:     "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef12",
			expected: false,
		},
		{
			name:     "invalid hash - invalid characters",
			hash:     "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdeX",
			expected: false,
		},
		{
			name:     "empty hash",
			hash:     "",
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidHash(tt.hash)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsValidAddress(t *testing.T) {
	tests := []struct {
		name     string
		address  string
		expected bool
	}{
		{
			name:     "valid address",
			address:  "0x1234567890abcdef1234567890abcdef12345678",
			expected: true,
		},
		{
			name:     "valid address - uppercase",
			address:  "0x1234567890ABCDEF1234567890ABCDEF12345678",
			expected: true,
		},
		{
			name:     "valid address - mixed case",
			address:  "0x1234567890AbCdEf1234567890aBcDeF12345678",
			expected: true,
		},
		{
			name:     "empty address",
			address:  "",
			expected: true, // 空地址在某些情况下是有效的
		},
		{
			name:     "invalid address - no 0x prefix",
			address:  "1234567890abcdef1234567890abcdef12345678",
			expected: false,
		},
		{
			name:     "invalid address - too short",
			address:  "0x123456",
			expected: false,
		},
		{
			name:     "invalid address - too long",
			address:  "0x1234567890abcdef1234567890abcdef1234567890",
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidAddress(tt.address)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBlockValidationRule(t *testing.T) {
	rule := NewBlockValidationRule()
	
	assert.Equal(t, "block", rule.Name())
	assert.Equal(t, "区块数据验证规则", rule.Description())
	
	// 测试有效区块
	validBlock := &models.Block{
		Size:             1024,
		TransactionCount: 150,
	}
	
	err := rule.Validate(validBlock)
	assert.NoError(t, err)
	
	// 测试过大的区块
	largeBlock := &models.Block{
		Size:             50 * 1024 * 1024, // 50MB
		TransactionCount: 150,
	}
	
	err = rule.Validate(largeBlock)
	assert.Error(t, err)
	
	// 测试错误的数据类型
	err = rule.Validate("not a block")
	assert.Error(t, err)
}

func TestTransactionValidationRule(t *testing.T) {
	rule := NewTransactionValidationRule()
	
	assert.Equal(t, "transaction", rule.Name())
	assert.Equal(t, "交易数据验证规则", rule.Description())
	
	// 测试有效交易
	validTx := &models.Transaction{
		Type: 0,
	}
	
	err := rule.Validate(validTx)
	assert.NoError(t, err)
	
	// 测试未知交易类型
	unknownTypeTx := &models.Transaction{
		Type: 99,
	}
	
	err = rule.Validate(unknownTypeTx)
	assert.Error(t, err)
}

func TestValidateBlockUpgradeConsistency(t *testing.T) {
	logger := logrus.New()
	validator := NewValidator(logger, false)
	
	// 测试Shanghai升级前的区块
	preShanghai := &models.Block{
		Number: models.SHANGHAI_CAPELLA_HEIGHT - 1,
	}
	
	result := &ValidationResult{
		Valid:    true,
		Warnings: make([]string, 0),
	}
	
	validator.validateBlockUpgradeConsistency(preShanghai, result)
	assert.Empty(t, result.Warnings) // 升级前不应该有警告
	
	// 测试Shanghai升级后的区块但缺少withdrawals_count
	postShanghai := &models.Block{
		Number:           models.SHANGHAI_CAPELLA_HEIGHT + 1000,
		WithdrawalsCount: nil,
	}
	
	result = &ValidationResult{
		Valid:    true,
		Warnings: make([]string, 0),
	}
	
	validator.validateBlockUpgradeConsistency(postShanghai, result)
	assert.NotEmpty(t, result.Warnings)
	assert.Contains(t, result.Warnings[0], "withdrawals_count")
	
	// 测试Dencun升级后的区块但缺少blob字段
	postDencun := &models.Block{
		Number:      models.DENCUN_BLOB_HEIGHT + 1000,
		BlobGasUsed: nil,
	}
	
	result = &ValidationResult{
		Valid:    true,
		Warnings: make([]string, 0),
	}
	
	validator.validateBlockUpgradeConsistency(postDencun, result)
	assert.NotEmpty(t, result.Warnings)
	// 检查是否包含 blob_gas_used 相关的警告
	found := false
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "blob_gas_used") {
			found = true
			break
		}
	}
	assert.True(t, found, "应该包含blob_gas_used相关的警告")
}

func TestValidatorStrictMode(t *testing.T) {
	logger := logrus.New()
	
	// 测试严格模式
	strictValidator := NewValidator(logger, true)
	assert.True(t, strictValidator.strictMode)
	
	// 测试非严格模式
	lenientValidator := NewValidator(logger, false)
	assert.False(t, lenientValidator.strictMode)
	
	// 测试设置严格模式
	lenientValidator.SetStrictMode(true)
	assert.True(t, lenientValidator.strictMode)
}

func TestGetValidationStats(t *testing.T) {
	logger := logrus.New()
	validator := NewValidator(logger, true)
	
	stats := validator.GetValidationStats()
	
	assert.NotNil(t, stats)
	assert.Contains(t, stats, "strict_mode")
	assert.Contains(t, stats, "registered_rules")
	assert.Contains(t, stats, "error_stats")
	
	assert.Equal(t, true, stats["strict_mode"])
	assert.Equal(t, 5, stats["registered_rules"]) // 默认规则数量
}

// 基准测试
func BenchmarkValidateBlock(b *testing.B) {
	logger := logrus.New()
	validator := NewValidator(logger, false)
	
	block := &models.Block{
		Number:           1000,
		Hash:             "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		ParentHash:       "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		Timestamp:        time.Now(),
		Miner:            "0x1234567890abcdef1234567890abcdef12345678",
		GasLimit:         21000000,
		GasUsed:          15000000,
		Size:             1024,
		TransactionCount: 150,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.ValidateBlock(block)
	}
}

func BenchmarkValidateTransaction(b *testing.B) {
	logger := logrus.New()
	validator := NewValidator(logger, false)
	
	tx := &models.Transaction{
		Hash:        "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		From:        "0x1234567890abcdef1234567890abcdef12345678",
		To:          "0xabcdef1234567890abcdef1234567890abcdef12",
		Value:       big.NewInt(1000000000000000000),
		Gas:         21000,
		GasUsed:     21000,
		Timestamp:   time.Now(),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.ValidateTransaction(tx)
	}
}