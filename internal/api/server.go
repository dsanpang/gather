package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"gather/internal/collector"
	"gather/internal/config"
	"gather/internal/output"
)

// Server API服务器
type Server struct {
	collector  *collector.Collector
	config     *config.Config
	outputter  output.Output
	logger     *logrus.Logger
	logManager *LogManager
	server     *http.Server
	mu         sync.RWMutex
	isRunning  bool
	isPaused   bool
	ctx        context.Context
	cancel     context.CancelFunc
	port       int
}

// NewServer 创建新的API服务器
func NewServer(cfg *config.Config, out output.Output, logger *logrus.Logger, port int) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	// 创建日志管理器
	logManager := NewLogManager(1000) // 最多保存1000条日志

	// 添加日志钩子
	logger.AddHook(NewLogHook(logManager))

	return &Server{
		config:     cfg,
		outputter:  out,
		logger:     logger,
		logManager: logManager,
		ctx:        ctx,
		cancel:     cancel,
		port:       port,
	}
}

// Start 启动API服务器
func (s *Server) Start() error {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// 添加CORS中间件
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// 添加中间件
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// 设置路由
	s.setupRoutes(router)

	// 创建HTTP服务器
	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: router,
	}

	s.logger.Infof("API服务器启动在端口 %d", s.port)
	return s.server.ListenAndServe()
}

// Stop 停止API服务器
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		s.cancel()
		s.isRunning = false
		s.logger.Info("采集任务已停止")
	}

	if s.server != nil {
		return s.server.Shutdown(context.Background())
	}
	return nil
}

// setupRoutes 设置路由
func (s *Server) setupRoutes(router *gin.Engine) {
	// 健康检查
	router.GET("/health", s.healthCheck)

	// 采集器控制
	api := router.Group("/api/v1")
	{
		// 采集器状态
		api.GET("/status", s.getStatus)

		// 采集器控制
		api.POST("/start", s.startCollection)
		api.POST("/stop", s.stopCollection)
		api.POST("/pause", s.pauseCollection)
		api.POST("/resume", s.resumeCollection)

		// 配置管理
		api.GET("/config", s.getConfig)
		api.PUT("/config", s.updateConfig)

		// 统计信息
		api.GET("/stats", s.getStats)

		// 日志管理
		api.GET("/logs", s.getLogs)
		api.DELETE("/logs", s.clearLogs)

		// 节点管理
		api.GET("/nodes", s.getNodes)
	}
}

// healthCheck 健康检查
func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"service":   "gather-api",
	})
}

// getStatus 获取采集器状态
func (s *Server) getStatus(c *gin.Context) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"running": s.isRunning,
		"paused":  s.isPaused,
		"status":  s.getStatusString(),
	})
}

