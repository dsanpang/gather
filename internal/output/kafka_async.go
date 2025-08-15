package output

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"gather/pkg/models"

	"github.com/IBM/sarama"
	"github.com/sirupsen/logrus"
)

// AsyncKafkaOutput 异步Kafka输出器
type AsyncKafkaOutput struct {
	logger      *logrus.Logger
	topics      map[string]string
	producer    sarama.AsyncProducer
	successChan <-chan *sarama.ProducerMessage
	errorChan   <-chan *sarama.ProducerError
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup

	// 统计信息
	sentCount  int64
	errorCount int64
	mu         sync.RWMutex
}

// NewAsyncKafkaOutput 创建异步Kafka输出器
func NewAsyncKafkaOutput(brokers []string, topics map[string]string, logger *logrus.Logger) (*AsyncKafkaOutput, error) {
	logger.Infof("初始化异步Kafka输出器，brokers: %v", brokers)
	logger.Infof("Kafka topics配置: %v", topics)

	// 配置异步Kafka生产者
	config := sarama.NewConfig()

	// 异步生产者配置
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Producer.RequiredAcks = sarama.WaitForLocal // 更快的响应
	config.Producer.Retry.Max = 3
	config.Producer.Timeout = 3 * time.Second
	config.Version = sarama.V2_8_0_0

	// 性能优化配置
	config.Producer.Flush.Frequency = 100 * time.Millisecond // 批量发送频率
	config.Producer.Flush.Messages = 100                     // 批量发送消息数
	config.Producer.Flush.Bytes = 1024 * 1024                // 1MB批量大小
	config.Producer.Compression = sarama.CompressionSnappy   // 启用压缩

	// 缓冲区配置
	config.ChannelBufferSize = 1000 // 增大通道缓冲区

	// 创建异步生产者
	producer, err := sarama.NewAsyncProducer(brokers, config)
	if err != nil {
		return nil, fmt.Errorf("创建异步Kafka生产者失败: %w", err)
	}

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())

	asyncOutput := &AsyncKafkaOutput{
		logger:      logger,
		topics:      topics,
		producer:    producer,
		successChan: producer.Successes(),
		errorChan:   producer.Errors(),
		ctx:         ctx,
		cancel:      cancel,
	}

	// 启动后台处理goroutines
	asyncOutput.startBackgroundHandlers()

	logger.Info("异步Kafka生产者已创建并启动")
	return asyncOutput, nil
}

// startBackgroundHandlers 启动后台处理程序
func (k *AsyncKafkaOutput) startBackgroundHandlers() {
	// 成功消息处理器
	k.wg.Add(1)
	go func() {
		defer k.wg.Done()
		k.handleSuccesses()
	}()

	// 错误消息处理器
	k.wg.Add(1)
	go func() {
		defer k.wg.Done()
		k.handleErrors()
	}()

	// 统计信息报告器
	k.wg.Add(1)
	go func() {
		defer k.wg.Done()
		k.reportStats()
	}()
}

// handleSuccesses 处理成功发送的消息
func (k *AsyncKafkaOutput) handleSuccesses() {
	for {
		select {
		case success := <-k.successChan:
			if success != nil {
				k.mu.Lock()
				k.sentCount++
				k.mu.Unlock()

				k.logger.Debugf("消息成功发送到 topic %s, partition %d, offset %d",
					success.Topic, success.Partition, success.Offset)
			}
		case <-k.ctx.Done():
			k.logger.Debug("成功消息处理器停止")
			return
		}
	}
}

// handleErrors 处理发送失败的消息
func (k *AsyncKafkaOutput) handleErrors() {
	for {
		select {
		case err := <-k.errorChan:
			if err != nil {
				k.mu.Lock()
				k.errorCount++
				k.mu.Unlock()

				k.logger.Errorf("Kafka发送失败: topic=%s, partition=%d, offset=%d, error=%v",
					err.Msg.Topic, err.Msg.Partition, err.Msg.Offset, err.Err)
			}
		case <-k.ctx.Done():
			k.logger.Debug("错误消息处理器停止")
			return
		}
	}
}

// reportStats 定期报告统计信息
func (k *AsyncKafkaOutput) reportStats() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			k.mu.RLock()
			sent := k.sentCount
			errors := k.errorCount
			k.mu.RUnlock()

			if sent > 0 || errors > 0 {
				successRate := float64(sent) / float64(sent+errors) * 100
				k.logger.Infof("Kafka统计: 已发送 %d 条消息, 失败 %d 条, 成功率 %.2f%%",
					sent, errors, successRate)
			}
		case <-k.ctx.Done():
			k.logger.Debug("统计报告器停止")
			return
		}
	}
}

// sendToKafkaAsync 异步发送数据到Kafka
func (k *AsyncKafkaOutput) sendToKafkaAsync(topic string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("序列化数据失败: %w", err)
	}

	// 创建Kafka消息
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.StringEncoder(jsonData),
		// 可以添加分区键以确保相关消息在同一分区
		// Key: sarama.StringEncoder(partitionKey),
	}

	// 异步发送消息
	select {
	case k.producer.Input() <- msg:
		// 消息已发送到输入通道
		return nil
	case <-k.ctx.Done():
		return fmt.Errorf("Kafka生产者已关闭")
	default:
		return fmt.Errorf("Kafka生产者输入通道已满")
	}
}

