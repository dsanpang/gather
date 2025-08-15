package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// RetryConfig 重试配置
type RetryConfig struct {
	MaxAttempts      int           `json:"max_attempts"`       // 最大重试次数
	InitialInterval  time.Duration `json:"initial_interval"`   // 初始重试间隔
	MaxInterval      time.Duration `json:"max_interval"`       // 最大重试间隔
	BackoffFactor    float64       `json:"backoff_factor"`     // 退避因子
	RandomizationFactor float64    `json:"randomization_factor"` // 随机化因子
	EnableJitter     bool          `json:"enable_jitter"`      // 启用抖动
}

// DefaultRetryConfig 默认重试配置
var DefaultRetryConfig = &RetryConfig{
	MaxAttempts:         5,
	InitialInterval:     100 * time.Millisecond,
	MaxInterval:         30 * time.Second,
	BackoffFactor:       2.0,
	RandomizationFactor: 0.1,
	EnableJitter:        true,
}

// NetworkRetryConfig 网络请求重试配置
var NetworkRetryConfig = &RetryConfig{
	MaxAttempts:         3,
	InitialInterval:     500 * time.Millisecond,
	MaxInterval:         10 * time.Second,
	BackoffFactor:       2.0,
	RandomizationFactor: 0.2,
	EnableJitter:        true,
}

// CriticalRetryConfig 关键操作重试配置
var CriticalRetryConfig = &RetryConfig{
	MaxAttempts:         10,
	InitialInterval:     50 * time.Millisecond,
	MaxInterval:         60 * time.Second,
	BackoffFactor:       1.5,
	RandomizationFactor: 0.15,
	EnableJitter:        true,
}

// RetryableError 可重试错误接口
type RetryableError interface {
	error
	IsRetryable() bool
}

// RetryableErrorImpl 可重试错误实现
type RetryableErrorImpl struct {
	Err       error
	Retryable bool
}

func (r *RetryableErrorImpl) Error() string {
	return r.Err.Error()
}

func (r *RetryableErrorImpl) IsRetryable() bool {
	return r.Retryable
}

func (r *RetryableErrorImpl) Unwrap() error {
	return r.Err
}

// NewRetryableError 创建可重试错误
func NewRetryableError(err error, retryable bool) RetryableError {
	return &RetryableErrorImpl{
		Err:       err,
		Retryable: retryable,
	}
}

// IsRetryableError 判断是否为可重试错误
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	// 检查是否实现了RetryableError接口
	if retryableErr, ok := err.(RetryableError); ok {
		return retryableErr.IsRetryable()
	}
	
	// 检查常见的可重试错误
	errStr := strings.ToLower(err.Error())
	
	// 网络相关错误
	networkErrors := []string{
		"connection refused",
		"connection reset",
		"connection timeout",
		"timeout",
		"temporary failure",
		"service unavailable",
		"too many requests", // 429
		"rate limit",
		"i/o timeout",
		"no such host",
		"network is unreachable",
		"broken pipe",
	}
	
	for _, networkErr := range networkErrors {
		if strings.Contains(errStr, networkErr) {
			return true
		}
	}
	
	// Ethereum 节点相关错误
	ethereumErrors := []string{
		"node not ready",
		"execution reverted",
		"insufficient funds",
		"nonce too low",
		"replacement transaction underpriced",
		"already known",
		"future transaction",
	}
	
	for _, ethErr := range ethereumErrors {
		if strings.Contains(errStr, ethErr) {
			return true
		}
	}
	
	return false
}

// Retrier 重试器
type Retrier struct {
	config *RetryConfig
	logger *logrus.Logger
	rand   *rand.Rand
}

