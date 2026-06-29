package kafka

// 本文件封装 Kafka 生产与消费初始化逻辑，供消息模式切换时复用。

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
	myconfig "echo_chat_server/internal/config"
	"echo_chat_server/internal/observability"
	"echo_chat_server/internal/pressure"
	"echo_chat_server/pkg/zlog"
)

type kafkaService struct {
	// ChatWriter 负责把聊天消息写入 chat topic。
	ChatWriter sarama.SyncProducer
	// ChatReader 负责以消费组方式读取 chat topic。
	ChatReader sarama.ConsumerGroup
	// kafkaConfig 保存 Sarama 的公共配置，便于 Producer / Consumer / Admin 复用。
	kafkaConfig *sarama.Config
	// brokers 是从配置里解析出的 broker 地址列表。
	brokers []string
}

var KafkaService = new(kafkaService)

// KafkaInit 初始化 Kafka 生产者与消费者。
func (k *kafkaService) KafkaInit() {
	conf := myconfig.GetConfig().KafkaConfig
	k.brokers = splitBrokers(conf.HostPort)
	if len(k.brokers) == 0 {
		zlog.Fatal("kafka hostPort 配置错误：未解析到有效 broker，请检查 kafkaConfig.hostPort")
	}
	k.kafkaConfig = newSaramaConfig(conf)

	writer, err := sarama.NewSyncProducer(k.brokers, k.kafkaConfig)
	if err != nil {
		zlog.Fatal(err.Error())
	}
	k.ChatWriter = writer

	reader, err := sarama.NewConsumerGroup(k.brokers, conf.ResolveConsumerGroup(), k.kafkaConfig)
	if err != nil {
		_ = writer.Close()
		zlog.Fatal(err.Error())
	}
	k.ChatReader = reader
}

func splitBrokers(hostPort string) []string {
	// 这个函数的目标是把配置中的 broker 字符串整理成 []string。
	// 例如：
	// "127.0.0.1:9092, 127.0.0.1:9093,127.0.0.1:9094"
	// 会先被 Split 成：
	// ["127.0.0.1:9092", " 127.0.0.1:9093", "127.0.0.1:9094"]
	// 再经过 TrimSpace 后变成：
	// ["127.0.0.1:9092", "127.0.0.1:9093", "127.0.0.1:9094"]
	rawBrokers := strings.Split(hostPort, ",")
	brokers := make([]string, 0, len(rawBrokers))
	for _, broker := range rawBrokers {
		// 去掉每个 broker 两端的空格，避免手写配置时因为空格导致地址不合法。
		broker = strings.TrimSpace(broker)
		if broker != "" {
			// 只保留非空地址。
			// 例如 "127.0.0.1:9092,,127.0.0.1:9093,"
			// Split 后可能得到空字符串，这里会被跳过。
			brokers = append(brokers, broker)
		}
	}
	// 如果最终一个有效 broker 都没有，说明该配置没有提供可用地址。
	// 例如 hostPort 是 ""、"   "、",,,"、" , , " 这类值时，
	// Split 之后的元素在 TrimSpace 后都会变成空字符串，最终全部被过滤掉。
	return brokers
}

func newSaramaConfig(conf myconfig.KafkaConfig) *sarama.Config {
	timeout := conf.Timeout
	config := sarama.NewConfig()         // 创建一份 Sarama 默认配置，后面在这个基础上按项目需要覆盖。
	config.Version = sarama.V3_6_0_0     // 指定 Kafka 协议版本，确保客户端按这个版本能力与 broker 通信。
	config.ClientID = "echo-chat-server" // 给当前客户端设置标识，便于在 Kafka 侧日志和监控中区分来源。

	config.Producer.Return.Successes = true                    // 开启后 SendMessage 成功时才会返回 partition 和 offset，SyncProducer 必须打开。
	config.Producer.Partitioner = sarama.NewHashPartitioner    // 按消息 key 做哈希分区；相同 key 的消息会尽量进入同一分区以保持分区内顺序。
	config.Producer.RequiredAcks = sarama.WaitForAll           // 等待 ISR 副本确认后再返回，提升消息写入成功语义。
	config.Producer.Timeout = timeout * time.Second            // 生产者等待 broker 确认的超时时间，超过后本次发送会报错。
	config.Producer.Retry.Max = conf.ResolveProducerRetryMax() // 发送异常时自动重试，减少短暂抖动造成的失败。
	config.Producer.Retry.Backoff = conf.ResolveProducerRetryBackoff()
	config.Producer.Idempotent = true // 开启幂等生产，配合重试避免同一条消息被重复写入。

	config.Consumer.Offsets.Initial = sarama.OffsetNewest               // 当消费组没有历史 offset 时，从最新消息开始消费，不回头读旧消息。
	config.Consumer.Offsets.AutoCommit.Enable = false                   // 关闭自动提交 offset，改为业务处理成功后手动提交。
	config.Consumer.Offsets.AutoCommit.Interval = timeout * time.Second // 保留默认提交间隔配置，兼容旧配置和后续切回自动提交的场景。
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
		sarama.NewBalanceStrategyRoundRobin(), // 消费组重平衡时使用轮询分配策略，把分区尽量平均分给各消费者。
	}

	config.Net.DialTimeout = timeout * time.Second  // 建立 TCP 连接的超时时间，broker 长时间连不上时及时失败。
	config.Net.ReadTimeout = timeout * time.Second  // 从 broker 读取响应的超时时间，避免读操作无限阻塞。
	config.Net.WriteTimeout = timeout * time.Second // 向 broker 写请求的超时时间，避免写操作长时间卡住。
	config.Net.MaxOpenRequests = 1                  // Sarama 幂等生产要求单连接上最多 1 个未完成请求。

	return config
}

