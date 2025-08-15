package models

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// Transaction 交易数据模型
type Transaction struct {
	Hash              string    `json:"hash"`
	BlockNumber       uint64    `json:"block_number"`
	BlockHash         string    `json:"block_hash"`
	From              string    `json:"from"`
	To                string    `json:"to"`
	Value             *big.Int  `json:"value"`
	Gas               uint64    `json:"gas"`
	GasPrice          *big.Int  `json:"gas_price"`
	GasUsed           uint64    `json:"gas_used"`
	Nonce             uint64    `json:"nonce"`
	Input             string    `json:"input"`
	Status            uint64    `json:"status"`
	ContractAddress   string    `json:"contract_address"`
	CumulativeGasUsed uint64    `json:"cumulative_gas_used"`
	EffectiveGasPrice *big.Int  `json:"effective_gas_price"`
	Type              uint64    `json:"type"`
	Timestamp         time.Time `json:"timestamp"`
	LogCount          int       `json:"log_count"`

	// 输入数据解码字段
	MethodSignature string                 `json:"method_signature,omitempty"` // 前4字节方法签名
	MethodName      string                 `json:"method_name,omitempty"`      // 解码后的方法名
	DecodedInput    map[string]interface{} `json:"decoded_input,omitempty"`    // 解码后的输入参数
	IsContractCall  bool                   `json:"is_contract_call"`           // 是否为合约调用
}

// FromEthereumTransaction 从以太坊交易转换为内部模型
func (t *Transaction) FromEthereumTransaction(tx *types.Transaction, receipt *types.Receipt, blockTime uint64) {
	if tx == nil {
		return
	}

	// 检查receipt是否为nil
	if receipt == nil {
		// 如果receipt为nil，设置默认值
		t.Hash = tx.Hash().Hex()
		t.BlockHash = ""
		t.From = ""
		t.To = ""
		if tx.To() != nil {
			t.To = tx.To().Hex()
		}
		t.Value = tx.Value()
		t.Gas = tx.Gas()
		t.GasPrice = tx.GasPrice()
		t.GasUsed = 0
		t.Nonce = tx.Nonce()
		t.Input = common.Bytes2Hex(tx.Data())
		t.Status = 0
		t.ContractAddress = ""
		t.CumulativeGasUsed = 0
		t.EffectiveGasPrice = big.NewInt(0)
		t.Type = uint64(tx.Type())
		t.Timestamp = time.Unix(int64(blockTime), 0)
		t.LogCount = 0
		t.BlockNumber = 0
		return
	}

	t.Hash = tx.Hash().Hex()
	t.BlockHash = receipt.BlockHash.Hex()

	// 尝试多种方法获取发送地址，支持所有交易类型
	var from common.Address
	var err error

	// 首先尝试根据交易类型获取发送地址
	switch tx.Type() {
	case types.LegacyTxType:
		from, err = types.Sender(types.NewEIP155Signer(tx.ChainId()), tx)
	case types.AccessListTxType:
		from, err = types.Sender(types.NewEIP2930Signer(tx.ChainId()), tx)
	case types.DynamicFeeTxType:
		from, err = types.Sender(types.NewLondonSigner(tx.ChainId()), tx)
	case types.BlobTxType:
		// Blob 交易 (EIP-4844) 使用 Cancun 签名者
		from, err = types.Sender(types.NewCancunSigner(tx.ChainId()), tx)
	case types.SetCodeTxType:
		// SetCode 交易 (EIP-7702) 使用 Prague 签名者
		from, err = types.Sender(types.NewPragueSigner(tx.ChainId()), tx)
	default:
		// 对于未知交易类型，尝试所有可能的签名者
		// 按优先级尝试：Prague -> Cancun -> London -> EIP2930 -> EIP155
		signers := []types.Signer{
			types.NewPragueSigner(tx.ChainId()),  // 支持所有最新交易类型
			types.NewCancunSigner(tx.ChainId()),  // 支持 Blob 交易
			types.NewLondonSigner(tx.ChainId()),  // 支持 EIP-1559
			types.NewEIP2930Signer(tx.ChainId()), // 支持访问列表
			types.NewEIP155Signer(tx.ChainId()),  // 支持重放保护
		}

		for _, signer := range signers {
			from, err = types.Sender(signer, tx)
			if err == nil {
				break
			}
		}

		// 如果所有签名者都失败，尝试使用无签名者（仅用于某些特殊交易）
		if err != nil {
			// 对于某些特殊交易，可能无法获取发送地址
			// 在这种情况下，我们设置一个默认值
			from = common.Address{}
			err = nil
		}
	}

	// 设置发送地址
	if err == nil && from != (common.Address{}) {
		t.From = from.Hex()
	} else {
		// 如果无法获取发送地址，设置为空字符串
		t.From = ""
	}

	if tx.To() != nil {
		t.To = tx.To().Hex()
	}
	t.Value = tx.Value()
	t.Gas = tx.Gas()
	t.GasPrice = tx.GasPrice()
	t.GasUsed = receipt.GasUsed
	t.Nonce = tx.Nonce()
	t.Input = common.Bytes2Hex(tx.Data()) // 将二进制数据转换为十六进制字符串
	t.Status = receipt.Status
	if receipt.ContractAddress != (common.Address{}) {
		t.ContractAddress = receipt.ContractAddress.Hex()
	}
	t.CumulativeGasUsed = receipt.CumulativeGasUsed
	t.EffectiveGasPrice = receipt.EffectiveGasPrice
	t.Type = uint64(tx.Type())
	t.Timestamp = time.Unix(int64(blockTime), 0)
	t.LogCount = len(receipt.Logs)
	t.BlockNumber = receipt.BlockNumber.Uint64() // 设置正确的区块号
}

// TransactionLog 交易日志模型
type TransactionLog struct {
	TransactionHash string    `json:"transaction_hash"`
	BlockNumber     uint64    `json:"block_number"`
	BlockHash       string    `json:"block_hash"`
	Address         string    `json:"address"`
	Topics          []string  `json:"topics"`
	Data            string    `json:"data"`
	LogIndex        uint      `json:"log_index"`
	Removed         bool      `json:"removed"`
	Timestamp       time.Time `json:"timestamp"`
}

// FromEthereumLog 从以太坊日志转换为内部模型
func (l *TransactionLog) FromEthereumLog(log *types.Log, txHash string, blockNumber uint64, blockTime uint64) {
	if log == nil {
		return
	}

	l.TransactionHash = txHash
	l.BlockNumber = blockNumber
	l.BlockHash = log.BlockHash.Hex()
	l.Address = log.Address.Hex()
	l.Topics = make([]string, len(log.Topics))
	for i, topic := range log.Topics {
		l.Topics[i] = topic.Hex()
	}
	l.Data = string(log.Data)
	l.LogIndex = log.Index
	l.Removed = log.Removed
	l.Timestamp = time.Unix(int64(blockTime), 0)
}

// TransactionStats 交易统计
type TransactionStats struct {
	TotalTransactions uint64   `json:"total_transactions"`
	SuccessfulTxs     uint64   `json:"successful_transactions"`
	FailedTxs         uint64   `json:"failed_transactions"`
	TotalGasUsed      uint64   `json:"total_gas_used"`
	AverageGasUsed    float64  `json:"average_gas_used"`
	TotalValue        *big.Int `json:"total_value"`
	AverageValue      *big.Int `json:"average_value"`
}
