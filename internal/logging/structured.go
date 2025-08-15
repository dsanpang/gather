package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

// LogConfig 日志配置
type LogConfig struct {
	Level      string `json:"level" yaml:"level"`             // 日志级别 (debug, info, warn, error)
	Format     string `json:"format" yaml:"format"`           // 日志格式 (json, text)
	Output     string `json:"output" yaml:"output"`           // 输出路径 (stdout, stderr, file path)
	Rotation   bool   `json:"rotation" yaml:"rotation"`       // 是否启用日志轮转
	MaxSize    int    `json:"max_size" yaml:"max_size"`       // 单个日志文件最大大小(MB)
	MaxAge     int    `json:"max_age" yaml:"max_age"`         // 日志文件保留天数
	MaxBackups int    `json:"max_backups" yaml:"max_backups"` // 保留的日志文件数量
	Compress   bool   `json:"compress" yaml:"compress"`       // 是否压缩轮转的日志文件
}

// DefaultLogConfig 默认日志配置
var DefaultLogConfig = &LogConfig{
	Level:      "info",
	Format:     "json",
	Output:     "stdout",
	Rotation:   false,
	MaxSize:    100,
	MaxAge:     30,
	MaxBackups: 3,
	Compress:   true,
}

// StructuredLogger 结构化日志器
type StructuredLogger struct {
	slogger *slog.Logger
	config  *LogConfig
	writer  io.Writer
}

// NewStructuredLogger 创建结构化日志器
func NewStructuredLogger(config *LogConfig) (*StructuredLogger, error) {
	if config == nil {
		config = DefaultLogConfig
	}

	// 解析日志级别
	level, err := parseLogLevel(config.Level)
	if err != nil {
		return nil, fmt.Errorf("无效的日志级别 '%s': %w", config.Level, err)
	}

	// 设置输出
	writer, err := getLogWriter(config)
	if err != nil {
		return nil, fmt.Errorf("创建日志输出失败: %w", err)
	}

	// 创建日志处理器
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level:       level,
		AddSource:   true, // 添加源码位置信息
		ReplaceAttr: replaceAttr,
	}

	switch config.Format {
	case "json":
		handler = slog.NewJSONHandler(writer, opts)
	case "text":
		handler = slog.NewTextHandler(writer, opts)
	default:
		return nil, fmt.Errorf("不支持的日志格式: %s", config.Format)
	}

	logger := slog.New(handler)

	return &StructuredLogger{
		slogger: logger,
		config:  config,
		writer:  writer,
	}, nil
}

// parseLogLevel 解析日志级别
func parseLogLevel(levelStr string) (slog.Level, error) {
	switch levelStr {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("未知的日志级别: %s", levelStr)
	}
}

// getLogWriter 获取日志输出
func getLogWriter(config *LogConfig) (io.Writer, error) {
	switch config.Output {
	case "stdout":
		return os.Stdout, nil
	case "stderr":
		return os.Stderr, nil
	default:
		// 文件输出
		dir := filepath.Dir(config.Output)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("创建日志目录失败: %w", err)
		}

		file, err := os.OpenFile(config.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("打开日志文件失败: %w", err)
		}

		return file, nil
	}
}

// replaceAttr 自定义属性替换函数
func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	// 自定义时间格式
	if a.Key == slog.TimeKey {
		return slog.Attr{
			Key:   a.Key,
			Value: slog.StringValue(a.Value.Time().Format(time.RFC3339)),
		}
	}

	// 简化源码路径
	if a.Key == slog.SourceKey {
		source := a.Value.Any().(*slog.Source)
		// 只保留文件名和行号
		source.File = filepath.Base(source.File)
	}

	return a
}

// Debug 调试日志
func (sl *StructuredLogger) Debug(msg string, args ...any) {
	sl.slogger.Debug(msg, args...)
}

// Debugf 格式化调试日志
func (sl *StructuredLogger) Debugf(format string, args ...any) {
	sl.slogger.Debug(fmt.Sprintf(format, args...))
}

// DebugWithFields 带字段的调试日志
func (sl *StructuredLogger) DebugWithFields(msg string, fields map[string]any) {
	sl.logWithFields(slog.LevelDebug, msg, fields)
}

// Info 信息日志
func (sl *StructuredLogger) Info(msg string, args ...any) {
	sl.slogger.Info(msg, args...)
}

// Infof 格式化信息日志
func (sl *StructuredLogger) Infof(format string, args ...any) {
	sl.slogger.Info(fmt.Sprintf(format, args...))
}

// InfoWithFields 带字段的信息日志
func (sl *StructuredLogger) InfoWithFields(msg string, fields map[string]any) {
	sl.logWithFields(slog.LevelInfo, msg, fields)
}

// Warn 警告日志
func (sl *StructuredLogger) Warn(msg string, args ...any) {
	sl.slogger.Warn(msg, args...)
}

// Warnf 格式化警告日志
func (sl *StructuredLogger) Warnf(format string, args ...any) {
	sl.slogger.Warn(fmt.Sprintf(format, args...))
}

// WarnWithFields 带字段的警告日志
func (sl *StructuredLogger) WarnWithFields(msg string, fields map[string]any) {
	sl.logWithFields(slog.LevelWarn, msg, fields)
}

// Error 错误日志
func (sl *StructuredLogger) Error(msg string, args ...any) {
	sl.slogger.Error(msg, args...)
}

// Errorf 格式化错误日志
func (sl *StructuredLogger) Errorf(format string, args ...any) {
	sl.slogger.Error(fmt.Sprintf(format, args...))
}

// ErrorWithFields 带字段的错误日志
func (sl *StructuredLogger) ErrorWithFields(msg string, fields map[string]any) {
	sl.logWithFields(slog.LevelError, msg, fields)
}

