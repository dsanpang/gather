package output

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gather/pkg/models"

	"github.com/sirupsen/logrus"
)

// AsyncFileOutput 异步文件输出器
type AsyncFileOutput struct {
	outputDir    string
	format       string
	compress     bool
	logger       *logrus.Logger
	
	// 文件句柄
	files map[string]*os.File
	
	// 异步写入通道
	blockChan       chan *models.Block
	txChan          chan *models.Transaction
	logChan         chan *models.TransactionLog
	internalTxChan  chan *models.InternalTransaction
	stateChangeChan chan *models.StateChange
	contractChan    chan *models.ContractCreation
	withdrawalChan  chan *models.Withdrawal
	reorgChan       chan *models.ReorgNotification
	
	// 控制通道
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	
	// 批量写入配置
	batchSize    int
	flushInterval time.Duration
}

// NewAsyncFileOutput 创建异步文件输出器
func NewAsyncFileOutput(outputPath, format string, compress bool, logger *logrus.Logger) (*AsyncFileOutput, error) {
	// 确保输出目录存在
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	
	output := &AsyncFileOutput{
		outputDir:       outputPath,
		format:          format,
		compress:        compress,
		logger:          logger,
		files:           make(map[string]*os.File),
		ctx:             ctx,
		cancel:          cancel,
		batchSize:       100,         // 批量大小
		flushInterval:   time.Second, // 刷新间隔
	}

	// 初始化通道 - 使用缓冲通道提高性能
	channelSize := 1000
	output.blockChan = make(chan *models.Block, channelSize)
	output.txChan = make(chan *models.Transaction, channelSize)
	output.logChan = make(chan *models.TransactionLog, channelSize)
	output.internalTxChan = make(chan *models.InternalTransaction, channelSize)
	output.stateChangeChan = make(chan *models.StateChange, channelSize)
	output.contractChan = make(chan *models.ContractCreation, channelSize)
	output.withdrawalChan = make(chan *models.Withdrawal, channelSize)
	output.reorgChan = make(chan *models.ReorgNotification, channelSize)

	// 创建文件
	if err := output.createFiles(); err != nil {
		return nil, err
	}

	// 启动异步写入处理器
	output.startWorkers()

	logger.Info("异步文件输出器已初始化")
	return output, nil
}

// createFiles 创建输出文件
func (o *AsyncFileOutput) createFiles() error {
	timestamp := time.Now().Format("20060102_150405")
	
	fileNames := map[string]string{
		"blocks":                fmt.Sprintf("blocks_%s.%s", timestamp, o.format),
		"transactions":          fmt.Sprintf("transactions_%s.%s", timestamp, o.format),
		"logs":                  fmt.Sprintf("logs_%s.%s", timestamp, o.format),
		"internal_transactions": fmt.Sprintf("internal_transactions_%s.%s", timestamp, o.format),
		"state_changes":         fmt.Sprintf("state_changes_%s.%s", timestamp, o.format),
		"contract_creations":    fmt.Sprintf("contract_creations_%s.%s", timestamp, o.format),
		"withdrawals":           fmt.Sprintf("withdrawals_%s.%s", timestamp, o.format),
		"reorg_notifications":   fmt.Sprintf("reorg_notifications_%s.%s", timestamp, o.format),
	}

	for key, fileName := range fileNames {
		file, err := os.Create(filepath.Join(o.outputDir, fileName))
		if err != nil {
			return fmt.Errorf("创建文件 %s 失败: %w", fileName, err)
		}
		o.files[key] = file
	}

	return nil
}

// startWorkers 启动异步写入工作器
func (o *AsyncFileOutput) startWorkers() {
	// 区块写入器
	o.wg.Add(1)
	go o.blockWriter()
	
	// 交易写入器
	o.wg.Add(1)
	go o.transactionWriter()
	
	// 日志写入器
	o.wg.Add(1)
	go o.logWriter()
	
	// 内部交易写入器
	o.wg.Add(1)
	go o.internalTransactionWriter()
	
	// 状态变更写入器
	o.wg.Add(1)
	go o.stateChangeWriter()
	
	// 合约创建写入器
	o.wg.Add(1)
	go o.contractCreationWriter()
	
	// 提款写入器
	o.wg.Add(1)
	go o.withdrawalWriter()
	
	// 重组通知写入器
	o.wg.Add(1)
	go o.reorgWriter()
}

