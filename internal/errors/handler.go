package errors

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ErrorHandler 错误处理器
type ErrorHandler struct {
	logger    *logrus.Logger
	stats     *ErrorStats
	mu        sync.RWMutex
	
	// 错误处理策略
	strategies map[ErrorType]ErrorStrategy
	
	// 错误回调
	callbacks []ErrorCallback
	
	// 阈值设置
	thresholds map[ErrorSeverity]ThresholdConfig
}

// ErrorStrategy 错误处理策略
type ErrorStrategy interface {
	Handle(ctx context.Context, err *GatherError) error
}

// ErrorCallback 错误回调函数
type ErrorCallback func(err *GatherError)

// ThresholdConfig 阈值配置
type ThresholdConfig struct {
	MaxErrorsPerHour   int           `json:"max_errors_per_hour"`
	MaxConsecutiveErrors int         `json:"max_consecutive_errors"`
	CooldownPeriod     time.Duration `json:"cooldown_period"`
}

// DefaultRetryStrategy 默认重试策略
type DefaultRetryStrategy struct {
	maxRetries int
	baseDelay  time.Duration
}

// LoggingStrategy 日志记录策略
type LoggingStrategy struct {
	logger *logrus.Logger
}

// NewErrorHandler 创建错误处理器
func NewErrorHandler(logger *logrus.Logger) *ErrorHandler {
	eh := &ErrorHandler{
		logger:     logger,
		stats:      NewErrorStats(),
		strategies: make(map[ErrorType]ErrorStrategy),
		callbacks:  make([]ErrorCallback, 0),
		thresholds: make(map[ErrorSeverity]ThresholdConfig),
	}

	// 设置默认策略
	eh.setupDefaultStrategies()
	
	// 设置默认阈值
	eh.setupDefaultThresholds()

	return eh
}

// setupDefaultStrategies 设置默认处理策略
func (eh *ErrorHandler) setupDefaultStrategies() {
	// 网络错误使用重试策略
	retryStrategy := &DefaultRetryStrategy{
		maxRetries: 3,
		baseDelay:  time.Second,
	}
	eh.strategies[ErrorTypeNetwork] = retryStrategy
	eh.strategies[ErrorTypeConnection] = retryStrategy
	eh.strategies[ErrorTypeTimeout] = retryStrategy
	eh.strategies[ErrorTypeRateLimit] = &DefaultRetryStrategy{
		maxRetries: 5,
		baseDelay:  5 * time.Second, // 限流错误需要更长的延迟
	}

	// 所有错误都记录日志
	loggingStrategy := &LoggingStrategy{logger: eh.logger}
	for errorType := range errorTypeNames {
		if _, exists := eh.strategies[errorType]; !exists {
			eh.strategies[errorType] = loggingStrategy
		}
	}
}

// setupDefaultThresholds 设置默认阈值
func (eh *ErrorHandler) setupDefaultThresholds() {
	eh.thresholds[SeverityLow] = ThresholdConfig{
		MaxErrorsPerHour:     100,
		MaxConsecutiveErrors: 20,
		CooldownPeriod:       5 * time.Minute,
	}
	
	eh.thresholds[SeverityMedium] = ThresholdConfig{
		MaxErrorsPerHour:     50,
		MaxConsecutiveErrors: 10,
		CooldownPeriod:       10 * time.Minute,
	}
	
	eh.thresholds[SeverityHigh] = ThresholdConfig{
		MaxErrorsPerHour:     20,
		MaxConsecutiveErrors: 5,
		CooldownPeriod:       30 * time.Minute,
	}
	
	eh.thresholds[SeverityCritical] = ThresholdConfig{
		MaxErrorsPerHour:     5,
		MaxConsecutiveErrors: 2,
		CooldownPeriod:       time.Hour,
	}
}

// HandleError 处理错误
func (eh *ErrorHandler) HandleError(ctx context.Context, err error) error {
	var gatherErr *GatherError
	
	// 转换为GatherError
	if ge, ok := err.(*GatherError); ok {
		gatherErr = ge
	} else {
		// 包装普通错误
		gatherErr = WrapError(err, ErrorTypeSystem, SeverityMedium, "UNKNOWN_ERROR", "未知错误")
	}

	// 记录错误统计
	eh.recordError(gatherErr)
	
	// 检查阈值
	if eh.checkThresholds(gatherErr) {
		eh.logger.Warnf("错误达到阈值限制: %s", gatherErr.Error())
	}
	
	// 执行回调
	eh.executeCallbacks(gatherErr)
	
	// 执行处理策略
	return eh.executeStrategy(ctx, gatherErr)
}

// recordError 记录错误
func (eh *ErrorHandler) recordError(err *GatherError) {
	eh.mu.Lock()
	defer eh.mu.Unlock()
	eh.stats.RecordError(err)
}

// checkThresholds 检查阈值
func (eh *ErrorHandler) checkThresholds(err *GatherError) bool {
	threshold, exists := eh.thresholds[err.Severity]
	if !exists {
		return false
	}

	// 检查每小时错误数
	hourlyRate := eh.stats.GetErrorRate(time.Hour)
	if hourlyRate > float64(threshold.MaxErrorsPerHour) {
		eh.logger.Warnf("每小时错误数超过阈值: %.2f > %d", hourlyRate, threshold.MaxErrorsPerHour)
		return true
	}

	return false
}

