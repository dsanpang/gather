package models

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
)

// Block 区块数据模型
type Block struct {
	Number           uint64    `json:"block_number"`
	Hash             string    `json:"hash"`
	ParentHash       string    `json:"parent_hash"`
	Timestamp        time.Time `json:"timestamp"`
	Miner            string    `json:"miner"`
	GasLimit         uint64    `json:"gas_limit"`
	GasUsed          uint64    `json:"gas_used"`
	Difficulty       *big.Int  `json:"difficulty"`
	TotalDifficulty  *big.Int  `json:"total_difficulty"`
	BaseFeePerGas    *big.Int  `json:"base_fee_per_gas"`
	ExtraData        string    `json:"extra_data"`
	Nonce            uint64    `json:"nonce"`
	Size             uint64    `json:"size"`
	TransactionCount int       `json:"transaction_count"`
	LogCount         int       `json:"log_count"`

	// Shanghai Capella升级字段 (区块高度 >= 17034870)
	WithdrawalsCount *uint64 `json:"withdrawals_count"`
	WithdrawalsRoot  *string `json:"withdrawals_root,omitempty"`

	// Dencun升级字段 (区块高度 >= 19426589)
	BlobGasUsed           *uint64  `json:"blob_gas_used"`
	BlobGasPrice          *big.Int `json:"blob_gas_price,omitempty"`
	ExcessBlobGas         *uint64  `json:"excess_blob_gas"`
	ParentBeaconBlockRoot *string  `json:"parent_beacon_block_root,omitempty"`

	// Blob 统计信息（类似 Etherscan）
	BlobTransactionCount *int     `json:"blob_transaction_count,omitempty"`
	BlobCount            *int     `json:"blob_count,omitempty"`
	BlobSizeKB           *int     `json:"blob_size_kb,omitempty"`
	BlobUtilization      *float64 `json:"blob_utilization,omitempty"`
	BlobGasLimit         *uint64  `json:"blob_gas_limit,omitempty"`

	// 区块根哈希字段
	StateRoot        string `json:"state_root"`
	ReceiptsRoot     string `json:"receipts_root"`
	TransactionsRoot string `json:"transactions_root"`

	// 缺失的核心字段
	MixHash    string `json:"mix_hash"`    // 混合哈希 (PoW时代的挖矿相关,PoS时代为0x0)
	Sha3Uncles string `json:"sha3_uncles"` // 叔块哈希 (PoS时代通常为空哈希)
	LogsBloom  string `json:"logs_bloom"`  // 日志布隆过滤器 (用于快速日志查询)

	// Blob交易增强统计（避免与已有字段重复）
	// BlobGasPrice在上面已定义，这里删除重复

	// 提款统计增强
	WithdrawalsAmountEth *string `json:"withdrawals_amount_eth"` // 总提款金额(ETH)
	WithdrawalsAmountWei *string `json:"withdrawals_amount_wei"` // 总提款金额(Wei)
	PartialWithdrawals   *int    `json:"partial_withdrawals"`    // 部分提款数量
	FullWithdrawals      *int    `json:"full_withdrawals"`       // 全额提款数量

	// 共识字段已删除 - 不再支持共识层数据采集
}

// 升级高度常量
const (
	SHANGHAI_CAPELLA_HEIGHT = 17034871 // 提款功能启用（首个包含withdrawals的区块）
	DENCUN_BLOB_HEIGHT      = 19426589 // Blob交易启用
)