// PublishChatMessage 把一条聊天消息写入 chat topic。
func (k *kafkaService) PublishChatMessage(_ context.Context, key string, value []byte, messageID string) error {
	if k.ChatWriter == nil {
		return errors.New("kafka chat writer is not initialized")
	}
	topic := myconfig.GetConfig().KafkaConfig.ChatTopic
	start := time.Now()
	pressure.ObserveBenchmarkEventAt(messageID, "kafka_producer_send_start", start, map[string]interface{}{
		"topic": topic,
		"key":   key,
	})
	partition, offset, err := k.ChatWriter.SendMessage(&sarama.ProducerMessage{
		Topic:     myconfig.GetConfig().KafkaConfig.ChatTopic,
		Key:       sarama.StringEncoder(key),
		Value:     sarama.ByteEncoder(value),
		Timestamp: time.Now(),
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("message_id"),
				Value: []byte(messageID),
			},
		},
	})
	if err != nil {
		observability.ObserveKafkaProduce(topic, 0, false, time.Since(start))
		zlog.Error(
			"kafka.produce.chat.failed",
			zap.String("event", "kafka.produce.chat.failed"),
			zap.String("module", "kafka.service"),
			zap.String("topic", topic),
			zap.String("key", key),
			zap.String("message_id", messageID),
			zap.Int("value_size", len(value)),
			zap.String("error", err.Error()),
		)
		return err
	}
	ackAt := time.Now()
	pressure.ObserveBenchmarkEventAt(messageID, "kafka_producer_ack_done", ackAt, map[string]interface{}{
		"topic":     topic,
		"key":       key,
		"partition": partition,
		"offset":    offset,
	})
	observability.ObserveKafkaProduce(topic, partition, true, time.Since(start))
	zlog.Info(
		"kafka.produce.chat",
		zap.String("event", "kafka.produce.chat"),
		zap.String("module", "kafka.service"),
		zap.String("topic", topic),
		zap.String("key", key),
		zap.String("message_id", messageID),
		zap.Int("value_size", len(value)),
		zap.Int32("partition", partition),
		zap.Int64("offset", offset),
	)
	return nil
}

// ConsumeChatMessages 以消费组方式持续消费 chat topic。
func (k *kafkaService) ConsumeChatMessages(ctx context.Context, handler sarama.ConsumerGroupHandler) error {
	if k.ChatReader == nil {
		return errors.New("kafka chat reader is not initialized")
	}
	return k.ChatReader.Consume(ctx, []string{myconfig.GetConfig().KafkaConfig.ChatTopic}, handler)
}

// KafkaClose 负责关闭 Kafka 读写器，释放网络连接和后台协程。
func (k *kafkaService) KafkaClose() {
	if k.ChatWriter != nil {
		if err := k.ChatWriter.Close(); err != nil {
			zlog.Error(err.Error())
		}
		k.ChatWriter = nil
	}
	if k.ChatReader != nil {
		if err := k.ChatReader.Close(); err != nil {
			zlog.Error(err.Error())
		}
		k.ChatReader = nil
	}
}

// CreateTopic 创建 chat topic；如果 topic 已存在则直接跳过。
func (k *kafkaService) CreateTopic() error {
	kafkaConfig := myconfig.GetConfig().KafkaConfig
	brokers := splitBrokers(kafkaConfig.HostPort)
	if len(brokers) == 0 {
		return errors.New("kafka hostPort 配置错误：未解析到有效 broker，请检查 kafkaConfig.hostPort")
	}
	admin, err := sarama.NewClusterAdmin(brokers, newSaramaConfig(kafkaConfig))
	if err != nil {
		return err
	}
	defer func() {
		if err := admin.Close(); err != nil {
			zlog.Error(err.Error())
		}
	}()

	detail := &sarama.TopicDetail{
		NumPartitions:     kafkaConfig.ResolveTopicPartitions(),
		ReplicationFactor: kafkaConfig.ResolveTopicReplicationFactor(),
	}
	if kafkaConfig.MinInsyncReplicas > 0 {
		detail.ConfigEntries = map[string]*string{
			"min.insync.replicas": stringPtr(strconv.Itoa(kafkaConfig.MinInsyncReplicas)),
		}
	}
	if err := admin.CreateTopic(kafkaConfig.ChatTopic, detail, false); err != nil {
		if errors.Is(err, sarama.ErrTopicAlreadyExists) || strings.Contains(err.Error(), sarama.ErrTopicAlreadyExists.Error()) {
			return ensureTopicPartitions(admin, kafkaConfig.ChatTopic, kafkaConfig.ResolveTopicPartitions())
		}
		return err
	}
	return nil
}

func stringPtr(value string) *string {
	return &value
}

func ensureTopicPartitions(admin sarama.ClusterAdmin, topic string, desiredPartitions int32) error {
	if desiredPartitions <= 1 {
		return nil
	}
	metadata, err := admin.DescribeTopics([]string{topic})
	if err != nil {
		return err
	}
	if len(metadata) == 0 {
		return errors.New("kafka topic metadata is empty")
	}
	currentPartitions := int32(len(metadata[0].Partitions))
	if currentPartitions >= desiredPartitions {
		return nil
	}
	return admin.CreatePartitions(topic, desiredPartitions, nil, false)
}