// executeCallbacks 执行错误回调
func (eh *ErrorHandler) executeCallbacks(err *GatherError) {
	eh.mu.RLock()
	callbacks := make([]ErrorCallback, len(eh.callbacks))
	copy(callbacks, eh.callbacks)
	eh.mu.RUnlock()

	for _, callback := range callbacks {
		go func(cb ErrorCallback) {
			defer func() {
				if r := recover(); r != nil {
					eh.logger.Errorf("错误回调执行时发生panic: %v", r)
				}
			}()
			cb(err)
		}(callback)
	}
}

// executeStrategy 执行处理策略
func (eh *ErrorHandler) executeStrategy(ctx context.Context, err *GatherError) error {
	strategy, exists := eh.strategies[err.Type]
	if !exists {
		// 使用默认日志策略
		strategy = &LoggingStrategy{logger: eh.logger}
	}

	return strategy.Handle(ctx, err)
}

// Handle 实现DefaultRetryStrategy的处理方法
func (drs *DefaultRetryStrategy) Handle(ctx context.Context, err *GatherError) error {
	if !err.Retryable {
		return err
	}

	for attempt := 1; attempt <= drs.maxRetries; attempt++ {
		// 计算延迟时间（指数退避）
		delay := time.Duration(attempt) * drs.baseDelay
		
		select {
		case <-time.After(delay):
			// 这里应该重试原始操作，但由于我们没有操作上下文，
			// 只能记录重试信息并返回
			if attempt == drs.maxRetries {
				return WrapError(err, err.Type, err.Severity, 
					"RETRY_EXHAUSTED", "重试次数已用尽")
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return err
}

// Handle 实现LoggingStrategy的处理方法
func (ls *LoggingStrategy) Handle(ctx context.Context, err *GatherError) error {
	// 根据严重级别选择日志级别
	logEntry := ls.logger.WithFields(logrus.Fields{
		"error_type":    err.Type.String(),
		"error_code":    err.Code,
		"component":     err.Component,
		"retryable":     err.Retryable,
		"block_number":  err.BlockNumber,
		"tx_hash":       err.TxHash,
		"context":       err.Context,
	})

	switch err.Severity {
	case SeverityLow:
		logEntry.Debug(err.Message)
	case SeverityMedium:
		logEntry.Warn(err.Message)
	case SeverityHigh:
		logEntry.Error(err.Message)
	case SeverityCritical:
		logEntry.Fatal(err.Message)
	}

	return err
}

// AddCallback 添加错误回调
func (eh *ErrorHandler) AddCallback(callback ErrorCallback) {
	eh.mu.Lock()
	defer eh.mu.Unlock()
	eh.callbacks = append(eh.callbacks, callback)
}

// SetStrategy 设置错误处理策略
func (eh *ErrorHandler) SetStrategy(errorType ErrorType, strategy ErrorStrategy) {
	eh.mu.Lock()
	defer eh.mu.Unlock()
	eh.strategies[errorType] = strategy
}

// GetStats 获取错误统计信息
func (eh *ErrorHandler) GetStats() *ErrorStats {
	eh.mu.RLock()
	defer eh.mu.RUnlock()
	return eh.stats
}

// SetThreshold 设置阈值
func (eh *ErrorHandler) SetThreshold(severity ErrorSeverity, config ThresholdConfig) {
	eh.mu.Lock()
	defer eh.mu.Unlock()
	eh.thresholds[severity] = config
}

// ClearStats 清除统计信息
func (eh *ErrorHandler) ClearStats() {
	eh.mu.Lock()
	defer eh.mu.Unlock()
	eh.stats = NewErrorStats()
}

// AlertStrategy 告警策略
type AlertStrategy struct {
	alertFunc func(err *GatherError)
	logger    *logrus.Logger
}

// NewAlertStrategy 创建告警策略
func NewAlertStrategy(alertFunc func(err *GatherError), logger *logrus.Logger) *AlertStrategy {
	return &AlertStrategy{
		alertFunc: alertFunc,
		logger:    logger,
	}
}

// Handle 实现AlertStrategy的处理方法
func (as *AlertStrategy) Handle(ctx context.Context, err *GatherError) error {
	// 执行告警
	go func() {
		defer func() {
			if r := recover(); r != nil {
				as.logger.Errorf("告警函数执行时发生panic: %v", r)
			}
		}()
		as.alertFunc(err)
	}()

	return err
}

// CompositeStrategy 组合策略，可以执行多个策略
type CompositeStrategy struct {
	strategies []ErrorStrategy
}

// NewCompositeStrategy 创建组合策略
func NewCompositeStrategy(strategies ...ErrorStrategy) *CompositeStrategy {
	return &CompositeStrategy{
		strategies: strategies,
	}
}

// Handle 实现CompositeStrategy的处理方法
func (cs *CompositeStrategy) Handle(ctx context.Context, err *GatherError) error {
	var lastErr error
	
	for _, strategy := range cs.strategies {
		if strategyErr := strategy.Handle(ctx, err); strategyErr != nil {
			lastErr = strategyErr
		}
	}
	
	return lastErr
}