// startCollection 启动采集
func (s *Server) startCollection(c *gin.Context) {
	var req struct {
		StartBlock uint64 `json:"start_block"`
		EndBlock   uint64 `json:"end_block"`
		Workers    int    `json:"workers"`
		BatchSize  int    `json:"batch_size"`
		Stream     bool   `json:"stream"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		c.JSON(http.StatusConflict, gin.H{"error": "采集器已在运行"})
		return
	}

	// 创建采集器
	s.collector = collector.NewCollector(s.config.Blockchain, s.outputter, s.logger)
	// 设置采集器配置
	if s.collector != nil {
		s.collector.SetCollectorConfig(s.config.Collector)
	}
	s.isRunning = true
	s.isPaused = false

	// 创建新的上下文用于采集任务
	collectCtx, collectCancel := context.WithCancel(context.Background())

	// 启动采集任务
	go func() {
		defer func() {
			s.mu.Lock()
			s.isRunning = false
			s.mu.Unlock()
			collectCancel() // 清理上下文
		}()

		if req.Stream {
			s.logger.Info("启动实时流采集")
			if err := s.collector.CollectStream(collectCtx); err != nil {
				s.logger.Errorf("流采集失败: %v", err)
			}
		} else {
			s.logger.Infof("启动批量采集: 区块 %d - %d", req.StartBlock, req.EndBlock)
			result, err := s.collector.CollectBatch(collectCtx, req.StartBlock, req.EndBlock, req.Workers, req.BatchSize)
			if err != nil {
				s.logger.Errorf("批量采集失败: %v", err)
			} else {
				s.logger.Infof("采集完成: %d 区块, %d 交易", result.ProcessedBlocks, result.TotalTransactions)
			}
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": "采集任务已启动",
		"status":  "started",
	})
}

// stopCollection 停止采集
func (s *Server) stopCollection(c *gin.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		c.JSON(http.StatusConflict, gin.H{"error": "采集器未在运行"})
		return
	}

	// 取消采集任务
	if s.collector != nil {
		s.collector.Close()
	}

	s.isRunning = false
	s.isPaused = false

	c.JSON(http.StatusOK, gin.H{
		"message": "采集任务已停止",
		"status":  "stopped",
	})
}

// pauseCollection 暂停采集
func (s *Server) pauseCollection(c *gin.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		c.JSON(http.StatusConflict, gin.H{"error": "采集器未在运行"})
		return
	}

	if s.isPaused {
		c.JSON(http.StatusConflict, gin.H{"error": "采集器已暂停"})
		return
	}

	s.isPaused = true
	c.JSON(http.StatusOK, gin.H{
		"message": "采集任务已暂停",
		"status":  "paused",
	})
}

// resumeCollection 恢复采集
func (s *Server) resumeCollection(c *gin.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		c.JSON(http.StatusConflict, gin.H{"error": "采集器未在运行"})
		return
	}

	if !s.isPaused {
		c.JSON(http.StatusConflict, gin.H{"error": "采集器未暂停"})
		return
	}

	s.isPaused = false
	c.JSON(http.StatusOK, gin.H{
		"message": "采集任务已恢复",
		"status":  "resumed",
	})
}

// getConfig 获取配置
func (s *Server) getConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"config": s.config,
	})
}

// updateConfig 更新配置
func (s *Server) updateConfig(c *gin.Context) {
	var newConfig config.BlockchainConfig
	if err := c.ShouldBindJSON(&newConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 更新配置
	s.config.Blockchain = &newConfig

	c.JSON(http.StatusOK, gin.H{
		"message": "配置已更新",
		"config":  s.config,
	})
}

// getStats 获取统计信息
func (s *Server) getStats(c *gin.Context) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := gin.H{
		"running": s.isRunning,
		"paused":  s.isPaused,
		"status":  s.getStatusString(),
		"uptime":  time.Since(time.Now()).String(), // 简化处理
	}

	c.JSON(http.StatusOK, stats)
}

// getStatusString 获取状态字符串
func (s *Server) getStatusString() string {
	if !s.isRunning {
		return "stopped"
	}
	if s.isPaused {
		return "paused"
	}
	return "running"
}

// getLogs 获取日志
func (s *Server) getLogs(c *gin.Context) {
	level := c.Query("level")
	pageStr := c.Query("page")
	pageSizeStr := c.Query("pageSize")

	page := 1 // 默认第1页
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	pageSize := 20 // 默认每页20条
	if pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	logs, total := s.logManager.GetLogsWithPagination(level, page, pageSize)

	c.JSON(http.StatusOK, gin.H{
		"logs":     logs,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
		"level":    level,
	})
}

// clearLogs 清空日志
func (s *Server) clearLogs(c *gin.Context) {
	s.logManager.ClearLogs()

	c.JSON(http.StatusOK, gin.H{
		"message": "日志已清空",
	})
}

// getNodes 获取节点状态
func (s *Server) getNodes(c *gin.Context) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.config == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "配置未初始化",
		})
		return
	}

	if s.config.Blockchain == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "区块链配置未加载",
		})
		return
	}

	if len(s.config.Blockchain.Nodes) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"nodes":   []gin.H{},
			"total":   0,
			"message": "未配置任何节点",
		})
		return
	}

	var nodes []gin.H
	for _, node := range s.config.Blockchain.Nodes {
		nodes = append(nodes, gin.H{
			"name":      node.Name,
			"type":      node.Type,
			"url":       node.URL,
			"available": true, // 暂时设为true，实际应该检查节点可用性
			"priority":  node.Priority,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"nodes":   nodes,
		"total":   len(nodes),
		"message": "多节点支持已启用",
	})
}