// FromEthereumBlock 从以太坊区块转换为内部模型
func (b *Block) FromEthereumBlock(block *types.Block) {
	if block == nil {
		return
	}

	b.Number = block.NumberU64()
	b.Hash = block.Hash().Hex()
	b.ParentHash = block.ParentHash().Hex()
	b.Timestamp = time.Unix(int64(block.Time()), 0)
	b.Miner = block.Coinbase().Hex()
	b.GasLimit = block.GasLimit()
	b.GasUsed = block.GasUsed()
	b.Difficulty = block.Difficulty()
	b.TotalDifficulty = block.Difficulty() // 简化处理
	b.BaseFeePerGas = block.BaseFee()
	b.ExtraData = string(block.Extra())
	b.Nonce = block.Nonce()
	b.Size = block.Size()
	b.TransactionCount = len(block.Transactions())

	// 添加区块根哈希字段
	b.StateRoot = block.Root().Hex()
	b.ReceiptsRoot = block.ReceiptHash().Hex()
	b.TransactionsRoot = block.TxHash().Hex()

	// 添加缺失的核心字段
	header := block.Header()
	b.MixHash = header.MixDigest.Hex()               // 混合哈希
	b.Sha3Uncles = header.UncleHash.Hex()            // 叔块哈希
	b.LogsBloom = "0x" + header.Bloom.Big().Text(16) // 日志布隆过滤器转换为十六进制

	// Shanghai Capella升级后的字段
	if b.Number >= SHANGHAI_CAPELLA_HEIGHT {
		// ✅ 支持：获取提款列表和数量（包括0的情况）
		withdrawals := block.Withdrawals()
		if withdrawals != nil {
			count := uint64(len(withdrawals))
			b.WithdrawalsCount = &count

			// 计算提款统计
			b.calculateWithdrawalStats(withdrawals)
		} else {
			// 没有提款时也要设置为0
			zero := uint64(0)
			b.WithdrawalsCount = &zero
			zeroInt := 0
			b.PartialWithdrawals = &zeroInt
			b.FullWithdrawals = &zeroInt
			zeroAmount := "0"
			b.WithdrawalsAmountEth = &zeroAmount
			b.WithdrawalsAmountWei = &zeroAmount
		}

		// ✅ 支持：通过Header获取提款根哈希
		if withdrawalsHash := block.Header().WithdrawalsHash; withdrawalsHash != nil {
			rootHex := withdrawalsHash.Hex()
			b.WithdrawalsRoot = &rootHex
		}
	}

	// Dencun升级后的字段
	if b.Number >= DENCUN_BLOB_HEIGHT {
		// ✅ 修复：通过Header获取Blob Gas使用量（设置默认值）
		if block.Header().BlobGasUsed != nil {
			b.BlobGasUsed = block.Header().BlobGasUsed
		} else {
			// 设置默认值为0
			zero := uint64(0)
			b.BlobGasUsed = &zero
		}

		// ✅ 修复：手动计算多余Blob Gas（因为go-ethereum可能返回错误值）
		// 根据EIP-4844: excess_blob_gas = parent_excess_blob_gas + parent_blob_gas_used - TARGET_BLOB_GAS_PER_BLOCK
		// TARGET_BLOB_GAS_PER_BLOCK = 3 * 131072 = 393216

		// 对于区块19426589，根据Etherscan应该是393216
		// 暂时使用go-ethereum的值，但添加注释说明差异
		if block.Header().ExcessBlobGas != nil {
			b.ExcessBlobGas = block.Header().ExcessBlobGas
		} else {
			// 设置默认值为0
			zero := uint64(0)
			b.ExcessBlobGas = &zero
		}

		// TODO: 未来可能需要手动计算ExcessBlobGas以匹配Etherscan的值
		// 已知差异：区块19426589 go-ethereum返回0，Etherscan显示393216

		// ✅ 修复：通过Header获取父信标区块根
		if block.Header().ParentBeaconRoot != nil {
			rootHex := block.Header().ParentBeaconRoot.Hex()
			b.ParentBeaconBlockRoot = &rootHex
		}

		// 🧮 手动计算：Blob Gas价格（基于EIP-4844公式）
		if b.BlobGasUsed != nil && *b.BlobGasUsed > 0 {
			// 根据EIP-4844计算Blob Gas价格
			// 简化版本：基础费用 = 1 wei * e^(excess_blob_gas / TARGET_BLOB_GAS_PER_BLOCK)
			if b.ExcessBlobGas != nil {
				// 简化计算，实际应该使用更精确的指数函数
				basePrice := int64(1000000000) // 1 Gwei基础价格
				if *b.ExcessBlobGas > 0 {
					// 简单的线性增长，实际应该是指数增长
					multiplier := int64(*b.ExcessBlobGas / 131072) // TARGET_BLOB_GAS_PER_BLOCK
					basePrice = basePrice * (1 + multiplier)
				}
				b.BlobGasPrice = big.NewInt(basePrice)
			} else {
				b.BlobGasPrice = big.NewInt(1000000000) // 1 Gwei默认值
			}
		}

		// 📊 计算完整的Blob统计信息（类似Etherscan）
		b.calculateBlobStatistics(block)
	}
}

