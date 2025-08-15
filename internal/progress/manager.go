package progress

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

const (
	// 默认数据库路径
	DefaultDBPath = "./data/progress.db"
	
	// 存储桶名称
	ProgressBucket = "progress"
	ConfigBucket   = "config"
	StatsBucket    = "stats"
	
	// 进度键
	LastProcessedBlockKey = "last_processed_block"
	StartTimeKey         = "start_time"
	LastUpdateTimeKey    = "last_update_time"
)

// ProgressInfo 进度信息
type ProgressInfo struct {
	LastProcessedBlock uint64    `json:"last_processed_block"`
	StartTime          time.Time `json:"start_time"`
	LastUpdateTime     time.Time `json:"last_update_time"`
	TotalBlocks        uint64    `json:"total_blocks"`
	TotalTransactions  uint64    `json:"total_transactions"`
	ProcessingRate     float64   `json:"processing_rate"` // 区块/秒
}

// Manager 进度管理器
type Manager struct {
	db     *bolt.DB
	logger *logrus.Logger
	dbPath string
	mu     sync.RWMutex
	
	// 内存缓存
	cache *ProgressInfo
}

// NewManager 创建进度管理器
func NewManager(dbPath string, logger *logrus.Logger) (*Manager, error) {
	if dbPath == "" {
		dbPath = DefaultDBPath
	}
	
	// 确保目录存在
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	
	// 打开BoltDB数据库
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("打开进度数据库失败: %w", err)
	}
	
	manager := &Manager{
		db:     db,
		logger: logger,
		dbPath: dbPath,
		cache:  &ProgressInfo{},
	}
	
	// 初始化数据库结构
	if err := manager.initDB(); err != nil {
		db.Close()
		return nil, fmt.Errorf("初始化数据库失败: %w", err)
	}
	
	// 加载缓存
	if err := manager.loadCache(); err != nil {
		logger.Warnf("加载进度缓存失败: %v", err)
	}
	
	logger.Infof("进度管理器已初始化，数据库路径: %s", dbPath)
	return manager, nil
}

// initDB 初始化数据库结构
func (m *Manager) initDB() error {
	return m.db.Update(func(tx *bolt.Tx) error {
		// 创建进度存储桶
		if _, err := tx.CreateBucketIfNotExists([]byte(ProgressBucket)); err != nil {
			return fmt.Errorf("创建进度存储桶失败: %w", err)
		}
		
		// 创建配置存储桶
		if _, err := tx.CreateBucketIfNotExists([]byte(ConfigBucket)); err != nil {
			return fmt.Errorf("创建配置存储桶失败: %w", err)
		}
		
		// 创建统计存储桶
		if _, err := tx.CreateBucketIfNotExists([]byte(StatsBucket)); err != nil {
			return fmt.Errorf("创建统计存储桶失败: %w", err)
		}
		
		return nil
	})
}

// loadCache 加载缓存
func (m *Manager) loadCache() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	return m.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(ProgressBucket))
		if bucket == nil {
			return nil
		}
		
		// 加载最后处理的区块
		if data := bucket.Get([]byte(LastProcessedBlockKey)); data != nil {
			m.cache.LastProcessedBlock = binary.BigEndian.Uint64(data)
		}
		
		// 加载开始时间
		if data := bucket.Get([]byte(StartTimeKey)); data != nil {
			var startTime time.Time
			if err := json.Unmarshal(data, &startTime); err == nil {
				m.cache.StartTime = startTime
			}
		}
		
		// 加载最后更新时间
		if data := bucket.Get([]byte(LastUpdateTimeKey)); data != nil {
			var lastUpdateTime time.Time
			if err := json.Unmarshal(data, &lastUpdateTime); err == nil {
				m.cache.LastUpdateTime = lastUpdateTime
			}
		}
		
		return nil
	})
}

// GetLastProcessedBlock 获取最后处理的区块号
func (m *Manager) GetLastProcessedBlock() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cache.LastProcessedBlock
}

