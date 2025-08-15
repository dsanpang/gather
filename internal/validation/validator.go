package validation

import (
	"fmt"
	"regexp"
	"strings"

	"gather/internal/errors"
	"gather/pkg/models"

	"github.com/ethereum/go-ethereum/common"
	"github.com/sirupsen/logrus"
)

// Validator 数据验证器
type Validator struct {
	logger        *logrus.Logger
	strictMode    bool // 严格模式
	errorHandler  *errors.ErrorHandler
	rules         map[string]ValidationRule
}

// ValidationRule 验证规则接口
type ValidationRule interface {
	Validate(data interface{}) error
	Name() string
	Description() string
}

// ValidationResult 验证结果
type ValidationResult struct {
	Valid    bool                    `json:"valid"`
	Errors   []*errors.GatherError   `json:"errors,omitempty"`
	Warnings []string                `json:"warnings,omitempty"`
	DataType string                  `json:"data_type"`
}

// NewValidator 创建数据验证器
func NewValidator(logger *logrus.Logger, strictMode bool) *Validator {
	v := &Validator{
		logger:       logger,
		strictMode:   strictMode,
		errorHandler: errors.NewErrorHandler(logger),
		rules:        make(map[string]ValidationRule),
	}

	// 注册默认验证规则
	v.registerDefaultRules()

	return v
}

// registerDefaultRules 注册默认验证规则
func (v *Validator) registerDefaultRules() {
	// 区块验证规则
	v.AddRule(NewBlockValidationRule())
	
	// 交易验证规则
	v.AddRule(NewTransactionValidationRule())
	
	// 日志验证规则
	v.AddRule(NewLogValidationRule())
	
	// 地址验证规则
	v.AddRule(NewAddressValidationRule())
	
	// 哈希验证规则
	v.AddRule(NewHashValidationRule())
}

// AddRule 添加验证规则
func (v *Validator) AddRule(rule ValidationRule) {
	v.rules[rule.Name()] = rule
	v.logger.Debugf("已注册验证规则: %s", rule.Name())
}

// ValidateBlock 验证区块数据
func (v *Validator) ValidateBlock(block *models.Block) *ValidationResult {
	if block == nil {
		return &ValidationResult{
			Valid:    false,
			Errors:   []*errors.GatherError{errors.ErrDataValidation.WithContext("reason", "区块为空")},
			DataType: "block",
		}
	}

	result := &ValidationResult{
		Valid:    true,
		DataType: "block",
		Errors:   make([]*errors.GatherError, 0),
		Warnings: make([]string, 0),
	}

	// 执行基础验证
	if err := v.validateBlockBasics(block); err != nil {
		result.Valid = false
		if gatherErr, ok := err.(*errors.GatherError); ok {
			result.Errors = append(result.Errors, gatherErr.WithBlockNumber(block.Number))
		} else {
			result.Errors = append(result.Errors, errors.WrapError(err, 
				errors.ErrorTypeValidation, errors.SeverityMedium,
				"BLOCK_VALIDATION_FAILED", "区块验证失败").WithBlockNumber(block.Number))
		}
	}

	// 执行扩展验证规则
	if rule, exists := v.rules["block"]; exists {
		if err := rule.Validate(block); err != nil {
			result.Valid = false
			if gatherErr, ok := err.(*errors.GatherError); ok {
				result.Errors = append(result.Errors, gatherErr.WithBlockNumber(block.Number))
			} else {
				result.Errors = append(result.Errors, errors.WrapError(err, 
					errors.ErrorTypeValidation, errors.SeverityMedium,
					"BLOCK_RULE_VALIDATION_FAILED", "区块规则验证失败").WithBlockNumber(block.Number))
			}
		}
	}

	// 检查区块链升级相关的数据一致性
	v.validateBlockUpgradeConsistency(block, result)

	return result
}