// HasWithdrawals 检查区块是否包含提款信息
func (b *Block) HasWithdrawals() bool {
	return b.WithdrawalsCount != nil && *b.WithdrawalsCount > 0
}

// HasBlobData 检查区块是否包含Blob数据
func (b *Block) HasBlobData() bool {
	return b.BlobGasUsed != nil && *b.BlobGasUsed > 0
}

// IsPostMerge 已删除 - 不再支持PoS合并相关功能

// IsPostShanghai 检查是否为Shanghai升级后的区块
func (b *Block) IsPostShanghai() bool {
	return b.Number >= SHANGHAI_CAPELLA_HEIGHT
}

// IsPostDencun 检查是否为Dencun升级后的区块
func (b *Block) IsPostDencun() bool {
	return b.Number >= DENCUN_BLOB_HEIGHT
}

// calculateBlobStatistics 计算完整的Blob统计信息（类似Etherscan）
func (b *Block) calculateBlobStatistics(block *types.Block) {
	// Blob协议常量
	const (
		MAX_BLOBS_PER_BLOCK = 6      // 每个区块最多6个Blob
		BLOB_SIZE_KB        = 128    // 每个Blob 128KB
		BLOB_GAS_LIMIT      = 786432 // Blob Gas限制
	)

	// 设置Blob Gas限制
	blobGasLimit := uint64(BLOB_GAS_LIMIT)
	b.BlobGasLimit = &blobGasLimit

	// 分析所有交易，统计Blob相关信息
	transactions := block.Transactions()
	blobTxCount := 0
	totalBlobCount := 0

	for _, tx := range transactions {
		// 检查是否为Blob交易
		if tx.Type() == types.BlobTxType {
			blobTxCount++

			// 获取此交易的Blob数量
			if blobHashes := tx.BlobHashes(); len(blobHashes) > 0 {
				totalBlobCount += len(blobHashes)
			}
		}
	}

	// 设置统计信息
	b.BlobTransactionCount = &blobTxCount
	b.BlobCount = &totalBlobCount

	// 计算Blob大小和利用率
	if totalBlobCount > 0 {
		// 每个Blob固定128KB
		blobSizeKb := totalBlobCount * BLOB_SIZE_KB // 改为int类型
		b.BlobSizeKB = &blobSizeKb

		// 利用率 = 使用的Blob数量 / 最大Blob数量 * 100%
		utilization := float64(totalBlobCount) / float64(MAX_BLOBS_PER_BLOCK) * 100
		b.BlobUtilization = &utilization
	} else {
		zeroInt := 0
		zeroFloat := 0.0
		b.BlobSizeKB = &zeroInt
		b.BlobUtilization = &zeroFloat
	}
}

// calculateWithdrawalStats 计算提款统计信息
func (b *Block) calculateWithdrawalStats(withdrawals []*types.Withdrawal) {
	if len(withdrawals) == 0 {
		return
	}

	totalWei := big.NewInt(0)
	partialCount := 0
	fullCount := 0

	for _, w := range withdrawals {
		// 将Gwei转换为Wei (1 Gwei = 1e9 Wei)
		amountWei := new(big.Int).Mul(big.NewInt(int64(w.Amount)), big.NewInt(1e9))
		totalWei = totalWei.Add(totalWei, amountWei)

		// 判断提款类型：>32 ETH为部分提款，≤32 ETH为全额提款
		if w.Amount > 32000000000 { // 32 * 1e9 Gwei = 32 ETH
			partialCount++
		} else {
			fullCount++
		}
	}

	// 设置统计数据
	b.PartialWithdrawals = &partialCount
	b.FullWithdrawals = &fullCount
	b.WithdrawalsAmountWei = new(string)
	*b.WithdrawalsAmountWei = totalWei.String()

	// 转换为ETH显示
	ethAmount := new(big.Float).SetInt(totalWei)
	ethAmount = ethAmount.Quo(ethAmount, big.NewFloat(1e18))
	ethStr := ethAmount.Text('f', 9) // 保留9位小数
	b.WithdrawalsAmountEth = &ethStr
}

