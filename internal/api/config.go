package api

import (
	"net/http"

	"gather/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// ConfigManager 配置管理器
type ConfigManager struct {
	dbConfig *config.DatabaseConfig
	logger   *logrus.Logger
}

// NewConfigManager 创建配置管理器
func NewConfigManager(dbConfig *config.DatabaseConfig, logger *logrus.Logger) *ConfigManager {
	return &ConfigManager{
		dbConfig: dbConfig,
		logger:   logger,
	}
}

// GetConfig 获取配置
func (cm *ConfigManager) GetConfig(c *gin.Context) {
	configType := c.Param("type")
	key := c.Query("key")

	if key == "" {
		// 获取所有配置
		configs, err := cm.dbConfig.ListConfigs(configType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "获取配置失败",
				"message": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"config_type": configType,
			"configs":     configs,
		})
		return
	}

	// 获取单个配置
	value, err := cm.dbConfig.GetConfig(configType, key)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "配置不存在",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"config_type": configType,
		"key":         key,
		"value":       value,
	})
}

// UpdateConfig 更新配置
func (cm *ConfigManager) UpdateConfig(c *gin.Context) {
	configType := c.Param("type")

	var req struct {
		Key   string `json:"key" binding:"required"`
		Value string `json:"value" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "请求参数错误",
			"message": err.Error(),
		})
		return
	}

	err := cm.dbConfig.UpdateConfig(configType, req.Key, req.Value)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "更新配置失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "配置更新成功",
		"config": gin.H{
			"type":  configType,
			"key":   req.Key,
			"value": req.Value,
		},
	})
}

// GetBlockchainNodes 获取区块链节点配置
func (cm *ConfigManager) GetBlockchainNodes(c *gin.Context) {
	query := `SELECT id, name, url, node_type, rate_limit, priority, is_active FROM blockchain_nodes ORDER BY priority`
	rows, err := cm.dbConfig.DB.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "获取节点配置失败",
			"message": err.Error(),
		})
		return
	}
	defer rows.Close()

	var nodes []gin.H
	for rows.Next() {
		var id int
		var name, url, nodeType string
		var rateLimit, priority int
		var isActive bool

		err := rows.Scan(&id, &name, &url, &nodeType, &rateLimit, &priority, &isActive)
		if err != nil {
			continue
		}

		nodes = append(nodes, gin.H{
			"id":         id,
			"name":       name,
			"url":        url,
			"node_type":  nodeType,
			"rate_limit": rateLimit,
			"priority":   priority,
			"is_active":  isActive,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"nodes": nodes,
	})
}

// AddBlockchainNode 添加区块链节点
func (cm *ConfigManager) AddBlockchainNode(c *gin.Context) {
	var req struct {
		Name      string `json:"name" binding:"required"`
		URL       string `json:"url" binding:"required"`
		NodeType  string `json:"node_type" binding:"required"`
		RateLimit int    `json:"rate_limit"`
		Priority  int    `json:"priority"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "请求参数错误",
			"message": err.Error(),
		})
		return
	}

	query := `INSERT INTO blockchain_nodes (name, url, node_type, rate_limit, priority) VALUES ($1, $2, $3, $4, $5)`
	_, err := cm.dbConfig.DB.Exec(query, req.Name, req.URL, req.NodeType, req.RateLimit, req.Priority)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "添加节点失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "节点添加成功",
		"node":    req,
	})
}

// UpdateBlockchainNode 更新区块链节点
func (cm *ConfigManager) UpdateBlockchainNode(c *gin.Context) {
	nodeID := c.Param("id")

	var req struct {
		Name      string `json:"name"`
		URL       string `json:"url"`
		NodeType  string `json:"node_type"`
		RateLimit int    `json:"rate_limit"`
		Priority  int    `json:"priority"`
		IsActive  bool   `json:"is_active"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "请求参数错误",
			"message": err.Error(),
		})
		return
	}

	query := `UPDATE blockchain_nodes SET name = $1, url = $2, node_type = $3, rate_limit = $4, priority = $5, is_active = $6 WHERE id = $7`
	_, err := cm.dbConfig.DB.Exec(query, req.Name, req.URL, req.NodeType, req.RateLimit, req.Priority, req.IsActive, nodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "更新节点失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "节点更新成功",
	})
}

// DeleteBlockchainNode 删除区块链节点
func (cm *ConfigManager) DeleteBlockchainNode(c *gin.Context) {
	nodeID := c.Param("id")

	query := `DELETE FROM blockchain_nodes WHERE id = $1`
	_, err := cm.dbConfig.DB.Exec(query, nodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "删除节点失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "节点删除成功",
	})
}

// GetKafkaTopics 获取Kafka主题配置
func (cm *ConfigManager) GetKafkaTopics(c *gin.Context) {
	query := `SELECT id, data_type, topic_name, description, is_active FROM kafka_topics ORDER BY data_type`
	rows, err := cm.dbConfig.DB.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "获取Kafka主题配置失败",
			"message": err.Error(),
		})
		return
	}
	defer rows.Close()

	var topics []gin.H
	for rows.Next() {
		var id int
		var dataType, topicName, description string
		var isActive bool

		err := rows.Scan(&id, &dataType, &topicName, &description, &isActive)
		if err != nil {
			continue
		}

		topics = append(topics, gin.H{
			"id":          id,
			"data_type":   dataType,
			"topic_name":  topicName,
			"description": description,
			"is_active":   isActive,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"topics": topics,
	})
}

// UpdateKafkaTopic 更新Kafka主题配置
func (cm *ConfigManager) UpdateKafkaTopic(c *gin.Context) {
	topicID := c.Param("id")

	var req struct {
		DataType    string `json:"data_type"`
		TopicName   string `json:"topic_name"`
		Description string `json:"description"`
		IsActive    bool   `json:"is_active"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "请求参数错误",
			"message": err.Error(),
		})
		return
	}

	query := `UPDATE kafka_topics SET data_type = $1, topic_name = $2, description = $3, is_active = $4 WHERE id = $5`
	_, err := cm.dbConfig.DB.Exec(query, req.DataType, req.TopicName, req.Description, req.IsActive, topicID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "更新Kafka主题失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Kafka主题更新成功",
	})
}