// logWithFields 带字段的日志记录
func (sl *StructuredLogger) logWithFields(level slog.Level, msg string, fields map[string]any) {
	attrs := make([]slog.Attr, 0, len(fields))
	for k, v := range fields {
		attrs = append(attrs, slog.Any(k, v))
	}
	sl.slogger.LogAttrs(context.Background(), level, msg, attrs...)
}

// WithContext 带上下文的日志器
func (sl *StructuredLogger) WithContext(ctx context.Context) *ContextLogger {
	return &ContextLogger{
		logger: sl.slogger,
		ctx:    ctx,
	}
}

// WithFields 带字段的日志器
func (sl *StructuredLogger) WithFields(fields map[string]any) *FieldLogger {
	// 转换为any类型的参数
	args := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}

	return &FieldLogger{
		logger: sl.slogger.With(args...),
	}
}

// GetSlogger 获取底层slog.Logger
func (sl *StructuredLogger) GetSlogger() *slog.Logger {
	return sl.slogger
}

// ContextLogger 带上下文的日志器
type ContextLogger struct {
	logger *slog.Logger
	ctx    context.Context
}

// Debug 调试日志
func (cl *ContextLogger) Debug(msg string, args ...any) {
	cl.logger.DebugContext(cl.ctx, msg, args...)
}

// Info 信息日志
func (cl *ContextLogger) Info(msg string, args ...any) {
	cl.logger.InfoContext(cl.ctx, msg, args...)
}

// Warn 警告日志
func (cl *ContextLogger) Warn(msg string, args ...any) {
	cl.logger.WarnContext(cl.ctx, msg, args...)
}

// Error 错误日志
func (cl *ContextLogger) Error(msg string, args ...any) {
	cl.logger.ErrorContext(cl.ctx, msg, args...)
}

// FieldLogger 带字段的日志器
type FieldLogger struct {
	logger *slog.Logger
}

// Debug 调试日志
func (fl *FieldLogger) Debug(msg string, args ...any) {
	fl.logger.Debug(msg, args...)
}

// Info 信息日志
func (fl *FieldLogger) Info(msg string, args ...any) {
	fl.logger.Info(msg, args...)
}

// Warn 警告日志
func (fl *FieldLogger) Warn(msg string, args ...any) {
	fl.logger.Warn(msg, args...)
}

// Error 错误日志
func (fl *FieldLogger) Error(msg string, args ...any) {
	fl.logger.Error(msg, args...)
}

// LogrusToSlogAdapter Logrus到Slog的适配器
type LogrusToSlogAdapter struct {
	slogger *StructuredLogger
}

// NewLogrusToSlogAdapter 创建适配器
func NewLogrusToSlogAdapter(slogger *StructuredLogger) *LogrusToSlogAdapter {
	return &LogrusToSlogAdapter{slogger: slogger}
}

// Debug 调试日志
func (a *LogrusToSlogAdapter) Debug(args ...interface{}) {
	a.slogger.Debug(fmt.Sprint(args...))
}

// Debugf 格式化调试日志
func (a *LogrusToSlogAdapter) Debugf(format string, args ...interface{}) {
	a.slogger.Debugf(format, args...)
}

// Info 信息日志
func (a *LogrusToSlogAdapter) Info(args ...interface{}) {
	a.slogger.Info(fmt.Sprint(args...))
}

// Infof 格式化信息日志
func (a *LogrusToSlogAdapter) Infof(format string, args ...interface{}) {
	a.slogger.Infof(format, args...)
}

// Warn 警告日志
func (a *LogrusToSlogAdapter) Warn(args ...interface{}) {
	a.slogger.Warn(fmt.Sprint(args...))
}

// Warnf 格式化警告日志
func (a *LogrusToSlogAdapter) Warnf(format string, args ...interface{}) {
	a.slogger.Warnf(format, args...)
}

// Error 错误日志
func (a *LogrusToSlogAdapter) Error(args ...interface{}) {
	a.slogger.Error(fmt.Sprint(args...))
}

// Errorf 格式化错误日志
func (a *LogrusToSlogAdapter) Errorf(format string, args ...interface{}) {
	a.slogger.Errorf(format, args...)
}

// SetLevel 设置日志级别（兼容性方法）
func (a *LogrusToSlogAdapter) SetLevel(level logrus.Level) {
	// 这里可以实现级别转换逻辑，但slog的级别在创建时设定
}

// SetFormatter 设置格式化器（兼容性方法）
func (a *LogrusToSlogAdapter) SetFormatter(formatter logrus.Formatter) {
	// slog的格式化在创建时设定，这里为了兼容性保留
}

// LoggerInterface 统一的日志接口
type LoggerInterface interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
}

// BlockLogger 区块处理专用日志器
func NewBlockLogger(baseLogger *StructuredLogger, blockNumber uint64) *FieldLogger {
	return baseLogger.WithFields(map[string]any{
		"component":    "block_processor",
		"block_number": blockNumber,
	})
}

// TransactionLogger 交易处理专用日志器
func NewTransactionLogger(baseLogger *StructuredLogger, blockNumber uint64, txHash string) *FieldLogger {
	return baseLogger.WithFields(map[string]any{
		"component":    "transaction_processor",
		"block_number": blockNumber,
		"tx_hash":      txHash,
	})
}

// RPCLogger RPC调用专用日志器
func NewRPCLogger(baseLogger *StructuredLogger, method string, nodeURL string) *FieldLogger {
	return baseLogger.WithFields(map[string]any{
		"component": "rpc_client",
		"method":    method,
		"node_url":  nodeURL,
	})
}