// validateBlockBasics 验证区块基础数据
func (v *Validator) validateBlockBasics(block *models.Block) error {
	// 验证区块号
	if block.Number == 0 && block.Hash != "0x" + strings.Repeat("0", 64) {
		return errors.NewGatherError(errors.ErrorTypeValidation, errors.SeverityMedium,
			"INVALID_GENESIS_BLOCK", "创世区块格式错误")
	}

	// 验证哈希格式
	if !isValidHash(block.Hash) {
		return errors.NewGatherError(errors.ErrorTypeValidation, errors.SeverityHigh,
			"INVALID_BLOCK_HASH", "区块哈希格式无效")
	}

	if !isValidHash(block.ParentHash) {
		return errors.NewGatherError(errors.ErrorTypeValidation, errors.SeverityHigh,
			"INVALID_PARENT_HASH", "父区块哈希格式无效")
	}

	// 验证时间戳
	if block.Timestamp.IsZero() {
		return errors.NewGatherError(errors.ErrorTypeValidation, errors.SeverityMedium,
			"INVALID_TIMESTAMP", "区块时间戳无效")
	}

	// 验证Gas相关字段
	if block.GasUsed > block.GasLimit {
		return errors.NewGatherError(errors.ErrorTypeValidation, errors.SeverityHigh,
			"GAS_USED_EXCEEDS_LIMIT", "Gas使用量超过限制")
	}

	// 验证矿工地址
	if !isValidAddress(block.Miner) {
		return errors.NewGatherError(errors.ErrorTypeValidation, errors.SeverityMedium,
			"INVALID_MINER_ADDRESS", "矿工地址格式无效")
	}

	return nil
}

// validateBlockUpgradeConsistency 验证区块升级一致性
func (v *Validator) validateBlockUpgradeConsistency(block *models.Block, result *ValidationResult) {
	// 验证Shanghai升级后的数据
	if block.Number >= models.SHANGHAI_CAPELLA_HEIGHT {
		if block.WithdrawalsCount == nil {
			result.Warnings = append(result.Warnings, "Shanghai升级后的区块缺少withdrawals_count字段")
		}
	} else {
		if block.WithdrawalsCount != nil {
			result.Warnings = append(result.Warnings, "Shanghai升级前的区块不应有withdrawals_count字段")
		}
	}

	// 验证Dencun升级后的数据
	if block.Number >= models.DENCUN_BLOB_HEIGHT {
		if block.BlobGasUsed == nil {
			result.Warnings = append(result.Warnings, "Dencun升级后的区块缺少blob_gas_used字段")
		}
		if block.ExcessBlobGas == nil {
			result.Warnings = append(result.Warnings, "Dencun升级后的区块缺少excess_blob_gas字段")
		}
	} else {
		if block.BlobGasUsed != nil {
			result.Warnings = append(result.Warnings, "Dencun升级前的区块不应有blob_gas_used字段")
		}
	}
}

// ValidateTransaction 验证交易数据
func (v *Validator) ValidateTransaction(tx *models.Transaction) *ValidationResult {
	if tx == nil {
		return &ValidationResult{
			Valid:    false,
			Errors:   []*errors.GatherError{errors.ErrDataValidation.WithContext("reason", "交易为空")},
			DataType: "transaction",
		}
	}

	result := &ValidationResult{
		Valid:    true,
		DataType: "transaction",
		Errors:   make([]*errors.GatherError, 0),
		Warnings: make([]string, 0),
	}

	// 验证哈希
	if !isValidHash(tx.Hash) {
		result.Valid = false
		result.Errors = append(result.Errors, 
			errors.NewGatherError(errors.ErrorTypeValidation, errors.SeverityHigh,
				"INVALID_TX_HASH", "交易哈希格式无效").WithTxHash(tx.Hash))
	}

	// 验证地址
	if tx.From != "" && !isValidAddress(tx.From) {
		result.Valid = false
		result.Errors = append(result.Errors,
			errors.NewGatherError(errors.ErrorTypeValidation, errors.SeverityHigh,
				"INVALID_FROM_ADDRESS", "发送方地址格式无效").WithTxHash(tx.Hash))
	}

	if tx.To != "" && !isValidAddress(tx.To) {
		result.Valid = false
		result.Errors = append(result.Errors,
			errors.NewGatherError(errors.ErrorTypeValidation, errors.SeverityHigh,
				"INVALID_TO_ADDRESS", "接收方地址格式无效").WithTxHash(tx.Hash))
	}

	// 验证值
	if tx.Value == nil {
		result.Warnings = append(result.Warnings, "交易值为空")
	} else if tx.Value.Sign() < 0 {
		result.Valid = false
		result.Errors = append(result.Errors,
			errors.NewGatherError(errors.ErrorTypeValidation, errors.SeverityHigh,
				"NEGATIVE_VALUE", "交易值不能为负数").WithTxHash(tx.Hash))
	}

	// 验证Gas相关字段
	if tx.GasUsed > tx.Gas {
		result.Valid = false
		result.Errors = append(result.Errors,
			errors.NewGatherError(errors.ErrorTypeValidation, errors.SeverityHigh,
				"GAS_USED_EXCEEDS_LIMIT", "Gas使用量超过限制").WithTxHash(tx.Hash))
	}

	// 验证交易状态
	if tx.Status != 0 && tx.Status != 1 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("异常的交易状态: %d", tx.Status))
	}

	return result
}