// blockWriter 区块写入工作器
func (o *AsyncFileOutput) blockWriter() {
	defer o.wg.Done()
	
	file := o.files["blocks"]
	batch := make([]*models.Block, 0, o.batchSize)
	ticker := time.NewTicker(o.flushInterval)
	defer ticker.Stop()
	
	for {
		select {
		case block := <-o.blockChan:
			batch = append(batch, block)
			if len(batch) >= o.batchSize {
				o.flushBlockBatch(file, batch)
				batch = batch[:0] // 重置切片
			}
			
		case <-ticker.C:
			if len(batch) > 0 {
				o.flushBlockBatch(file, batch)
				batch = batch[:0]
			}
			
		case <-o.ctx.Done():
			// 写入剩余数据
			if len(batch) > 0 {
				o.flushBlockBatch(file, batch)
			}
			return
		}
	}
}

// flushBlockBatch 批量写入区块数据
func (o *AsyncFileOutput) flushBlockBatch(file *os.File, batch []*models.Block) {
	for _, block := range batch {
		if block == nil {
			continue
		}
		
		data, err := json.Marshal(block)
		if err != nil {
			o.logger.Errorf("序列化区块数据失败: %v", err)
			continue
		}
		
		data = append(data, '\n')
		if _, err := file.Write(data); err != nil {
			o.logger.Errorf("写入区块文件失败: %v", err)
		}
	}
	
	// 批量刷新
	if err := file.Sync(); err != nil {
		o.logger.Errorf("刷新区块文件失败: %v", err)
	}
}

// transactionWriter 交易写入工作器
func (o *AsyncFileOutput) transactionWriter() {
	defer o.wg.Done()
	
	file := o.files["transactions"]
	batch := make([]*models.Transaction, 0, o.batchSize)
	ticker := time.NewTicker(o.flushInterval)
	defer ticker.Stop()
	
	for {
		select {
		case tx := <-o.txChan:
			batch = append(batch, tx)
			if len(batch) >= o.batchSize {
				o.flushTransactionBatch(file, batch)
				batch = batch[:0]
			}
			
		case <-ticker.C:
			if len(batch) > 0 {
				o.flushTransactionBatch(file, batch)
				batch = batch[:0]
			}
			
		case <-o.ctx.Done():
			if len(batch) > 0 {
				o.flushTransactionBatch(file, batch)
			}
			return
		}
	}
}

// flushTransactionBatch 批量写入交易数据
func (o *AsyncFileOutput) flushTransactionBatch(file *os.File, batch []*models.Transaction) {
	for _, tx := range batch {
		if tx == nil {
			continue
		}
		
		data, err := json.Marshal(tx)
		if err != nil {
			o.logger.Errorf("序列化交易数据失败: %v", err)
			continue
		}
		
		data = append(data, '\n')
		if _, err := file.Write(data); err != nil {
			o.logger.Errorf("写入交易文件失败: %v", err)
		}
	}
	
	if err := file.Sync(); err != nil {
		o.logger.Errorf("刷新交易文件失败: %v", err)
	}
}

// 类似地实现其他写入器...
func (o *AsyncFileOutput) logWriter() {
	defer o.wg.Done()
	o.genericWriter(o.logChan, o.files["logs"], "日志")
}

func (o *AsyncFileOutput) internalTransactionWriter() {
	defer o.wg.Done()
	o.genericWriter(o.internalTxChan, o.files["internal_transactions"], "内部交易")
}

func (o *AsyncFileOutput) stateChangeWriter() {
	defer o.wg.Done()
	o.genericWriter(o.stateChangeChan, o.files["state_changes"], "状态变更")
}

