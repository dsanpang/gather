package errors

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewGatherError(t *testing.T) {
	err := NewGatherError(ErrorTypeNetwork, SeverityHigh, "TEST_ERROR", "测试错误")
	
	assert.NotNil(t, err)
	assert.Equal(t, ErrorTypeNetwork, err.Type)
	assert.Equal(t, SeverityHigh, err.Severity)
	assert.Equal(t, "TEST_ERROR", err.Code)
	assert.Equal(t, "测试错误", err.Message)
	assert.True(t, err.Retryable) // 网络错误默认可重试
	assert.False(t, err.Timestamp.IsZero())
}

func TestWrapError(t *testing.T) {
	originalErr := errors.New("原始错误")
	wrappedErr := WrapError(originalErr, ErrorTypeSystem, SeverityMedium, "WRAPPED_ERROR", "包装错误")
	
	assert.NotNil(t, wrappedErr)
	assert.Equal(t, ErrorTypeSystem, wrappedErr.Type)
	assert.Equal(t, SeverityMedium, wrappedErr.Severity)
	assert.Equal(t, "WRAPPED_ERROR", wrappedErr.Code)
	assert.Equal(t, "包装错误", wrappedErr.Message)
	assert.Equal(t, originalErr, wrappedErr.Cause)
	assert.Contains(t, wrappedErr.Error(), "原始错误")
}

func TestGatherError_Error(t *testing.T) {
	// 测试没有原因的错误
	err := NewGatherError(ErrorTypeData, SeverityLow, "TEST_CODE", "测试消息")
	expected := "[TEST_CODE] 测试消息"
	assert.Equal(t, expected, err.Error())
	
	// 测试有原因的错误
	originalErr := errors.New("原始错误")
	wrappedErr := WrapError(originalErr, ErrorTypeData, SeverityLow, "TEST_CODE", "测试消息")
	expectedWithCause := "[TEST_CODE] 测试消息: 原始错误"
	assert.Equal(t, expectedWithCause, wrappedErr.Error())
}

func TestGatherError_Unwrap(t *testing.T) {
	originalErr := errors.New("原始错误")
	wrappedErr := WrapError(originalErr, ErrorTypeSystem, SeverityMedium, "WRAPPED", "包装")
	
	unwrapped := wrappedErr.Unwrap()
	assert.Equal(t, originalErr, unwrapped)
	
	// 测试没有原因的错误
	standaloneErr := NewGatherError(ErrorTypeData, SeverityLow, "STANDALONE", "独立错误")
	assert.Nil(t, standaloneErr.Unwrap())
}

func TestGatherError_IsRetryable(t *testing.T) {
	// 可重试的错误
	retryableErr := NewGatherError(ErrorTypeNetwork, SeverityMedium, "NETWORK_ERROR", "网络错误")
	assert.True(t, retryableErr.IsRetryable())
	
	// 不可重试的错误
	nonRetryableErr := NewGatherError(ErrorTypeConfig, SeverityCritical, "CONFIG_ERROR", "配置错误")
	assert.False(t, nonRetryableErr.IsRetryable())
}

func TestGatherError_WithContext(t *testing.T) {
	err := NewGatherError(ErrorTypeBlockchain, SeverityMedium, "BLOCK_ERROR", "区块错误")
	
	err.WithContext("node_url", "https://mainnet.infura.io")
	err.WithContext("attempt", 3)
	
	assert.NotNil(t, err.Context)
	assert.Equal(t, "https://mainnet.infura.io", err.Context["node_url"])
	assert.Equal(t, 3, err.Context["attempt"])
}

func TestGatherError_WithBlockNumber(t *testing.T) {
	err := NewGatherError(ErrorTypeBlockchain, SeverityMedium, "BLOCK_ERROR", "区块错误")
	
	err.WithBlockNumber(1000000)
	
	assert.NotNil(t, err.BlockNumber)
	assert.Equal(t, uint64(1000000), *err.BlockNumber)
}