// ValidateLog 验证日志数据
func (v *Validator) ValidateLog(log *models.TransactionLog) *ValidationResult {
	if log == nil {
		return &ValidationResult{
			Valid:    false,
			Errors:   []*errors.GatherError{errors.ErrDataValidation.WithContext("reason", "日志为空")},
			DataType: "log",
		}
	}

	result := &ValidationResult{
		Valid:    true,
		DataType: "log",
		Errors:   make([]*errors.GatherError, 0),
		Warnings: make([]string, 0),
	}

	// 验证交易哈希
	if !isValidHash(log.TransactionHash) {
		result.Valid = false
		result.Errors = append(result.Errors,
			errors.NewGatherError(errors.ErrorTypeValidation, errors.SeverityHigh,
				"INVALID_TX_HASH", "交易哈希格式无效").WithTxHash(log.TransactionHash))
	}

	// 验证合约地址
	if !isValidAddress(log.Address) {
		result.Valid = false
		result.Errors = append(result.Errors,
			errors.NewGatherError(errors.ErrorTypeValidation, errors.SeverityHigh,
				"INVALID_CONTRACT_ADDRESS", "合约地址格式无效").WithTxHash(log.TransactionHash))
	}

	// 验证主题
	for i, topic := range log.Topics {
		if !isValidHash(topic) {
			result.Valid = false
			result.Errors = append(result.Errors,
				errors.NewGatherError(errors.ErrorTypeValidation, errors.SeverityMedium,
					"INVALID_TOPIC", fmt.Sprintf("主题%d格式无效", i)).WithTxHash(log.TransactionHash))
		}
	}

	return result
}

// isValidHash 验证哈希格式
func isValidHash(hash string) bool {
	if len(hash) != 66 { // 0x + 64 hex chars
		return false
	}
	if !strings.HasPrefix(hash, "0x") {
		return false
	}
	
	hashRegex := regexp.MustCompile("^0x[0-9a-fA-F]{64}$")
	return hashRegex.MatchString(hash)
}

// isValidAddress 验证地址格式
func isValidAddress(addr string) bool {
	if addr == "" {
		return true // 空地址在某些情况下是有效的
	}
	
	// 检查是否以0x开头并且长度为42
	if !strings.HasPrefix(addr, "0x") {
		return false
	}
	
	return common.IsHexAddress(addr)
}

// BlockValidationRule 区块验证规则
type BlockValidationRule struct{}

func NewBlockValidationRule() *BlockValidationRule {
	return &BlockValidationRule{}
}

func (r *BlockValidationRule) Name() string {
	return "block"
}

func (r *BlockValidationRule) Description() string {
	return "区块数据验证规则"
}