func (o *AsyncFileOutput) contractCreationWriter() {
	defer o.wg.Done()
	o.genericWriter(o.contractChan, o.files["contract_creations"], "合约创建")
}

func (o *AsyncFileOutput) withdrawalWriter() {
	defer o.wg.Done()
	o.genericWriter(o.withdrawalChan, o.files["withdrawals"], "提款")
}

func (o *AsyncFileOutput) reorgWriter() {
	defer o.wg.Done()
	o.genericWriter(o.reorgChan, o.files["reorg_notifications"], "重组通知")
}

// genericWriter 通用写入器
func (o *AsyncFileOutput) genericWriter(ch interface{}, file *os.File, dataType string) {
	ticker := time.NewTicker(o.flushInterval)
	defer ticker.Stop()
	
	var buffer []byte
	itemCount := 0
	
	for {
		select {
		case <-ticker.C:
			if len(buffer) > 0 {
				o.flushBuffer(file, buffer, dataType)
				buffer = buffer[:0]
				itemCount = 0
			}
			
		case <-o.ctx.Done():
			if len(buffer) > 0 {
				o.flushBuffer(file, buffer, dataType)
			}
			return
			
		default:
			// 使用类型断言处理不同的通道类型
			var data []byte
			var err error
			
			switch c := ch.(type) {
			case chan *models.TransactionLog:
				select {
				case item := <-c:
					if item != nil {
						data, err = json.Marshal(item)
					}
				default:
					continue
				}
			case chan *models.InternalTransaction:
				select {
				case item := <-c:
					if item != nil {
						data, err = json.Marshal(item)
					}
				default:
					continue
				}
			case chan *models.StateChange:
				select {
				case item := <-c:
					if item != nil {
						data, err = json.Marshal(item)
					}
				default:
					continue
				}
			case chan *models.ContractCreation:
				select {
				case item := <-c:
					if item != nil {
						data, err = json.Marshal(item)
					}
				default:
					continue
				}
			case chan *models.Withdrawal:
				select {
				case item := <-c:
					if item != nil {
						data, err = json.Marshal(item)
					}
				default:
					continue
				}
			case chan *models.ReorgNotification:
				select {
				case item := <-c:
					if item != nil {
						data, err = json.Marshal(item)
					}
				default:
					continue
				}
			default:
				continue
			}
			
			if err != nil {
				o.logger.Errorf("序列化%s数据失败: %v", dataType, err)
				continue
			}
			
			if data != nil {
				data = append(data, '\n')
				buffer = append(buffer, data...)
				itemCount++
				
				if itemCount >= o.batchSize {
					o.flushBuffer(file, buffer, dataType)
					buffer = buffer[:0]
					itemCount = 0
				}
			}
		}
	}
}

// flushBuffer 刷新缓冲区
func (o *AsyncFileOutput) flushBuffer(file *os.File, buffer []byte, dataType string) {
	if _, err := file.Write(buffer); err != nil {
		o.logger.Errorf("写入%s文件失败: %v", dataType, err)
		return
	}
	
	if err := file.Sync(); err != nil {
		o.logger.Errorf("刷新%s文件失败: %v", dataType, err)
	}
}

// WriteBlock 写入区块数据
func (o *AsyncFileOutput) WriteBlock(block *models.Block) error {
	if block == nil {
		return nil
	}
	
	select {
	case o.blockChan <- block:
		return nil
	case <-o.ctx.Done():
		return fmt.Errorf("输出器已关闭")
	default:
		return fmt.Errorf("区块通道已满，丢弃数据")
	}
}

// WriteTransaction 写入交易数据
func (o *AsyncFileOutput) WriteTransaction(tx *models.Transaction) error {
	if tx == nil {
		return nil
	}
	
	select {
	case o.txChan <- tx:
		return nil
	case <-o.ctx.Done():
		return fmt.Errorf("输出器已关闭")
	default:
		return fmt.Errorf("交易通道已满，丢弃数据")
	}
}

