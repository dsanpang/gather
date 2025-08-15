package errors

import (
	"fmt"
	"time"
)

// ErrorType 错误类型
type ErrorType int

const (
	// 网络相关错误
	ErrorTypeNetwork ErrorType = iota
	ErrorTypeConnection
	ErrorTypeTimeout
	ErrorTypeRateLimit
	
	// 区块链相关错误
	ErrorTypeBlockchain
	ErrorTypeInvalidBlock
	ErrorTypeInvalidTransaction
	ErrorTypeReorg
	
	// 数据相关错误
	ErrorTypeData
	ErrorTypeSerialization
	ErrorTypeValidation
	
	// 系统相关错误
	ErrorTypeSystem
	ErrorTypeFileIO
	ErrorTypeMemory
	ErrorTypeConfig
	
	// 外部服务错误
	ErrorTypeExternalAPI
	ErrorTypeFourByte
	ErrorTypeKafka
)

// ErrorSeverity 错误严重级别
type ErrorSeverity int

const (
	SeverityLow ErrorSeverity = iota
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

// GatherError 自定义错误类型
type GatherError struct {
	Type        ErrorType     `json:"type"`
	Severity    ErrorSeverity `json:"severity"`
	Code        string        `json:"code"`
	Message     string        `json:"message"`
	Details     interface{}   `json:"details,omitempty"`
	Timestamp   time.Time     `json:"timestamp"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Cause       error         `json:"cause,omitempty"`
	Retryable   bool          `json:"retryable"`
	Component   string        `json:"component"`
	BlockNumber *uint64       `json:"block_number,omitempty"`
	TxHash      *string       `json:"tx_hash,omitempty"`
}

// Error 实现error接口
func (e *GatherError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 支持errors.Unwrap
func (e *GatherError) Unwrap() error {
	return e.Cause
}

// IsRetryable 判断是否可重试
func (e *GatherError) IsRetryable() bool {
	return e.Retryable
}

// WithContext 添加上下文信息
func (e *GatherError) WithContext(key string, value interface{}) *GatherError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithBlockNumber 添加区块号
func (e *GatherError) WithBlockNumber(blockNumber uint64) *GatherError {
	e.BlockNumber = &blockNumber
	return e
}

// WithTxHash 添加交易哈希
func (e *GatherError) WithTxHash(txHash string) *GatherError {
	e.TxHash = &txHash
	return e
}

// NewGatherError 创建新的错误
func NewGatherError(errorType ErrorType, severity ErrorSeverity, code, message string) *GatherError {
	return &GatherError{
		Type:      errorType,
		Severity:  severity,
		Code:      code,
		Message:   message,
		Timestamp: time.Now(),
		Retryable: determineRetryable(errorType, code),
	}
}

// WrapError 包装现有错误
func WrapError(err error, errorType ErrorType, severity ErrorSeverity, code, message string) *GatherError {
	return &GatherError{
		Type:      errorType,
		Severity:  severity,
		Code:      code,
		Message:   message,
		Timestamp: time.Now(),
		Cause:     err,
		Retryable: determineRetryable(errorType, code),
	}
}

// determineRetryable 根据错误类型判断是否可重试
func determineRetryable(errorType ErrorType, code string) bool {
	switch errorType {
	case ErrorTypeNetwork, ErrorTypeConnection, ErrorTypeTimeout:
		return true
	case ErrorTypeRateLimit:
		return true
	case ErrorTypeExternalAPI:
		return true
	case ErrorTypeKafka:
		return true
	case ErrorTypeBlockchain:
		// 大多数区块链错误可重试，除非是数据验证错误
		return code != "INVALID_DATA"
	default:
		return false
	}
}

// 预定义错误
var (
	// 网络错误
	ErrNetworkTimeout = NewGatherError(
		ErrorTypeTimeout,
		SeverityMedium,
		"NETWORK_TIMEOUT",
		"网络请求超时",
	)
	
	ErrConnectionFailed = NewGatherError(
		ErrorTypeConnection,
		SeverityHigh,
		"CONNECTION_FAILED",
		"连接失败",
	)
	
	ErrRateLimitExceeded = NewGatherError(
		ErrorTypeRateLimit,
		SeverityMedium,
		"RATE_LIMIT_EXCEEDED",
		"请求频率超限",
	)
	
	// 区块链错误
	ErrInvalidBlock = NewGatherError(
		ErrorTypeInvalidBlock,
		SeverityHigh,
		"INVALID_BLOCK",
		"无效的区块数据",
	)
	
	ErrBlockNotFound = NewGatherError(
		ErrorTypeBlockchain,
		SeverityMedium,
		"BLOCK_NOT_FOUND",
		"区块未找到",
	)
	
	ErrReorgDetected = NewGatherError(
		ErrorTypeReorg,
		SeverityCritical,
		"REORG_DETECTED",
		"检测到区块链重组",
	)
	
	// 数据错误
	ErrSerializationFailed = NewGatherError(
		ErrorTypeSerialization,
		SeverityMedium,
		"SERIALIZATION_FAILED",
		"数据序列化失败",
	)
	
	ErrDataValidation = NewGatherError(
		ErrorTypeValidation,
		SeverityMedium,
		"DATA_VALIDATION_FAILED",
		"数据验证失败",
	)
	
	// 系统错误
	ErrFileIOFailed = NewGatherError(
		ErrorTypeFileIO,
		SeverityHigh,
		"FILE_IO_FAILED",
		"文件操作失败",
	)
	
	ErrConfigInvalid = NewGatherError(
		ErrorTypeConfig,
		SeverityCritical,
		"CONFIG_INVALID",
		"配置无效",
	)
	
	// 外部服务错误
	ErrFourByteAPIFailed = NewGatherError(
		ErrorTypeFourByte,
		SeverityLow,
		"FOURBYTE_API_FAILED",
		"4byte.directory API调用失败",
	)
	
	ErrKafkaProduceFailed = NewGatherError(
		ErrorTypeKafka,
		SeverityHigh,
		"KAFKA_PRODUCE_FAILED",
		"Kafka消息发送失败",
	)
)

// 错误类型字符串映射
var errorTypeNames = map[ErrorType]string{
	ErrorTypeNetwork:       "Network",
	ErrorTypeConnection:    "Connection",
	ErrorTypeTimeout:       "Timeout",
	ErrorTypeRateLimit:     "RateLimit",
	ErrorTypeBlockchain:    "Blockchain",
	ErrorTypeInvalidBlock:  "InvalidBlock",
	ErrorTypeInvalidTransaction: "InvalidTransaction",
	ErrorTypeReorg:         "Reorg",
	ErrorTypeData:          "Data",
	ErrorTypeSerialization: "Serialization",
	ErrorTypeValidation:    "Validation",
	ErrorTypeSystem:        "System",
	ErrorTypeFileIO:        "FileIO",
	ErrorTypeMemory:        "Memory",
	ErrorTypeConfig:        "Config",
	ErrorTypeExternalAPI:   "ExternalAPI",
	ErrorTypeFourByte:      "FourByte",
	ErrorTypeKafka:         "Kafka",
}

// String 返回错误类型的字符串表示
func (et ErrorType) String() string {
	if name, exists := errorTypeNames[et]; exists {
		return name
	}
	return fmt.Sprintf("Unknown(%d)", et)
}

// 严重级别字符串映射
var severityNames = map[ErrorSeverity]string{
	SeverityLow:      "Low",
	SeverityMedium:   "Medium",
	SeverityHigh:     "High",
	SeverityCritical: "Critical",
}

// String 返回严重级别的字符串表示
func (es ErrorSeverity) String() string {
	if name, exists := severityNames[es]; exists {
		return name
	}
	return fmt.Sprintf("Unknown(%d)", es)
}

// ErrorStats 错误统计
type ErrorStats struct {
	TotalErrors    int                        `json:"total_errors"`
	ErrorsByType   map[ErrorType]int          `json:"errors_by_type"`
	ErrorsBySeverity map[ErrorSeverity]int    `json:"errors_by_severity"`
	ErrorsByComponent map[string]int           `json:"errors_by_component"`
	RecentErrors   []*GatherError             `json:"recent_errors"`
	LastError      *GatherError               `json:"last_error"`
	LastErrorTime  time.Time                  `json:"last_error_time"`
}

// NewErrorStats 创建错误统计
func NewErrorStats() *ErrorStats {
	return &ErrorStats{
		ErrorsByType:      make(map[ErrorType]int),
		ErrorsBySeverity:  make(map[ErrorSeverity]int),
		ErrorsByComponent: make(map[string]int),
		RecentErrors:      make([]*GatherError, 0),
	}
}

// RecordError 记录错误
func (es *ErrorStats) RecordError(err *GatherError) {
	es.TotalErrors++
	es.ErrorsByType[err.Type]++
	es.ErrorsBySeverity[err.Severity]++
	if err.Component != "" {
		es.ErrorsByComponent[err.Component]++
	}
	
	es.LastError = err
	es.LastErrorTime = err.Timestamp
	
	// 保留最近100个错误
	es.RecentErrors = append(es.RecentErrors, err)
	if len(es.RecentErrors) > 100 {
		es.RecentErrors = es.RecentErrors[1:]
	}
}

// GetErrorRate 获取错误率（错误/小时）
func (es *ErrorStats) GetErrorRate(duration time.Duration) float64 {
	if duration <= 0 {
		return 0
	}
	
	cutoff := time.Now().Add(-duration)
	recentCount := 0
	
	for _, err := range es.RecentErrors {
		if err.Timestamp.After(cutoff) {
			recentCount++
		}
	}
	
	hours := duration.Hours()
	if hours == 0 {
		return float64(recentCount)
	}
	
	return float64(recentCount) / hours
}