// UpdateProgress 更新进度
func (m *Manager) UpdateProgress(blockNumber uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	now := time.Now()
	
	// 更新缓存
	m.cache.LastProcessedBlock = blockNumber
	m.cache.LastUpdateTime = now
	m.cache.TotalBlocks++
	
	// 如果是第一次更新，设置开始时间
	if m.cache.StartTime.IsZero() {
		m.cache.StartTime = now
	}
	
	// 计算处理速率
	if !m.cache.StartTime.IsZero() {
		duration := now.Sub(m.cache.StartTime).Seconds()
		if duration > 0 {
			m.cache.ProcessingRate = float64(m.cache.TotalBlocks) / duration
		}
	}
	
	// 持久化到数据库
	return m.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(ProgressBucket))
		if bucket == nil {
			return fmt.Errorf("进度存储桶不存在")
		}
		
		// 保存区块号
		blockData := make([]byte, 8)
		binary.BigEndian.PutUint64(blockData, blockNumber)
		if err := bucket.Put([]byte(LastProcessedBlockKey), blockData); err != nil {
			return fmt.Errorf("保存区块号失败: %w", err)
		}
		
		// 保存开始时间
		if startTimeData, err := json.Marshal(m.cache.StartTime); err == nil {
			bucket.Put([]byte(StartTimeKey), startTimeData)
		}
		
		// 保存最后更新时间
		if updateTimeData, err := json.Marshal(now); err == nil {
			bucket.Put([]byte(LastUpdateTimeKey), updateTimeData)
		}
		
		return nil
	})
}

// UpdateTransactionCount 更新交易数量
func (m *Manager) UpdateTransactionCount(count uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache.TotalTransactions += count
}

// GetProgress 获取进度信息
func (m *Manager) GetProgress() *ProgressInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// 返回副本
	return &ProgressInfo{
		LastProcessedBlock: m.cache.LastProcessedBlock,
		StartTime:          m.cache.StartTime,
		LastUpdateTime:     m.cache.LastUpdateTime,
		TotalBlocks:        m.cache.TotalBlocks,
		TotalTransactions:  m.cache.TotalTransactions,
		ProcessingRate:     m.cache.ProcessingRate,
	}
}

// SetStartBlock 设置起始区块（用于初始化）
func (m *Manager) SetStartBlock(blockNumber uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 只有在没有设置过的情况下才设置起始区块
	if m.cache.LastProcessedBlock == 0 {
		m.cache.LastProcessedBlock = blockNumber
		m.cache.StartTime = time.Now()
		
		return m.db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(ProgressBucket))
			if bucket == nil {
				return fmt.Errorf("进度存储桶不存在")
			}
			
			// 保存起始区块号
			blockData := make([]byte, 8)
			binary.BigEndian.PutUint64(blockData, blockNumber)
			if err := bucket.Put([]byte(LastProcessedBlockKey), blockData); err != nil {
				return fmt.Errorf("保存起始区块号失败: %w", err)
			}
			
			// 保存开始时间
			if startTimeData, err := json.Marshal(m.cache.StartTime); err == nil {
				bucket.Put([]byte(StartTimeKey), startTimeData)
			}
			
			return nil
		})
	}
	
	return nil
}

// Reset 重置进度
func (m *Manager) Reset() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.cache = &ProgressInfo{}
	
	return m.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(ProgressBucket))
		if bucket == nil {
			return nil
		}
		
		// 清空所有数据
		return bucket.ForEach(func(k, v []byte) error {
			return bucket.Delete(k)
		})
	})
}

// SaveCheckpoint 保存检查点（完整的进度信息）
func (m *Manager) SaveCheckpoint(info *ProgressInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.cache = info
	
	return m.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(ProgressBucket))
		if bucket == nil {
			return fmt.Errorf("进度存储桶不存在")
		}
		
		// 保存区块号
		blockData := make([]byte, 8)
		binary.BigEndian.PutUint64(blockData, info.LastProcessedBlock)
		if err := bucket.Put([]byte(LastProcessedBlockKey), blockData); err != nil {
			return fmt.Errorf("保存区块号失败: %w", err)
		}
		
		// 保存开始时间
		if startTimeData, err := json.Marshal(info.StartTime); err == nil {
			bucket.Put([]byte(StartTimeKey), startTimeData)
		}
		
		// 保存最后更新时间
		if updateTimeData, err := json.Marshal(info.LastUpdateTime); err == nil {
			bucket.Put([]byte(LastUpdateTimeKey), updateTimeData)
		}
		
		return nil
	})
}

// GetDBPath 获取数据库路径
func (m *Manager) GetDBPath() string {
	return m.dbPath
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	info := m.GetProgress()
	
	stats := map[string]interface{}{
		"last_processed_block": info.LastProcessedBlock,
		"total_blocks":         info.TotalBlocks,
		"total_transactions":   info.TotalTransactions,
		"processing_rate":      fmt.Sprintf("%.2f blocks/sec", info.ProcessingRate),
		"start_time":           info.StartTime.Format(time.RFC3339),
		"last_update_time":     info.LastUpdateTime.Format(time.RFC3339),
	}
	
	if !info.StartTime.IsZero() {
		duration := time.Since(info.StartTime)
		stats["running_duration"] = duration.String()
	}
	
	return stats
}

// Close 关闭进度管理器
func (m *Manager) Close() error {
	if m.db != nil {
		m.logger.Info("关闭进度管理器")
		return m.db.Close()
	}
	return nil
}