func TestGatherError_WithTxHash(t *testing.T) {
	err := NewGatherError(ErrorTypeInvalidTransaction, SeverityHigh, "TX_ERROR", "交易错误")
	
	txHash := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	err.WithTxHash(txHash)
	
	assert.NotNil(t, err.TxHash)
	assert.Equal(t, txHash, *err.TxHash)
}

func TestDetermineRetryable(t *testing.T) {
	tests := []struct {
		errorType ErrorType
		code      string
		expected  bool
	}{
		{ErrorTypeNetwork, "NETWORK_TIMEOUT", true},
		{ErrorTypeConnection, "CONNECTION_FAILED", true},
		{ErrorTypeTimeout, "REQUEST_TIMEOUT", true},
		{ErrorTypeRateLimit, "RATE_LIMIT_EXCEEDED", true},
		{ErrorTypeExternalAPI, "API_ERROR", true},
		{ErrorTypeKafka, "KAFKA_ERROR", true},
		{ErrorTypeBlockchain, "BLOCK_NOT_FOUND", true},
		{ErrorTypeBlockchain, "INVALID_DATA", false},
		{ErrorTypeConfig, "CONFIG_ERROR", false},
		{ErrorTypeFileIO, "FILE_ERROR", false},
	}
	
	for _, tt := range tests {
		result := determineRetryable(tt.errorType, tt.code)
		assert.Equal(t, tt.expected, result, "errorType=%v, code=%s", tt.errorType, tt.code)
	}
}

func TestErrorType_String(t *testing.T) {
	tests := []struct {
		errorType ErrorType
		expected  string
	}{
		{ErrorTypeNetwork, "Network"},
		{ErrorTypeConnection, "Connection"},
		{ErrorTypeTimeout, "Timeout"},
		{ErrorTypeBlockchain, "Blockchain"},
		{ErrorTypeData, "Data"},
		{ErrorTypeSystem, "System"},
		{ErrorType(999), "Unknown(999)"}, // 未知类型
	}
	
	for _, tt := range tests {
		result := tt.errorType.String()
		assert.Equal(t, tt.expected, result)
	}
}

func TestErrorSeverity_String(t *testing.T) {
	tests := []struct {
		severity ErrorSeverity
		expected string
	}{
		{SeverityLow, "Low"},
		{SeverityMedium, "Medium"},
		{SeverityHigh, "High"},
		{SeverityCritical, "Critical"},
		{ErrorSeverity(999), "Unknown(999)"}, // 未知严重级别
	}
	
	for _, tt := range tests {
		result := tt.severity.String()
		assert.Equal(t, tt.expected, result)
	}
}

func TestNewErrorStats(t *testing.T) {
	stats := NewErrorStats()
	
	assert.NotNil(t, stats)
	assert.Equal(t, 0, stats.TotalErrors)
	assert.NotNil(t, stats.ErrorsByType)
	assert.NotNil(t, stats.ErrorsBySeverity)
	assert.NotNil(t, stats.ErrorsByComponent)
	assert.NotNil(t, stats.RecentErrors)
	assert.Empty(t, stats.RecentErrors)
	assert.Nil(t, stats.LastError)
}

func TestErrorStats_RecordError(t *testing.T) {
	stats := NewErrorStats()
	
	err1 := NewGatherError(ErrorTypeNetwork, SeverityMedium, "NET_ERROR", "网络错误")
	err1.Component = "collector"
	
	err2 := NewGatherError(ErrorTypeBlockchain, SeverityHigh, "BLOCK_ERROR", "区块错误")
	err2.Component = "collector"
	
	err3 := NewGatherError(ErrorTypeNetwork, SeverityLow, "NET_TIMEOUT", "网络超时")
	err3.Component = "api"
	
	stats.RecordError(err1)
	stats.RecordError(err2)
	stats.RecordError(err3)
	
	assert.Equal(t, 3, stats.TotalErrors)
	assert.Equal(t, 2, stats.ErrorsByType[ErrorTypeNetwork])
	assert.Equal(t, 1, stats.ErrorsByType[ErrorTypeBlockchain])
	assert.Equal(t, 1, stats.ErrorsBySeverity[SeverityLow])
	assert.Equal(t, 1, stats.ErrorsBySeverity[SeverityMedium])
	assert.Equal(t, 1, stats.ErrorsBySeverity[SeverityHigh])
	assert.Equal(t, 2, stats.ErrorsByComponent["collector"])
	assert.Equal(t, 1, stats.ErrorsByComponent["api"])
	assert.Equal(t, err3, stats.LastError)
	assert.Equal(t, 3, len(stats.RecentErrors))
}

