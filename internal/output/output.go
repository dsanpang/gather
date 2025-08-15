package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gather/internal/config"
	"gather/pkg/models"

	"github.com/sirupsen/logrus"
)

// Output 输出接口
type Output interface {
	WriteBlock(block *models.Block) error
	WriteTransaction(tx *models.Transaction) error
	WriteLog(log *models.TransactionLog) error
	WriteInternalTransaction(itx *models.InternalTransaction) error
	WriteStateChange(sc *models.StateChange) error
	WriteContractCreation(cc *models.ContractCreation) error
	WriteWithdrawal(w *models.Withdrawal) error                   // 新增：提款数据输出
	WriteReorgNotification(reorg *models.ReorgNotification) error // 新增：重组通知输出
	Close() error
}

// FileOutput 文件输出
type FileOutput struct {
	outputDir            string
	format               string
	compress             bool
	blockFile            *os.File
	txFile               *os.File
	logFile              *os.File
	internalTxFile       *os.File
	stateChangeFile      *os.File
	contractCreationFile *os.File
	withdrawalFile       *os.File
	reorgFile            *os.File
}

// NewOutput 创建输出器
func NewOutput(outputPath, format string, compress bool) (Output, error) {
	return NewOutputWithConfig(outputPath, format, compress, nil)
}

// NewOutputWithConfig 创建输出器（带配置）
func NewOutputWithConfig(outputPath, format string, compress bool, kafkaConfig *config.KafkaConfig) (Output, error) {
	// 检查是否是Kafka输出
	if format == "kafka" || format == "kafka_async" {
		// 从环境变量或配置中获取Kafka配置
		brokers := []string{"localhost:9092"} // 默认Kafka地址
		if kafkaBrokers := os.Getenv("KAFKA_BROKERS"); kafkaBrokers != "" {
			brokers = strings.Split(kafkaBrokers, ",")
		}

		// 默认topic映射
		topics := map[string]string{
			"blocks":                "blockchain_blocks",
			"transactions":          "blockchain_transactions",
			"logs":                  "blockchain_logs",
			"internal_transactions": "blockchain_internal_transactions",
			"state_changes":         "blockchain_state_changes",
			"contract_creations":    "blockchain_contract_creations",
			"reorg_notifications":   "blockchain_reorg_notifications",
		}

		// 如果提供了Kafka配置，使用配置中的设置
		if kafkaConfig != nil {
			if len(kafkaConfig.Brokers) > 0 {
				brokers = kafkaConfig.Brokers
			}
			if len(kafkaConfig.Topics) > 0 {
				topics = kafkaConfig.Topics
			}
		}

		// 创建logger
		logger := logrus.New()
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})

		// 选择同步或异步Kafka输出器
		if format == "kafka_async" {
			return NewAsyncKafkaOutput(brokers, topics, logger)
		} else {
			return NewKafkaOutput(brokers, topics, logger)
		}
	}

	// 检查是否是异步文件输出
	if format == "json_async" {
		logger := logrus.New()
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
		return NewAsyncFileOutput(outputPath, "json", compress, logger)
	}

	// 确保输出目录存在
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}

	output := &FileOutput{
		outputDir: outputPath,
		format:    format,
		compress:  compress,
	}

	// 创建输出文件
	timestamp := time.Now().Format("20060102_150405")

	blockFile, err := os.Create(filepath.Join(outputPath, fmt.Sprintf("blocks_%s.%s", timestamp, format)))
	if err != nil {
		return nil, fmt.Errorf("创建区块文件失败: %w", err)
	}
	output.blockFile = blockFile

	txFile, err := os.Create(filepath.Join(outputPath, fmt.Sprintf("transactions_%s.%s", timestamp, format)))
	if err != nil {
		return nil, fmt.Errorf("创建交易文件失败: %w", err)
	}
	output.txFile = txFile

	logFile, err := os.Create(filepath.Join(outputPath, fmt.Sprintf("logs_%s.%s", timestamp, format)))
	if err != nil {
		return nil, fmt.Errorf("创建日志文件失败: %w", err)
	}
	output.logFile = logFile

	internalTxFile, err := os.Create(filepath.Join(outputPath, fmt.Sprintf("internal_transactions_%s.%s", timestamp, format)))
	if err != nil {
		return nil, fmt.Errorf("创建内部交易文件失败: %w", err)
	}
	output.internalTxFile = internalTxFile

	stateChangeFile, err := os.Create(filepath.Join(outputPath, fmt.Sprintf("state_changes_%s.%s", timestamp, format)))
	if err != nil {
		return nil, fmt.Errorf("创建状态变更文件失败: %w", err)
	}
	output.stateChangeFile = stateChangeFile

	contractCreationFile, err := os.Create(filepath.Join(outputPath, fmt.Sprintf("contract_creations_%s.%s", timestamp, format)))
	if err != nil {
		return nil, fmt.Errorf("创建合约创建文件失败: %w", err)
	}
	output.contractCreationFile = contractCreationFile

	withdrawalFile, err := os.Create(filepath.Join(outputPath, fmt.Sprintf("withdrawals_%s.%s", timestamp, format)))
	if err != nil {
		return nil, fmt.Errorf("创建提款文件失败: %w", err)
	}
	output.withdrawalFile = withdrawalFile

	reorgFile, err := os.Create(filepath.Join(outputPath, fmt.Sprintf("reorg_notifications_%s.%s", timestamp, format)))
	if err != nil {
		return nil, fmt.Errorf("创建重组通知文件失败: %w", err)
	}
	output.reorgFile = reorgFile

	return output, nil
}