// WriteBlock 异步写入区块数据
func (k *AsyncKafkaOutput) WriteBlock(block *models.Block) error {
	if block == nil {
		return nil
	}

	topic, exists := k.topics["blocks"]
	if !exists {
		topic = "blockchain_blocks"
	}

	return k.sendToKafkaAsync(topic, block)
}

// WriteTransaction 异步写入交易数据
func (k *AsyncKafkaOutput) WriteTransaction(tx *models.Transaction) error {
	if tx == nil {
		return nil
	}

	topic, exists := k.topics["transactions"]
	if !exists {
		topic = "blockchain_transactions"
	}

	return k.sendToKafkaAsync(topic, tx)
}

// WriteLog 异步写入日志数据
func (k *AsyncKafkaOutput) WriteLog(log *models.TransactionLog) error {
	if log == nil {
		return nil
	}

	topic, exists := k.topics["logs"]
	if !exists {
		topic = "blockchain_logs"
	}

	return k.sendToKafkaAsync(topic, log)
}

// WriteInternalTransaction 异步写入内部交易数据
func (k *AsyncKafkaOutput) WriteInternalTransaction(itx *models.InternalTransaction) error {
	if itx == nil {
		return nil
	}

	topic, exists := k.topics["internal_transactions"]
	if !exists {
		topic = "blockchain_internal_transactions"
	}

	return k.sendToKafkaAsync(topic, itx)
}

// WriteStateChange 异步写入状态变更数据
func (k *AsyncKafkaOutput) WriteStateChange(sc *models.StateChange) error {
	if sc == nil {
		return nil
	}

	topic, exists := k.topics["state_changes"]
	if !exists {
		topic = "blockchain_state_changes"
	}

	return k.sendToKafkaAsync(topic, sc)
}

// WriteContractCreation 异步写入合约创建数据
func (k *AsyncKafkaOutput) WriteContractCreation(cc *models.ContractCreation) error {
	if cc == nil {
		return nil
	}

	topic, exists := k.topics["contract_creations"]
	if !exists {
		topic = "blockchain_contract_creations"
	}

	return k.sendToKafkaAsync(topic, cc)
}

// WriteWithdrawal 异步写入提款数据
func (k *AsyncKafkaOutput) WriteWithdrawal(w *models.Withdrawal) error {
	if w == nil {
		return nil
	}

	topic, exists := k.topics["withdrawals"]
	if !exists {
		topic = "blockchain_withdrawals"
	}

	return k.sendToKafkaAsync(topic, w.ToKafkaMessage())
}

// WriteReorgNotification 异步写入重组通知
func (k *AsyncKafkaOutput) WriteReorgNotification(reorg *models.ReorgNotification) error {
	if reorg == nil {
		return nil
	}

	topic, exists := k.topics["reorg_notifications"]
	if !exists {
		topic = "blockchain_reorg_notifications"
	}

	return k.sendToKafkaAsync(topic, reorg.ToKafkaMessage())
}

// Flush 刷新所有缓冲的消息
func (k *AsyncKafkaOutput) Flush() error {
	k.logger.Info("刷新Kafka生产者缓冲区...")

	// 等待一段时间让异步消息处理完成
	time.Sleep(1 * time.Second)

	// 检查是否还有待处理的消息
	k.mu.RLock()
	pending := len(k.producer.Input())
	k.mu.RUnlock()

	if pending > 0 {
		k.logger.Infof("等待 %d 条消息完成发送...", pending)

		// 等待所有消息发送完成
		timeout := time.After(30 * time.Second)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				k.mu.RLock()
				remaining := len(k.producer.Input())
				k.mu.RUnlock()

				if remaining == 0 {
					k.logger.Info("所有消息已发送完成")
					return nil
				}
			case <-timeout:
				k.logger.Warn("刷新超时，部分消息可能未发送完成")
				return fmt.Errorf("刷新超时")
			}
		}
	}

	k.logger.Info("缓冲区刷新完成")
	return nil
}

// GetStats 获取统计信息
func (k *AsyncKafkaOutput) GetStats() (int64, int64) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.sentCount, k.errorCount
}

// Close 关闭异步Kafka连接
func (k *AsyncKafkaOutput) Close() error {
	k.logger.Info("关闭异步Kafka生产者...")

	// 首先刷新缓冲区
	if err := k.Flush(); err != nil {
		k.logger.Warnf("刷新缓冲区时出现错误: %v", err)
	}

	// 取消上下文，停止后台goroutines
	k.cancel()

	// 关闭生产者
	if err := k.producer.Close(); err != nil {
		k.logger.Errorf("关闭Kafka生产者失败: %v", err)
		return err
	}

	// 等待所有goroutines完成
	k.wg.Wait()

	// 最终统计信息
	sent, errors := k.GetStats()
	k.logger.Infof("异步Kafka生产者已关闭，总计发送: %d，错误: %d", sent, errors)

	return nil
}
