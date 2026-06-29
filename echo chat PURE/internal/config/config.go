package config

// 本文件负责加载配置文件，并向项目其他模块暴露统一的配置读取入口。

import (
	"github.com/BurntSushi/toml"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

type MainConfig struct {
	// AppName 是服务名或默认数据库名。
	AppName string `toml:"appName"`
	// Host 是 HTTP 服务监听地址。
	Host string `toml:"host"`
	// Port 是 HTTP 服务监听端口。
	Port int `toml:"port"`
}

type MysqlConfig struct {
	// Host 是 MySQL 主机地址。
	Host string `toml:"host"`
	// Port 是 MySQL 端口。
	Port int `toml:"port"`
	// User 是数据库账号。
	User string `toml:"user"`
	// Password 是数据库密码，允许为空。
	Password string `toml:"password"`
	// DatabaseName 是实际连接的数据库名。
	DatabaseName string `toml:"databaseName"`
	// MaxOpenConns 是数据库连接池最大打开连接数。
	MaxOpenConns int `toml:"maxOpenConns"`
	// MaxIdleConns 是数据库连接池最大空闲连接数。
	MaxIdleConns int `toml:"maxIdleConns"`
	// ConnMaxLifetimeMinutes 是单条连接的最大生命周期，单位分钟。
	ConnMaxLifetimeMinutes int `toml:"connMaxLifetimeMinutes"`
	// ConnMaxIdleTimeMinutes 是单条连接的最大空闲时间，单位分钟。
	ConnMaxIdleTimeMinutes int `toml:"connMaxIdleTimeMinutes"`
}

type RedisConfig struct {
	// Host 是 Redis 服务地址。
	Host string `toml:"host"`
	// Port 是 Redis 服务端口。
	Port int `toml:"port"`
	// Password 是 Redis 密码。
	Password string `toml:"password"`
	// Db 是 Redis 逻辑库编号。
	Db int `toml:"db"`
}

type AuthCodeConfig struct {
	// AccessKeyID 是短信服务访问密钥 ID。
	AccessKeyID string `toml:"accessKeyID"`
	// AccessKeySecret 是短信服务访问密钥 Secret。
	AccessKeySecret string `toml:"accessKeySecret"`
	// SignName 是短信签名。
	SignName string `toml:"signName"`
	// TemplateCode 是短信模板编号。
	TemplateCode string `toml:"templateCode"`
}

type LogConfig struct {
	// LogPath 是日志文件输出位置。
	LogPath string `toml:"logPath"`
	// Level 是全局日志级别。
	Level string `toml:"level"`
	// DisableStdout 为 true 时不再把日志输出到 stdout。
	DisableStdout bool `toml:"disableStdout"`
}

type JwtConfig struct {
	// AccessExpireMinutes 是 access token 过期时长，单位分钟。
	AccessExpireMinutes int `toml:"accessExpireMinutes"`
	// RefreshExpireHours 是 refresh token 过期时长，单位小时。
	RefreshExpireHours int `toml:"refreshExpireHours"`
	// Issuer 是 JWT 签发者。
	Issuer string `toml:"issuer"`
	// Subject 是 JWT 主题。
	Subject string `toml:"subject"`
	// Key 是 JWT HS256 签名密钥。
	Key string `toml:"key"`
}

type KafkaConfig struct {
	// MessageMode 决定聊天消息走本地 channel 还是 Kafka。
	MessageMode string `toml:"messageMode"`
	// HostPort 是 Kafka broker 地址。
	HostPort string `toml:"hostPort"`
	// LoginTopic 是登录事件主题。
	LoginTopic string `toml:"loginTopic"`
	// LogoutTopic 是登出事件主题。
	LogoutTopic string `toml:"logoutTopic"`
	// ChatTopic 是聊天消息主题。
	ChatTopic string `toml:"chatTopic"`
	// MessageKey 是聊天消息写入 Kafka 时的兜底 key。
	// 当前项目会优先使用 session_id 作为消息 key；只有 session_id 为空时才回退到这里。
	MessageKey string `toml:"messageKey"`
	// TopicPartitions 是创建 chat topic 时使用的分区数量。
	TopicPartitions int `toml:"topicPartitions"`
	// TopicReplicationFactor 是创建 chat topic 时使用的副本因子。
	TopicReplicationFactor int16 `toml:"topicReplicationFactor"`
	// Partition 是旧版本遗留字段。
	// 兼容策略：
	// 1. 发送消息时，如果未配置 messageKey，则回退为 strconv.Itoa(Partition)
	// 2. 创建 topic 时，不再使用该字段表示“topic 分区数量”
	Partition int `toml:"partition"`
	// Timeout 是 Kafka 操作超时时间。
	Timeout time.Duration `toml:"timeout"`
	// ConsumerGroup 是聊天消息消费组名。
	ConsumerGroup string `toml:"consumerGroup"`
	// ProducerRetryMax 是生产端发送失败后的最大重试次数。
	ProducerRetryMax int `toml:"producerRetryMax"`
	// ProducerRetryBackoffMs 是生产端重试退避时间，单位毫秒。
	ProducerRetryBackoffMs int `toml:"producerRetryBackoffMs"`
	// MinInsyncReplicas 是创建 topic 时可选写入的 min.insync.replicas。
	MinInsyncReplicas int `toml:"minInsyncReplicas"`
	// ConsumerCommitBatchSize 是消费侧手动提交 offset 的批量阈值。
	ConsumerCommitBatchSize int `toml:"consumerCommitBatchSize"`
	// ConsumerCommitIntervalMs 是消费侧手动提交 offset 的时间窗口，单位毫秒。
	ConsumerCommitIntervalMs int `toml:"consumerCommitIntervalMs"`
	// MysqlPersistBatchSize 是 mysql_persist 批量写的批次大小。
	MysqlPersistBatchSize int `toml:"mysqlPersistBatchSize"`
	// MysqlPersistFlushIntervalMs 是 mysql_persist 批量写的最长等待时间，单位毫秒。
	MysqlPersistFlushIntervalMs int `toml:"mysqlPersistFlushIntervalMs"`
	// MysqlPersistFirstJobHoldMs 是 batch 第一条消息进入后额外等待后续消息补齐的时间，单位毫秒。
	MysqlPersistFirstJobHoldMs float64 `toml:"mysqlPersistFirstJobHoldMs"`
	// MysqlPersistWorkerCount 是 mysql_persist 批量写 worker 数量。
	MysqlPersistWorkerCount int `toml:"mysqlPersistWorkerCount"`
	// MysqlPersistQueueSize 是 mysql_persist 每个 worker 队列的缓冲长度。
	MysqlPersistQueueSize int `toml:"mysqlPersistQueueSize"`
	// SessionSeqRedisOnlyExperimental 为 true 时，session_seq 实验性跳过每条消息的 MySQL floor 查询，只保留 Redis 递增。
	SessionSeqRedisOnlyExperimental bool `toml:"sessionSeqRedisOnlyExperimental"`
	// MysqlPersistNoopExperimental 为 true 时，mysql_persist 实验性旁路真实消息落库，仅用于定位性能瓶颈。
	MysqlPersistNoopExperimental bool `toml:"mysqlPersistNoopExperimental"`
	// StatusUpdateNoopExperimental 为 true 时，实验性旁路 websocket 写回后的 status=sent 更新。
	StatusUpdateNoopExperimental bool `toml:"statusUpdateNoopExperimental"`
	// GroupAsyncPipelineExperimental 为 true 时，群聊消息实验性改为“顺序入口 + 下游子队列并行”。
	GroupAsyncPipelineExperimental bool `toml:"groupAsyncPipelineExperimental"`
	// GroupAsyncShardCount 是群聊异步子队列的 shard 数。
	GroupAsyncShardCount int `toml:"groupAsyncShardCount"`
	// PartitionAsyncEnabled 为 true 时，单聊消费实验性启用 fixed-shard 分区内二次并发。
	PartitionAsyncEnabled bool `toml:"partitionAsyncEnabled"`
	// PartitionAsyncShardCount 是单聊 fixed-shard 模式的 shard 数。
	PartitionAsyncShardCount int `toml:"partitionAsyncShardCount"`
	// PartitionAsyncQueueSize 是每个 shard 的缓冲队列长度。
	PartitionAsyncQueueSize int `toml:"partitionAsyncQueueSize"`
	// PartitionAsyncDrainTimeoutMs 是 claim 结束时等待 fixed-shard 在途消息回收的最长时间，单位毫秒。
	PartitionAsyncDrainTimeoutMs int `toml:"partitionAsyncDrainTimeoutMs"`
	// ConversationBucketEnabled 为 true 时，单聊消费实验性启用 conversation bucket 模式。
	ConversationBucketEnabled bool `toml:"conversationBucketEnabled"`
	// ConversationBucketWorkerCount 是每个分区内部的 conversation worker 数量。
	ConversationBucketWorkerCount int `toml:"conversationBucketWorkerCount"`
	// ConversationBucketReadyQueueSize 是每个分区 ready queue 的缓冲长度。
	ConversationBucketReadyQueueSize int `toml:"conversationBucketReadyQueueSize"`
	// ConversationBucketQueueSize 是每个会话 bucket 的缓冲长度。
	ConversationBucketQueueSize int `toml:"conversationBucketQueueSize"`
	// ConversationBucketMaxMessagesPerTurn 是每个 worker 单次处理同一个 bucket 的最大消息数。
	ConversationBucketMaxMessagesPerTurn int `toml:"conversationBucketMaxMessagesPerTurn"`
	// ConversationBucketMaxRunDurationMs 是每个 worker 单次处理同一个 bucket 的最长运行时间，单位毫秒。
	ConversationBucketMaxRunDurationMs int `toml:"conversationBucketMaxRunDurationMs"`
	// ConversationBucketDrainTimeoutMs 是 claim 结束时等待 conversation bucket 在途任务回收的最长时间，单位毫秒。
	ConversationBucketDrainTimeoutMs int `toml:"conversationBucketDrainTimeoutMs"`
}

// ResolveMessageKey 返回当前聊天消息写入 Kafka 时应使用的 key。
// 当前策略是：
// 1. 优先使用 session_id，让同一会话的消息尽量进入同一分区，便于保持会话内顺序
// 2. 如果 session_id 为空，则回退到配置里的 messageKey
// 3. 如果 messageKey 也为空，则继续兼容旧配置 partition
func (k KafkaConfig) ResolveMessageKey(sessionID string) string {
	if sessionID != "" {
		return sessionID
	}
	if k.MessageKey != "" {
		return k.MessageKey
	}
	return strconv.Itoa(k.Partition)
}

// ResolveTopicPartitions 返回创建 chat topic 时应使用的分区数量。
func (k KafkaConfig) ResolveTopicPartitions() int32 {
	if k.TopicPartitions > 0 {
		return int32(k.TopicPartitions)
	}
	return 1
}

// ResolveTopicReplicationFactor 返回创建 chat topic 时应使用的副本因子。
func (k KafkaConfig) ResolveTopicReplicationFactor() int16 {
	if k.TopicReplicationFactor > 0 {
		return k.TopicReplicationFactor
	}
	return 1
}

// ResolveConsumerGroup 返回聊天消息消费组名，未配置时默认使用 chat。
func (k KafkaConfig) ResolveConsumerGroup() string {
	if k.ConsumerGroup != "" {
		return k.ConsumerGroup
	}
	return "chat"
}

// ResolveProducerRetryMax 返回生产端重试次数，默认值兼顾基础可靠性与实现复杂度。
func (k KafkaConfig) ResolveProducerRetryMax() int {
	if k.ProducerRetryMax > 0 {
		return k.ProducerRetryMax
	}
	return 3
}

// ResolveProducerRetryBackoff 返回生产端重试退避时间。
func (k KafkaConfig) ResolveProducerRetryBackoff() time.Duration {
	if k.ProducerRetryBackoffMs > 0 {
		return time.Duration(k.ProducerRetryBackoffMs) * time.Millisecond
	}
	return 200 * time.Millisecond
}

// ResolveConsumerCommitBatchSize 返回消费侧批量提交阈值。
func (k KafkaConfig) ResolveConsumerCommitBatchSize() int {
	if k.ConsumerCommitBatchSize > 0 {
		return k.ConsumerCommitBatchSize
	}
	return 100
}

// ResolveConsumerCommitInterval 返回消费侧 offset 提交时间窗口。
func (k KafkaConfig) ResolveConsumerCommitInterval() time.Duration {
	if k.ConsumerCommitIntervalMs > 0 {
		return time.Duration(k.ConsumerCommitIntervalMs) * time.Millisecond
	}
	return 250 * time.Millisecond
}

// ResolveMysqlPersistBatchSize 返回 mysql_persist 批量写默认批次大小。
func (k KafkaConfig) ResolveMysqlPersistBatchSize() int {
	if k.MysqlPersistBatchSize > 0 {
		return k.MysqlPersistBatchSize
	}
	return 64
}

// ResolveMysqlPersistFlushInterval 返回 mysql_persist 批量写最长等待时间。
func (k KafkaConfig) ResolveMysqlPersistFlushInterval() time.Duration {
	if k.MysqlPersistFlushIntervalMs > 0 {
		return time.Duration(k.MysqlPersistFlushIntervalMs) * time.Millisecond
	}
	return 5 * time.Millisecond
}

// ResolveMysqlPersistFirstJobHold 返回 mysql_persist 第一条消息进入后额外等待的时间。
func (k KafkaConfig) ResolveMysqlPersistFirstJobHold() time.Duration {
	if k.MysqlPersistFirstJobHoldMs > 0 {
		return time.Duration(float64(time.Millisecond) * k.MysqlPersistFirstJobHoldMs)
	}
	return 0
}

// ResolveMysqlPersistWorkerCount 返回 mysql_persist worker 数量。
func (k KafkaConfig) ResolveMysqlPersistWorkerCount() int {
	if k.MysqlPersistWorkerCount > 0 {
		return k.MysqlPersistWorkerCount
	}
	return 8
}

// ResolveMysqlPersistQueueSize 返回 mysql_persist worker 队列长度。
func (k KafkaConfig) ResolveMysqlPersistQueueSize() int {
	if k.MysqlPersistQueueSize > 0 {
		return k.MysqlPersistQueueSize
	}
	return 2048
}

// UsePartitionAsyncExperimental 返回当前是否启用单聊 fixed-shard 分区内二次并发路径。
func (k KafkaConfig) UsePartitionAsyncExperimental() bool {
	return k.PartitionAsyncEnabled
}

// ResolvePartitionAsyncShardCount 返回单聊 fixed-shard 分区内二次并发的 shard 数。
func (k KafkaConfig) ResolvePartitionAsyncShardCount() int {
	if k.PartitionAsyncShardCount > 0 {
		return k.PartitionAsyncShardCount
	}
	return 4
}

// ResolvePartitionAsyncQueueSize 返回单聊 fixed-shard 分区内队列长度。
func (k KafkaConfig) ResolvePartitionAsyncQueueSize() int {
	if k.PartitionAsyncQueueSize > 0 {
		return k.PartitionAsyncQueueSize
	}
	return 512
}

// ResolvePartitionAsyncDrainTimeout 返回 fixed-shard claim drain 最长等待时间。
func (k KafkaConfig) ResolvePartitionAsyncDrainTimeout() time.Duration {
	if k.PartitionAsyncDrainTimeoutMs > 0 {
		return time.Duration(k.PartitionAsyncDrainTimeoutMs) * time.Millisecond
	}
	return 3 * time.Second
}

// UseConversationBucketExperimental 返回当前是否启用会话桶消费路径。
func (k KafkaConfig) UseConversationBucketExperimental() bool {
	return k.ConversationBucketEnabled
}

// ResolveConversationBucketWorkerCount 返回每个分区内部的 conversation worker 数量。
func (k KafkaConfig) ResolveConversationBucketWorkerCount() int {
	if k.ConversationBucketWorkerCount > 0 {
		return k.ConversationBucketWorkerCount
	}
	return 8
}

// ResolveConversationBucketReadyQueueSize 返回每个分区 ready queue 缓冲长度。
func (k KafkaConfig) ResolveConversationBucketReadyQueueSize() int {
	if k.ConversationBucketReadyQueueSize > 0 {
		return k.ConversationBucketReadyQueueSize
	}
	return 512
}

// ResolveConversationBucketQueueSize 返回每个会话 bucket 的缓冲长度。
func (k KafkaConfig) ResolveConversationBucketQueueSize() int {
	if k.ConversationBucketQueueSize > 0 {
		return k.ConversationBucketQueueSize
	}
	return 256
}

// ResolveConversationBucketMaxMessagesPerTurn 返回 worker 单次处理同一 bucket 的最大消息数。
func (k KafkaConfig) ResolveConversationBucketMaxMessagesPerTurn() int {
	if k.ConversationBucketMaxMessagesPerTurn > 0 {
		return k.ConversationBucketMaxMessagesPerTurn
	}
	return 32
}

// ResolveConversationBucketMaxRunDuration 返回 worker 单次处理同一 bucket 的最长运行时间。
func (k KafkaConfig) ResolveConversationBucketMaxRunDuration() time.Duration {
	if k.ConversationBucketMaxRunDurationMs > 0 {
		return time.Duration(k.ConversationBucketMaxRunDurationMs) * time.Millisecond
	}
	return 5 * time.Millisecond
}

// ResolveConversationBucketDrainTimeout 返回会话桶 claim drain 最长等待时间。
func (k KafkaConfig) ResolveConversationBucketDrainTimeout() time.Duration {
	if k.ConversationBucketDrainTimeoutMs > 0 {
		return time.Duration(k.ConversationBucketDrainTimeoutMs) * time.Millisecond
	}
	return 3 * time.Second
}

// UseRedisOnlySessionSeqExperimental 返回当前是否启用 session_seq 的 Redis-only 实验路径。
func (k KafkaConfig) UseRedisOnlySessionSeqExperimental() bool {
	return k.SessionSeqRedisOnlyExperimental
}

// UseMysqlPersistNoopExperimental 返回当前是否启用 mysql_persist 的 noop 实验路径。
func (k KafkaConfig) UseMysqlPersistNoopExperimental() bool {
	return k.MysqlPersistNoopExperimental
}

// UseStatusUpdateNoopExperimental 返回当前是否启用 status=sent 的 noop 实验路径。
func (k KafkaConfig) UseStatusUpdateNoopExperimental() bool {
	return k.StatusUpdateNoopExperimental
}

// UseGroupAsyncPipelineExperimental 返回当前是否启用群聊异步子队列实验路径。
func (k KafkaConfig) UseGroupAsyncPipelineExperimental() bool {
	return k.GroupAsyncPipelineExperimental
}

// ResolveGroupAsyncShardCount 返回群聊异步子队列 shard 数。
func (k KafkaConfig) ResolveGroupAsyncShardCount() int {
	if k.GroupAsyncShardCount > 0 {
		return k.GroupAsyncShardCount
	}
	return 4
}

type StaticSrcConfig struct {
	// StaticAvatarPath 是头像上传落盘目录。
	StaticAvatarPath string `toml:"staticAvatarPath"`
	// StaticFilePath 是聊天文件上传落盘目录。
	StaticFilePath string `toml:"staticFilePath"`
}

type Config struct {
	// MainConfig 聚合服务主监听配置。
	MainConfig `toml:"mainConfig"`
	// MysqlConfig 聚合数据库配置。
	MysqlConfig `toml:"mysqlConfig"`
	// RedisConfig 聚合缓存配置。
	RedisConfig `toml:"redisConfig"`
	// AuthCodeConfig 聚合短信验证码配置。
	AuthCodeConfig `toml:"authCodeConfig"`
	// LogConfig 聚合日志输出配置。
	LogConfig `toml:"logConfig"`
	// JwtConfig 聚合 JWT 配置。
	JwtConfig `toml:"jwtConfig"`
	// KafkaConfig 聚合消息队列配置。
	KafkaConfig `toml:"kafkaConfig"`
	// StaticSrcConfig 聚合静态资源目录配置。
	StaticSrcConfig `toml:"staticSrcConfig"`
}

// config 作为单例缓存，避免在各模块中重复解析配置文件。
var config *Config

func resolvePathFromConfigDir(baseDir, raw string) string {
	if raw == "" || filepath.IsAbs(raw) {
		return raw
	}
	return filepath.Clean(filepath.Join(baseDir, raw))
}

func normalizeRelativePaths(conf *Config, candidate string) {
	baseDir := filepath.Dir(candidate)
	conf.LogConfig.LogPath = resolvePathFromConfigDir(baseDir, conf.LogConfig.LogPath)
	conf.StaticSrcConfig.StaticAvatarPath = resolvePathFromConfigDir(baseDir, conf.StaticSrcConfig.StaticAvatarPath)
	conf.StaticSrcConfig.StaticFilePath = resolvePathFromConfigDir(baseDir, conf.StaticSrcConfig.StaticFilePath)
}

func candidateConfigPaths() []string {
	candidates := []string{}
	if envPath := os.Getenv("ECHOCHAT_CONFIG"); envPath != "" {
		candidates = append(candidates, envPath)
	}
	candidates = append(candidates, "configs/config_local.toml", "configs/config.toml")
	if repoRoot := os.Getenv("ECHOCHAT_REPO_ROOT"); repoRoot != "" {
		candidates = append(candidates,
			filepath.Join(repoRoot, "configs", "config_local.toml"),
			filepath.Join(repoRoot, "configs", "config.toml"),
		)
	}
	if cwd, err := os.Getwd(); err == nil {
		current := cwd
		for {
			candidates = append(candidates,
				filepath.Join(current, "configs", "config_local.toml"),
				filepath.Join(current, "configs", "config.toml"),
			)
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
			current = parent
		}
	}
	execPath, err := os.Executable()
	if err == nil {
		current := filepath.Dir(execPath)
		for {
			candidates = append(candidates,
				filepath.Join(current, "configs", "config_local.toml"),
				filepath.Join(current, "configs", "config.toml"),
			)
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
			current = parent
		}
	}
	if _, sourceFile, _, ok := runtime.Caller(0); ok {
		current := filepath.Dir(filepath.Dir(filepath.Dir(sourceFile)))
		for {
			candidates = append(candidates,
				filepath.Join(current, "configs", "config_local.toml"),
				filepath.Join(current, "configs", "config.toml"),
			)
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
			current = parent
		}
	}
	seen := map[string]struct{}{}
	unique := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		clean := filepath.Clean(candidate)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		unique = append(unique, clean)
	}
	return unique
}

// LoadConfig 按顺序尝试多个候选路径，优先加载环境变量显式指定的配置文件。
func LoadConfig() error {
	for _, candidate := range candidateConfigPaths() {
		// 只要候选文件存在，就立即尝试解析并返回。
		if _, err := os.Stat(candidate); err == nil {
			if _, err := toml.DecodeFile(candidate, config); err != nil {
				log.Fatal(err.Error())
				return err
			}
			normalizeRelativePaths(config, candidate)
			return nil
		}
	}
	log.Fatal("failed to locate config file")
	return os.ErrNotExist
}

// GetConfig 对外暴露配置读取入口，并在首次调用时完成懒加载。
func GetConfig() *Config {
	if config == nil {
		config = new(Config)
		_ = LoadConfig()
	}
	return config
}