// WriteBlock 写入区块数据
func (o *FileOutput) WriteBlock(block *models.Block) error {
	if block == nil {
		return nil
	}

	data, err := json.Marshal(block)
	if err != nil {
		return fmt.Errorf("序列化区块数据失败: %w", err)
	}

	// 添加换行符
	data = append(data, '\n')

	if _, err := o.blockFile.Write(data); err != nil {
		return fmt.Errorf("写入区块文件失败: %w", err)
	}

	// 强制刷新到磁盘
	if err := o.blockFile.Sync(); err != nil {
		return fmt.Errorf("刷新区块文件失败: %w", err)
	}

	return nil
}

// WriteTransaction 写入交易数据
func (o *FileOutput) WriteTransaction(tx *models.Transaction) error {
	if tx == nil {
		return nil
	}

	data, err := json.Marshal(tx)
	if err != nil {
		return fmt.Errorf("序列化交易数据失败: %w", err)
	}

	// 添加换行符
	data = append(data, '\n')

	if _, err := o.txFile.Write(data); err != nil {
		return fmt.Errorf("写入交易文件失败: %w", err)
	}

	// 强制刷新到磁盘
	if err := o.txFile.Sync(); err != nil {
		return fmt.Errorf("刷新交易文件失败: %w", err)
	}

	return nil
}

// WriteLog 写入日志数据
func (o *FileOutput) WriteLog(log *models.TransactionLog) error {
	if log == nil {
		return nil
	}

	data, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("序列化日志数据失败: %w", err)
	}

	// 添加换行符
	data = append(data, '\n')

	if _, err := o.logFile.Write(data); err != nil {
		return fmt.Errorf("写入日志文件失败: %w", err)
	}

	// 强制刷新到磁盘
	if err := o.logFile.Sync(); err != nil {
		return fmt.Errorf("刷新日志文件失败: %w", err)
	}

	return nil
}

// WriteInternalTransaction 写入内部交易数据
func (o *FileOutput) WriteInternalTransaction(itx *models.InternalTransaction) error {
	if itx == nil {
		return nil
	}

	data, err := json.Marshal(itx)
	if err != nil {
		return fmt.Errorf("序列化内部交易数据失败: %w", err)
	}

	// 添加换行符
	data = append(data, '\n')

	if _, err := o.internalTxFile.Write(data); err != nil {
		return fmt.Errorf("写入内部交易文件失败: %w", err)
	}

	// 强制刷新到磁盘
	if err := o.internalTxFile.Sync(); err != nil {
		return fmt.Errorf("刷新内部交易文件失败: %w", err)
	}

	return nil
}

