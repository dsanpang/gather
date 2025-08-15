package models

import (
	"time"
)

// StateChange 状态变更模型
type StateChange struct {
	TransactionHash string    `json:"transaction_hash"` // 交易哈希
	BlockNumber     uint64    `json:"block_number"`     // 区块号
	BlockHash       string    `json:"block_hash"`       // 区块哈希
	Address         string    `json:"address"`          // 合约地址
	StorageKey      string    `json:"storage_key"`      // 存储键
	OldValue        string    `json:"old_value"`        // 旧值
	NewValue        string    `json:"new_value"`        // 新值
	Type            string    `json:"type"`             // 变更类型: "storage", "balance", "code", "nonce"
	Timestamp       time.Time `json:"timestamp"`        // 时间戳
}

// ContractCreation 合约创建详情
type ContractCreation struct {
	TransactionHash       string    `json:"transaction_hash"`                 // 创建交易哈希
	BlockNumber           uint64    `json:"block_number"`                     // 区块号
	BlockHash             string    `json:"block_hash"`                       // 区块哈希
	ContractAddress       string    `json:"contract_address"`                 // 合约地址
	Creator               string    `json:"creator"`                          // 创建者地址
	Code                  string    `json:"code"`                             // 合约字节码
	CodeSize              int       `json:"code_size"`                        // 字节码大小
	RuntimeCode           string    `json:"runtime_code"`                     // 运行时字节码
	RuntimeCodeSize       int       `json:"runtime_code_size"`                // 运行时字节码大小
	ConstructorArgs       string    `json:"constructor_args"`                 // 构造函数参数
	GasUsed               uint64    `json:"gas_used"`                         // Gas使用量
	ContractType          string    `json:"contract_type"`                    // 合约类型 ("eoa", "contract", "proxy", "token", "unknown")
	IsProxy               bool      `json:"is_proxy"`                         // 是否为代理合约
	ImplementationAddress string    `json:"implementation_address,omitempty"` // 实现合约地址（代理合约）
	Timestamp             time.Time `json:"timestamp"`                        // 时间戳
}
