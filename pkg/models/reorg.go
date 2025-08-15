package models

import (
	"time"
)

// ReorgNotification 链重组通知
type ReorgNotification struct {
	Type                string    `json:"type"`                  // 固定为 "reorg"
	DetectedBlockNumber uint64    `json:"detected_block_number"` // 检测到重组的区块号
	RollbackToBlock     uint64    `json:"rollback_to_block"`     // 需要回滚到的区块号
	OldBlockHash        string    `json:"old_block_hash"`        // 原来的区块哈希
	NewBlockHash        string    `json:"new_block_hash"`        // 新的区块哈希
	DetectionTime       time.Time `json:"detection_time"`        // 检测时间
	AffectedBlocks      uint64    `json:"affected_blocks"`       // 受影响的区块数量
	Message             string    `json:"message"`               // 详细信息
	Severity            string    `json:"severity"`              // 严重程度: "minor", "major", "critical"
}

// ToKafkaMessage 转换为Kafka消息格式
func (r *ReorgNotification) ToKafkaMessage() map[string]interface{} {
	return map[string]interface{}{
		"type":                  r.Type,
		"detected_block_number": r.DetectedBlockNumber,
		"rollback_to_block":     r.RollbackToBlock,
		"old_block_hash":        r.OldBlockHash,
		"new_block_hash":        r.NewBlockHash,
		"detection_time":        r.DetectionTime.Unix(),
		"affected_blocks":       r.AffectedBlocks,
		"message":               r.Message,
		"severity":              r.Severity,
	}
}

// DetermineSeverity 根据影响范围确定严重程度
func (r *ReorgNotification) DetermineSeverity() {
	switch {
	case r.AffectedBlocks == 1:
		r.Severity = "minor"
	case r.AffectedBlocks <= 5:
		r.Severity = "major"
	default:
		r.Severity = "critical"
	}
}
