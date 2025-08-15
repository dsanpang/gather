package api

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// LogEntry 日志条目
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// LogManager 日志管理器
type LogManager struct {
	logs    []LogEntry
	maxLogs int
	mu      sync.RWMutex
}

// NewLogManager 创建日志管理器
func NewLogManager(maxLogs int) *LogManager {
	return &LogManager{
		logs:    make([]LogEntry, 0, maxLogs),
		maxLogs: maxLogs,
	}
}

// AddLog 添加日志
func (lm *LogManager) AddLog(entry *logrus.Entry) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	logEntry := LogEntry{
		Timestamp: entry.Time,
		Level:     entry.Level.String(),
		Message:   entry.Message,
		Fields:    entry.Data,
	}

	// 添加到日志列表
	lm.logs = append(lm.logs, logEntry)

	// 如果超过最大数量，移除最旧的日志
	if len(lm.logs) > lm.maxLogs {
		lm.logs = lm.logs[1:]
	}
}

// GetLogs 获取日志
func (lm *LogManager) GetLogs(level string, limit int) []LogEntry {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	if limit <= 0 {
		limit = len(lm.logs)
	}

	if limit > len(lm.logs) {
		limit = len(lm.logs)
	}

	// 返回最新的日志
	logs := make([]LogEntry, limit)
	copy(logs, lm.logs[len(lm.logs)-limit:])

	// 如果指定了级别，过滤日志
	if level != "" {
		filtered := make([]LogEntry, 0)
		for _, log := range logs {
			if log.Level == level {
				filtered = append(filtered, log)
			}
		}
		return filtered
	}

	return logs
}

// GetLogsWithPagination 获取分页日志
func (lm *LogManager) GetLogsWithPagination(level string, page, pageSize int) ([]LogEntry, int) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	// 获取所有日志
	allLogs := make([]LogEntry, len(lm.logs))
	copy(allLogs, lm.logs)

	// 如果指定了级别，过滤日志
	if level != "" {
		filtered := make([]LogEntry, 0)
		for _, log := range allLogs {
			if log.Level == level {
				filtered = append(filtered, log)
			}
		}
		allLogs = filtered
	}

	total := len(allLogs)

	// 计算分页
	start := (page - 1) * pageSize
	end := start + pageSize

	if start >= total {
		return []LogEntry{}, total
	}

	if end > total {
		end = total
	}

	return allLogs[start:end], total
}

// ClearLogs 清空日志
func (lm *LogManager) ClearLogs() {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.logs = make([]LogEntry, 0, lm.maxLogs)
}

// LogHook 日志钩子
type LogHook struct {
	manager *LogManager
}

// NewLogHook 创建日志钩子
func NewLogHook(manager *LogManager) *LogHook {
	return &LogHook{manager: manager}
}

// Fire 实现 logrus.Hook 接口
func (h *LogHook) Fire(entry *logrus.Entry) error {
	h.manager.AddLog(entry)
	return nil
}

// Levels 实现 logrus.Hook 接口
func (h *LogHook) Levels() []logrus.Level {
	return logrus.AllLevels
}