func TestErrorStats_RecordError_RecentErrorsLimit(t *testing.T) {
	stats := NewErrorStats()
	
	// 添加超过100个错误
	for i := 0; i < 150; i++ {
		err := NewGatherError(ErrorTypeNetwork, SeverityLow, "TEST_ERROR", "测试错误")
		stats.RecordError(err)
	}
	
	assert.Equal(t, 150, stats.TotalErrors)
	assert.Equal(t, 100, len(stats.RecentErrors)) // 应该限制在100个
}

func TestErrorStats_GetErrorRate(t *testing.T) {
	stats := NewErrorStats()
	
	now := time.Now()
	
	// 添加一些在过去1小时内的错误
	for i := 0; i < 10; i++ {
		err := NewGatherError(ErrorTypeNetwork, SeverityLow, "TEST_ERROR", "测试错误")
		err.Timestamp = now.Add(-time.Duration(i*5) * time.Minute) // 每5分钟一个错误
		stats.RecentErrors = append(stats.RecentErrors, err)
	}
	
	// 添加一些超过1小时的错误
	for i := 0; i < 5; i++ {
		err := NewGatherError(ErrorTypeNetwork, SeverityLow, "OLD_ERROR", "旧错误")
		err.Timestamp = now.Add(-time.Duration(70+i*10) * time.Minute) // 超过1小时前
		stats.RecentErrors = append(stats.RecentErrors, err)
	}
	
	// 测试1小时的错误率
	hourlyRate := stats.GetErrorRate(time.Hour)
	assert.Equal(t, 10.0, hourlyRate) // 应该只计算过去1小时内的10个错误
	
	// 测试0持续时间
	zeroRate := stats.GetErrorRate(0)
	assert.Equal(t, 0.0, zeroRate)
	
	// 测试30分钟的错误率
	halfHourRate := stats.GetErrorRate(30 * time.Minute)
	assert.Equal(t, 12.0, halfHourRate) // 30分钟内的6个错误 * 2 = 12错误/小时
}

func TestPredefinedErrors(t *testing.T) {
	// 测试预定义错误是否正确初始化
	assert.Equal(t, ErrorTypeTimeout, ErrNetworkTimeout.Type)
	assert.Equal(t, "NETWORK_TIMEOUT", ErrNetworkTimeout.Code)
	assert.True(t, ErrNetworkTimeout.Retryable)
	
	assert.Equal(t, ErrorTypeConnection, ErrConnectionFailed.Type)
	assert.Equal(t, "CONNECTION_FAILED", ErrConnectionFailed.Code)
	assert.True(t, ErrConnectionFailed.Retryable)
	
	assert.Equal(t, ErrorTypeReorg, ErrReorgDetected.Type)
	assert.Equal(t, SeverityCritical, ErrReorgDetected.Severity)
	assert.Equal(t, "REORG_DETECTED", ErrReorgDetected.Code)
	
	assert.Equal(t, ErrorTypeConfig, ErrConfigInvalid.Type)
	assert.Equal(t, SeverityCritical, ErrConfigInvalid.Severity)
	assert.False(t, ErrConfigInvalid.Retryable)
}

// 基准测试
func BenchmarkNewGatherError(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewGatherError(ErrorTypeNetwork, SeverityMedium, "BENCH_ERROR", "基准测试错误")
	}
}

func BenchmarkErrorStats_RecordError(b *testing.B) {
	stats := NewErrorStats()
	err := NewGatherError(ErrorTypeNetwork, SeverityMedium, "BENCH_ERROR", "基准测试错误")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stats.RecordError(err)
	}
}

func BenchmarkGatherError_Error(b *testing.B) {
	err := NewGatherError(ErrorTypeNetwork, SeverityMedium, "BENCH_ERROR", "基准测试错误")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.Error()
	}
}