// NewRetrier 创建重试器
func NewRetrier(config *RetryConfig, logger *logrus.Logger) *Retrier {
	if config == nil {
		config = DefaultRetryConfig
	}
	
	return &Retrier{
		config: config,
		logger: logger,
		rand:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// ExecuteFunc 执行函数类型
type ExecuteFunc func() error

// Execute 执行重试逻辑
func (r *Retrier) Execute(ctx context.Context, operation string, fn ExecuteFunc) error {
	var lastErr error
	
	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		// 检查上下文是否被取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		// 执行操作
		err := fn()
		if err == nil {
			// 成功，记录重试统计
			if attempt > 1 {
				r.logger.Debugf("操作 '%s' 在第 %d 次尝试后成功", operation, attempt)
			}
			return nil
		}
		
		lastErr = err
		
		// 检查是否为可重试错误
		if !IsRetryableError(err) {
			r.logger.Debugf("操作 '%s' 失败且不可重试: %v", operation, err)
			return err
		}
		
		// 如果是最后一次尝试，直接返回错误
		if attempt == r.config.MaxAttempts {
			r.logger.Errorf("操作 '%s' 在 %d 次尝试后最终失败: %v", operation, attempt, err)
			return fmt.Errorf("重试 %d 次后失败: %w", attempt, err)
		}
		
		// 计算延迟时间
		delay := r.calculateDelay(attempt)
		r.logger.Debugf("操作 '%s' 第 %d 次失败: %v，%v 后重试", operation, attempt, err, delay)
		
		// 等待重试
		select {
		case <-time.After(delay):
			// 继续下一次重试
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	return lastErr
}

// ExecuteWithResult 执行重试逻辑并返回结果（简化版本，不使用泛型）
func (r *Retrier) ExecuteWithResult(ctx context.Context, operation string, fn func() (interface{}, error)) (interface{}, error) {
	var lastErr error
	
	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		// 检查上下文是否被取消
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		
		// 执行操作
		result, err := fn()
		if err == nil {
			// 成功，记录重试统计
			if attempt > 1 {
				r.logger.Debugf("操作 '%s' 在第 %d 次尝试后成功", operation, attempt)
			}
			return result, nil
		}
		
		lastErr = err
		
		// 检查是否为可重试错误
		if !IsRetryableError(err) {
			r.logger.Debugf("操作 '%s' 失败且不可重试: %v", operation, err)
			return nil, err
		}
		
		// 如果是最后一次尝试，直接返回错误
		if attempt == r.config.MaxAttempts {
			r.logger.Errorf("操作 '%s' 在 %d 次尝试后最终失败: %v", operation, attempt, err)
			return nil, fmt.Errorf("重试 %d 次后失败: %w", attempt, err)
		}
		
		// 计算延迟时间
		delay := r.calculateDelay(attempt)
		r.logger.Debugf("操作 '%s' 第 %d 次失败: %v，%v 后重试", operation, attempt, err, delay)
		
		// 等待重试
		select {
		case <-time.After(delay):
			// 继续下一次重试
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	
	return nil, lastErr
}

// calculateDelay 计算延迟时间
func (r *Retrier) calculateDelay(attempt int) time.Duration {
	// 指数退避计算
	delay := float64(r.config.InitialInterval) * math.Pow(r.config.BackoffFactor, float64(attempt-1))
	
	// 限制最大延迟
	if delay > float64(r.config.MaxInterval) {
		delay = float64(r.config.MaxInterval)
	}
	
	// 添加抖动避免惊群效应
	if r.config.EnableJitter {
		// 随机化因子应用
		jitter := delay * r.config.RandomizationFactor
		jitterRange := jitter * 2
		delay = delay - jitter + (r.rand.Float64() * jitterRange)
		
		// 确保延迟不为负数
		if delay < 0 {
			delay = float64(r.config.InitialInterval)
		}
	}
	
	return time.Duration(delay)
}

// GetConfig 获取重试配置
func (r *Retrier) GetConfig() *RetryConfig {
	return r.config
}

// UpdateConfig 更新重试配置
func (r *Retrier) UpdateConfig(config *RetryConfig) {
	if config != nil {
		r.config = config
	}
}

// RetryWithBackoff 简单的重试函数
func RetryWithBackoff(ctx context.Context, operation string, fn ExecuteFunc, logger *logrus.Logger) error {
	retrier := NewRetrier(DefaultRetryConfig, logger)
	return retrier.Execute(ctx, operation, fn)
}

// RetryNetworkOperation 网络操作重试
func RetryNetworkOperation(ctx context.Context, operation string, fn ExecuteFunc, logger *logrus.Logger) error {
	retrier := NewRetrier(NetworkRetryConfig, logger)
	return retrier.Execute(ctx, operation, fn)
}

// RetryCriticalOperation 关键操作重试
func RetryCriticalOperation(ctx context.Context, operation string, fn ExecuteFunc, logger *logrus.Logger) error {
	retrier := NewRetrier(CriticalRetryConfig, logger)
	return retrier.Execute(ctx, operation, fn)
}