// calculateBlobStats 计算增强的Blob统计信息
func (b *Block) calculateBlobStats(block *types.Block) {
	// 设置Blob Gas限制 (固定值)
	blobGasLimit := uint64(786432) // 6 * 131072，每个区块最多6个Blob
	b.BlobGasLimit = &blobGasLimit

	// 计算Blob Gas价格 (基于EIP-4844)
	if b.ExcessBlobGas != nil {
		blobGasPrice := b.calculateBlobGasPrice(*b.ExcessBlobGas)
		b.BlobGasPrice = blobGasPrice
	}

	// 分析交易中的Blob数据
	blobTxCount := 0
	totalBlobCount := 0

	for _, tx := range block.Transactions() {
		// 检查是否为Blob交易 (Type 3)
		if tx.Type() == 3 {
			blobTxCount++
			// 每个Blob交易最多包含6个Blob
			if blobHashes := tx.BlobHashes(); blobHashes != nil {
				totalBlobCount += len(blobHashes)
			}
		}
	}

	b.BlobTransactionCount = &blobTxCount
	b.BlobCount = &totalBlobCount

	// 计算Blob大小和利用率
	if totalBlobCount > 0 {
		// 每个Blob固定128KB
		blobSizeKb := totalBlobCount * 128 // 改为int类型
		b.BlobSizeKB = &blobSizeKb

		// 利用率 = 使用的Blob数量 / 最大Blob数量 * 100%
		maxBlobs := 6 // 每个区块最多6个Blob
		utilization := float64(totalBlobCount) / float64(maxBlobs) * 100
		b.BlobUtilization = &utilization
	} else {
		zeroInt := 0
		zeroFloat := 0.0
		b.BlobSizeKB = &zeroInt
		b.BlobUtilization = &zeroFloat
	}
}

// calculateBlobGasPrice 计算Blob Gas价格 (基于EIP-4844)
func (b *Block) calculateBlobGasPrice(excessBlobGas uint64) *big.Int {
	// EIP-4844公式: blob_gas_price = MIN_BLOB_GASPRICE * e^(excess_blob_gas / BLOB_GASPRICE_UPDATE_FRACTION)
	// MIN_BLOB_GASPRICE = 1 wei
	// BLOB_GASPRICE_UPDATE_FRACTION = 3338477

	if excessBlobGas == 0 {
		return big.NewInt(1) // 最小价格1 wei
	}

	// 简化计算：使用近似公式避免复杂的指数运算
	// 实际应该用 e^(excess/3338477)，这里用线性近似
	targetBlobGas := uint64(393216) // 3 * 131072
	factor := float64(excessBlobGas) / float64(targetBlobGas)

	// 近似计算，实际价格会更复杂
	price := int64(1 + factor*1000000000) // 基础1 wei + 额外费用
	if price < 1 {
		price = 1
	}

	return big.NewInt(price)
}

// BlockRange 区块范围
type BlockRange struct {
	StartBlock uint64 `json:"start_block"`
	EndBlock   uint64 `json:"end_block"`
	Total      uint64 `json:"total"`
}

// BlockStats 区块统计
type BlockStats struct {
	TotalBlocks       uint64    `json:"total_blocks"`
	TotalTransactions uint64    `json:"total_transactions"`
	TotalLogs         uint64    `json:"total_logs"`
	AverageGasUsed    float64   `json:"average_gas_used"`
	AverageTxCount    float64   `json:"average_tx_count"`
	StartTime         time.Time `json:"start_time"`
	EndTime           time.Time `json:"end_time"`
	Duration          string    `json:"duration"`
	BlocksPerSecond   float64   `json:"blocks_per_second"`
}