// WriteLog 写入日志数据
func (o *AsyncFileOutput) WriteLog(log *models.TransactionLog) error {
	if log == nil {
		return nil
	}
	
	select {
	case o.logChan <- log:
		return nil
	case <-o.ctx.Done():
		return fmt.Errorf("输出器已关闭")
	default:
		return fmt.Errorf("日志通道已满，丢弃数据")
	}
}

// WriteInternalTransaction 写入内部交易数据
func (o *AsyncFileOutput) WriteInternalTransaction(itx *models.InternalTransaction) error {
	if itx == nil {
		return nil
	}
	
	select {
	case o.internalTxChan <- itx:
		return nil
	case <-o.ctx.Done():
		return fmt.Errorf("输出器已关闭")
	default:
		return fmt.Errorf("内部交易通道已满，丢弃数据")
	}
}

// WriteStateChange 写入状态变更数据
func (o *AsyncFileOutput) WriteStateChange(sc *models.StateChange) error {
	if sc == nil {
		return nil
	}
	
	select {
	case o.stateChangeChan <- sc:
		return nil
	case <-o.ctx.Done():
		return fmt.Errorf("输出器已关闭")
	default:
		return fmt.Errorf("状态变更通道已满，丢弃数据")
	}
}

// WriteContractCreation 写入合约创建数据
func (o *AsyncFileOutput) WriteContractCreation(cc *models.ContractCreation) error {
	if cc == nil {
		return nil
	}
	
	select {
	case o.contractChan <- cc:
		return nil
	case <-o.ctx.Done():
		return fmt.Errorf("输出器已关闭")
	default:
		return fmt.Errorf("合约创建通道已满，丢弃数据")
	}
}

// WriteWithdrawal 写入提款数据
func (o *AsyncFileOutput) WriteWithdrawal(w *models.Withdrawal) error {
	if w == nil {
		return nil
	}
	
	select {
	case o.withdrawalChan <- w:
		return nil
	case <-o.ctx.Done():
		return fmt.Errorf("输出器已关闭")
	default:
		return fmt.Errorf("提款通道已满，丢弃数据")
	}
}

// WriteReorgNotification 写入重组通知
func (o *AsyncFileOutput) WriteReorgNotification(reorg *models.ReorgNotification) error {
	if reorg == nil {
		return nil
	}
	
	select {
	case o.reorgChan <- reorg:
		return nil
	case <-o.ctx.Done():
		return fmt.Errorf("输出器已关闭")
	default:
		return fmt.Errorf("重组通知通道已满，丢弃数据")
	}
}

// Close 关闭异步文件输出器
func (o *AsyncFileOutput) Close() error {
	o.logger.Info("关闭异步文件输出器...")
	
	// 停止接收新数据
	o.cancel()
	
	// 等待所有工作器完成
	o.wg.Wait()
	
	// 关闭所有文件
	var errors []error
	for name, file := range o.files {
		if err := file.Close(); err != nil {
			errors = append(errors, fmt.Errorf("关闭文件 %s 失败: %w", name, err))
		}
	}
	
	// 关闭通道
	close(o.blockChan)
	close(o.txChan)
	close(o.logChan)
	close(o.internalTxChan)
	close(o.stateChangeChan)
	close(o.contractChan)
	close(o.withdrawalChan)
	close(o.reorgChan)
	
	if len(errors) > 0 {
		return fmt.Errorf("关闭文件时发生错误: %v", errors)
	}
	
	o.logger.Info("异步文件输出器已关闭")
	return nil
}

// GetStats 获取输出器统计信息
func (o *AsyncFileOutput) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"block_queue_size":       len(o.blockChan),
		"transaction_queue_size": len(o.txChan),
		"log_queue_size":         len(o.logChan),
		"internal_tx_queue_size": len(o.internalTxChan),
		"state_change_queue_size": len(o.stateChangeChan),
		"contract_queue_size":    len(o.contractChan),
		"withdrawal_queue_size":  len(o.withdrawalChan),
		"reorg_queue_size":       len(o.reorgChan),
		"batch_size":             o.batchSize,
		"flush_interval":         o.flushInterval.String(),
	}
}