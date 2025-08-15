package models

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
)

// Withdrawal 提款数据模型
type Withdrawal struct {
	// 基础字段
	Index          uint64   `json:"index"`           // 提款索引
	ValidatorIndex uint64   `json:"validator_index"` // 验证者索引
	Address        string   `json:"address"`         // 提款地址
	Amount         *big.Int `json:"amount"`          // 提款金额(wei)
	AmountGwei     uint64   `json:"amount_gwei"`     // 提款金额(gwei)
	AmountEth      string   `json:"amount_eth"`      // 提款金额(eth,格式化字符串)

	// 关联字段
	BlockNumber uint64    `json:"block_number"` // 所属区块号
	BlockHash   string    `json:"block_hash"`   // 所属区块哈希
	Timestamp   time.Time `json:"timestamp"`    // 时间戳

	// 统计字段
	WithdrawalType string `json:"withdrawal_type"` // 提款类型(partial/full)
}

// FromEthereumWithdrawal 从以太坊提款数据转换为内部模型
func (w *Withdrawal) FromEthereumWithdrawal(withdrawal *types.Withdrawal, blockNumber uint64, blockHash string, timestamp time.Time) {
	if withdrawal == nil {
		return
	}

	w.Index = withdrawal.Index
	w.ValidatorIndex = withdrawal.Validator
	w.Address = withdrawal.Address.Hex()

	// 转换金额：从Gwei到Wei (1 Gwei = 1e9 Wei)
	w.AmountGwei = withdrawal.Amount
	w.Amount = new(big.Int).Mul(big.NewInt(int64(withdrawal.Amount)), big.NewInt(1e9))

	// 转换为ETH显示 (1 ETH = 1e18 Wei)
	ethAmount := new(big.Float).SetInt(w.Amount)
	ethAmount = ethAmount.Quo(ethAmount, big.NewFloat(1e18))
	w.AmountEth = ethAmount.Text('f', 9) // 保留9位小数

	// 关联信息
	w.BlockNumber = blockNumber
	w.BlockHash = blockHash
	w.Timestamp = timestamp

	// 判断提款类型 (简化逻辑：>32 ETH为部分提款，=32 ETH为全额提款)
	if w.AmountGwei > 32000000000 { // 32 * 1e9 Gwei
		w.WithdrawalType = "partial"
	} else {
		w.WithdrawalType = "full"
	}
}

// ToKafkaMessage 转换为Kafka消息格式
func (w *Withdrawal) ToKafkaMessage() map[string]interface{} {
	return map[string]interface{}{
		"index":           w.Index,
		"validator_index": w.ValidatorIndex,
		"address":         w.Address,
		"amount":          w.Amount.String(),
		"amount_gwei":     w.AmountGwei,
		"amount_eth":      w.AmountEth,
		"block_number":    w.BlockNumber,
		"block_hash":      w.BlockHash,
		"timestamp":       w.Timestamp.Unix(),
		"withdrawal_type": w.WithdrawalType,
	}
}
