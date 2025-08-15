package output

import (
	"encoding/json"
	"fmt"
	"time"

	"gather/pkg/models"

	"github.com/IBM/sarama"
	"github.com/sirupsen/logrus"
)

// KafkaOutput Kafka输出器
type KafkaOutput struct {
	logger   *logrus.Logger
	topics   map[string]string // 数据类型到topic的映射
	producer sarama.SyncProducer
}

// NewKafkaOutput 创建Kafka输出器
func NewKafkaOutput(brokers []string, topics map[string]string, logger *logrus.Logger) (*KafkaOutput, error) {
	logger.Infof("初始化Kafka输出器，brokers: %v", brokers)
	logger.Infof("Kafka topics配置: %v", topics)

	// 配置Kafka生产者
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Return.Successes = true
	config.Producer.Timeout = 5 * time.Second
	config.Version = sarama.V2_8_0_0

	// 创建同步生产者
	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, fmt.Errorf("创建Kafka生产者失败: %w", err)
	}

	logger.Info("Kafka生产者已创建")

	return &KafkaOutput{
		logger:   logger,
		topics:   topics,
		producer: producer,
	}, nil
}

// sendToKafka 发送数据到Kafka
func (k *KafkaOutput) sendToKafka(topic string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("序列化数据失败: %w", err)
	}

	// 创建Kafka消息
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.StringEncoder(jsonData),
	}

	// 发送消息
	partition, offset, err := k.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("发送消息到Kafka失败: %w", err)
	}

	k.logger.Infof("成功发送数据到Kafka topic '%s' (partition: %d, offset: %d): %s",
		topic, partition, offset, string(jsonData))

	return nil
}

// WriteBlock 写入区块数据
func (k *KafkaOutput) WriteBlock(block *models.Block) error {
	if block == nil {
		return nil
	}

	topic, exists := k.topics["blocks"]
	if !exists {
		topic = "blockchain_blocks"
	}

	return k.sendToKafka(topic, block)
}

// WriteTransaction 写入交易数据
func (k *KafkaOutput) WriteTransaction(tx *models.Transaction) error {
	if tx == nil {
		return nil
	}

	topic, exists := k.topics["transactions"]
	if !exists {
		topic = "blockchain_transactions"
	}

	return k.sendToKafka(topic, tx)
}

// WriteLog 写入日志数据
func (k *KafkaOutput) WriteLog(log *models.TransactionLog) error {
	if log == nil {
		return nil
	}

	topic, exists := k.topics["logs"]
	if !exists {
		topic = "blockchain_logs"
	}

	return k.sendToKafka(topic, log)
}

// WriteInternalTransaction 写入内部交易数据
func (k *KafkaOutput) WriteInternalTransaction(itx *models.InternalTransaction) error {
	if itx == nil {
		return nil
	}

	topic, exists := k.topics["internal_transactions"]
	if !exists {
		topic = "blockchain_internal_transactions"
	}

	return k.sendToKafka(topic, itx)
}

// WriteStateChange 写入状态变更数据
func (k *KafkaOutput) WriteStateChange(sc *models.StateChange) error {
	if sc == nil {
		return nil
	}

	topic, exists := k.topics["state_changes"]
	if !exists {
		topic = "blockchain_state_changes"
	}

	return k.sendToKafka(topic, sc)
}

// WriteContractCreation 写入合约创建数据
func (k *KafkaOutput) WriteContractCreation(cc *models.ContractCreation) error {
	if cc == nil {
		return nil
	}

	topic, exists := k.topics["contract_creations"]
	if !exists {
		topic = "blockchain_contract_creations"
	}

	return k.sendToKafka(topic, cc)
}

// WriteWithdrawal 写入提款数据
func (k *KafkaOutput) WriteWithdrawal(w *models.Withdrawal) error {
	if w == nil {
		return nil
	}

	topic, exists := k.topics["withdrawals"]
	if !exists {
		topic = "blockchain_withdrawals" // 默认topic名称
	}

	// 使用专门的Kafka消息格式
	return k.sendToKafka(topic, w.ToKafkaMessage())
}

// WriteReorgNotification 写入重组通知
func (k *KafkaOutput) WriteReorgNotification(reorg *models.ReorgNotification) error {
	if reorg == nil {
		return nil
	}

	topic, exists := k.topics["reorg_notifications"]
	if !exists {
		topic = "blockchain_reorg_notifications" // 默认topic名称
	}

	return k.sendToKafka(topic, reorg.ToKafkaMessage())
}

// Close 关闭Kafka连接
func (k *KafkaOutput) Close() error {
	if k.producer != nil {
		return k.producer.Close()
	}
	return nil
}
