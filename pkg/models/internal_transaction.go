package models

import (
	"math/big"
	"time"
)

// InternalTransaction 内部交易模型
type InternalTransaction struct {
	TransactionHash string    `json:"transaction_hash"` // 外部交易哈希
	BlockNumber     uint64    `json:"block_number"`     // 区块号
	BlockHash       string    `json:"block_hash"`       // 区块哈希
	Type            string    `json:"type"`             // 类型: "call", "create", "suicide", "reward"
	From            string    `json:"from"`             // 发送方地址
	To              string    `json:"to"`               // 接收方地址
	Value           *big.Int  `json:"value"`            // 转移的以太币
	Gas             uint64    `json:"gas"`              // Gas限制
	GasUsed         uint64    `json:"gas_used"`         // 实际Gas使用量
	Input           string    `json:"input"`            // 输入数据
	Output          string    `json:"output"`           // 输出数据
	Error           string    `json:"error"`            // 错误信息
	TraceAddress    []int     `json:"trace_address"`    // 追踪地址
	CallType        string    `json:"call_type"`        // 调用类型: "call", "delegatecall", "staticcall"
	Timestamp       time.Time `json:"timestamp"`        // 时间戳
}

// TraceResult 交易追踪结果
type TraceResult struct {
	TransactionHash string                 `json:"transaction_hash"`
	BlockNumber     uint64                 `json:"block_number"`
	BlockHash       string                 `json:"block_hash"`
	InternalTxs     []*InternalTransaction `json:"internal_transactions"`
	Status          string                 `json:"status"` // "success", "failed"
	GasUsed         uint64                 `json:"gas_used"`
	Timestamp       time.Time              `json:"timestamp"`
}