func (r *BlockValidationRule) Validate(data interface{}) error {
	block, ok := data.(*models.Block)
	if !ok {
		return fmt.Errorf("数据类型不是区块")
	}

	// 验证区块大小合理性
	if block.Size > 30*1024*1024 { // 30MB
		return errors.NewGatherError(errors.ErrorTypeValidation, errors.SeverityMedium,
			"BLOCK_TOO_LARGE", "区块大小异常")
	}

	// 验证交易数量
	if block.TransactionCount < 0 {
		return errors.NewGatherError(errors.ErrorTypeValidation, errors.SeverityHigh,
			"INVALID_TX_COUNT", "交易数量无效")
	}

	return nil
}

// TransactionValidationRule 交易验证规则
type TransactionValidationRule struct{}

func NewTransactionValidationRule() *TransactionValidationRule {
	return &TransactionValidationRule{}
}

func (r *TransactionValidationRule) Name() string {
	return "transaction"
}

func (r *TransactionValidationRule) Description() string {
	return "交易数据验证规则"
}

func (r *TransactionValidationRule) Validate(data interface{}) error {
	tx, ok := data.(*models.Transaction)
	if !ok {
		return fmt.Errorf("数据类型不是交易")
	}

	// 验证交易类型
	if tx.Type > 5 { // 当前已知的最大交易类型
		return errors.NewGatherError(errors.ErrorTypeValidation, errors.SeverityMedium,
			"UNKNOWN_TX_TYPE", fmt.Sprintf("未知的交易类型: %d", tx.Type))
	}

	return nil
}

// LogValidationRule 日志验证规则
type LogValidationRule struct{}

func NewLogValidationRule() *LogValidationRule {
	return &LogValidationRule{}
}

func (r *LogValidationRule) Name() string {
	return "log"
}

func (r *LogValidationRule) Description() string {
	return "日志数据验证规则"
}

func (r *LogValidationRule) Validate(data interface{}) error {
	log, ok := data.(*models.TransactionLog)
	if !ok {
		return fmt.Errorf("数据类型不是日志")
	}

	// 验证主题数量
	if len(log.Topics) > 4 { // Solidity事件最多4个indexed参数
		return errors.NewGatherError(errors.ErrorTypeValidation, errors.SeverityMedium,
			"TOO_MANY_TOPICS", "日志主题数量过多")
	}

	return nil
}

// AddressValidationRule 地址验证规则
type AddressValidationRule struct{}

func NewAddressValidationRule() *AddressValidationRule {
	return &AddressValidationRule{}
}

func (r *AddressValidationRule) Name() string {
	return "address"
}

func (r *AddressValidationRule) Description() string {
	return "以太坊地址验证规则"
}

func (r *AddressValidationRule) Validate(data interface{}) error {
	addr, ok := data.(string)
	if !ok {
		return fmt.Errorf("数据类型不是字符串")
	}

	if !isValidAddress(addr) {
		return errors.NewGatherError(errors.ErrorTypeValidation, errors.SeverityHigh,
			"INVALID_ADDRESS_FORMAT", "地址格式无效")
	}

	return nil
}

// HashValidationRule 哈希验证规则
type HashValidationRule struct{}

func NewHashValidationRule() *HashValidationRule {
	return &HashValidationRule{}
}

func (r *HashValidationRule) Name() string {
	return "hash"
}

func (r *HashValidationRule) Description() string {
	return "哈希值验证规则"
}

func (r *HashValidationRule) Validate(data interface{}) error {
	hash, ok := data.(string)
	if !ok {
		return fmt.Errorf("数据类型不是字符串")
	}

	if !isValidHash(hash) {
		return errors.NewGatherError(errors.ErrorTypeValidation, errors.SeverityHigh,
			"INVALID_HASH_FORMAT", "哈希格式无效")
	}

	return nil
}

// GetValidationStats 获取验证统计信息
func (v *Validator) GetValidationStats() map[string]interface{} {
	return map[string]interface{}{
		"strict_mode":     v.strictMode,
		"registered_rules": len(v.rules),
		"error_stats":     v.errorHandler.GetStats(),
	}
}

// SetStrictMode 设置严格模式
func (v *Validator) SetStrictMode(strict bool) {
	v.strictMode = strict
	v.logger.Infof("验证器严格模式设置为: %t", strict)
}