// WriteStateChange 写入状态变更数据
func (o *FileOutput) WriteStateChange(sc *models.StateChange) error {
	if sc == nil {
		return nil
	}

	data, err := json.Marshal(sc)
	if err != nil {
		return fmt.Errorf("序列化状态变更数据失败: %w", err)
	}

	// 添加换行符
	data = append(data, '\n')

	if _, err := o.stateChangeFile.Write(data); err != nil {
		return fmt.Errorf("写入状态变更文件失败: %w", err)
	}

	// 强制刷新到磁盘
	if err := o.stateChangeFile.Sync(); err != nil {
		return fmt.Errorf("刷新状态变更文件失败: %w", err)
	}

	return nil
}

// WriteContractCreation 写入合约创建数据
func (o *FileOutput) WriteContractCreation(cc *models.ContractCreation) error {
	if cc == nil {
		return nil
	}

	data, err := json.Marshal(cc)
	if err != nil {
		return fmt.Errorf("序列化合约创建数据失败: %w", err)
	}

	// 添加换行符
	data = append(data, '\n')

	if _, err := o.contractCreationFile.Write(data); err != nil {
		return fmt.Errorf("写入合约创建文件失败: %w", err)
	}

	// 强制刷新到磁盘
	if err := o.contractCreationFile.Sync(); err != nil {
		return fmt.Errorf("刷新合约创建文件失败: %w", err)
	}

	return nil
}

// Close 关闭文件
func (o *FileOutput) Close() error {
	var errors []error

	if o.blockFile != nil {
		if err := o.blockFile.Close(); err != nil {
			errors = append(errors, fmt.Errorf("关闭区块文件失败: %w", err))
		}
	}

	if o.txFile != nil {
		if err := o.txFile.Close(); err != nil {
			errors = append(errors, fmt.Errorf("关闭交易文件失败: %w", err))
		}
	}

	if o.logFile != nil {
		if err := o.logFile.Close(); err != nil {
			errors = append(errors, fmt.Errorf("关闭日志文件失败: %w", err))
		}
	}

	if o.internalTxFile != nil {
		if err := o.internalTxFile.Close(); err != nil {
			errors = append(errors, fmt.Errorf("关闭内部交易文件失败: %w", err))
		}
	}

	if o.stateChangeFile != nil {
		if err := o.stateChangeFile.Close(); err != nil {
			errors = append(errors, fmt.Errorf("关闭状态变更文件失败: %w", err))
		}
	}

	if o.contractCreationFile != nil {
		if err := o.contractCreationFile.Close(); err != nil {
			errors = append(errors, fmt.Errorf("关闭合约创建文件失败: %w", err))
		}
	}

	if o.withdrawalFile != nil {
		if err := o.withdrawalFile.Close(); err != nil {
			errors = append(errors, fmt.Errorf("关闭提款文件失败: %w", err))
		}
	}

	if o.reorgFile != nil {
		if err := o.reorgFile.Close(); err != nil {
			errors = append(errors, fmt.Errorf("关闭重组通知文件失败: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("关闭输出文件时发生错误: %v", errors)
	}

	return nil
}

// WriteWithdrawal 写入提款数据
func (o *FileOutput) WriteWithdrawal(w *models.Withdrawal) error {
	if w == nil {
		return nil
	}

	data, err := json.Marshal(w)
	if err != nil {
		return fmt.Errorf("序列化提款数据失败: %w", err)
	}

	// 添加换行符
	data = append(data, '\n')

	if _, err := o.withdrawalFile.Write(data); err != nil {
		return fmt.Errorf("写入提款文件失败: %w", err)
	}

	// 强制刷新到磁盘
	if err := o.withdrawalFile.Sync(); err != nil {
		return fmt.Errorf("刷新提款文件失败: %w", err)
	}

	return nil
}

// WriteReorgNotification 写入重组通知
func (o *FileOutput) WriteReorgNotification(reorg *models.ReorgNotification) error {
	if reorg == nil {
		return nil
	}

	data, err := json.Marshal(reorg)
	if err != nil {
		return fmt.Errorf("序列化重组通知数据失败: %w", err)
	}

	// 添加换行符
	data = append(data, '\n')

	if _, err := o.reorgFile.Write(data); err != nil {
		return fmt.Errorf("写入重组通知文件失败: %w", err)
	}

	// 强制刷新到磁盘
	if err := o.reorgFile.Sync(); err != nil {
		return fmt.Errorf("刷新重组通知文件失败: %w", err)
	}

	return nil
}
