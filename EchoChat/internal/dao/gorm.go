package dao

// 本文件负责初始化 Gorm 数据库连接，供整个后端服务共享使用。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"echo_chat_server/internal/config"
	"echo_chat_server/internal/model"
	"echo_chat_server/pkg/zlog"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"time"
)

// GormDB 是整个服务共享的数据库连接句柄。
var GormDB *gorm.DB

func shouldSkipDBInit() bool {
	if os.Getenv("ECHOCHAT_SKIP_DAO_INIT") == "1" {
		return true
	}
	return isRunningUnderGoTest()
}

func isRunningUnderGoTest() bool {
	if len(os.Args) == 0 {
		return false
	}
	executableName := filepath.Base(os.Args[0])
	return strings.HasSuffix(executableName, ".test")
}

// init 在进程启动时建立数据库连接并自动迁移核心表结构。
func init() {
	if shouldSkipDBInit() {
		return
	}
	conf := config.GetConfig()
	user := conf.MysqlConfig.User
	password := conf.MysqlConfig.Password
	host := conf.MysqlConfig.Host
	port := conf.MysqlConfig.Port
	databaseName := conf.MysqlConfig.DatabaseName
	var dsn string
	// 兼容无密码的本地开发环境和带密码的部署环境。
	if password == "" {
		dsn = fmt.Sprintf("%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", user, host, port, databaseName)
	} else {
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", user, password, host, port, databaseName)
	}
	var err error
	GormDB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
		Logger:                 logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		zlog.Fatal(err.Error())
	}
	sqlDB, err := GormDB.DB()
	if err != nil {
		zlog.Fatal(err.Error())
	}
	maxOpenConns := conf.MysqlConfig.MaxOpenConns
	if maxOpenConns <= 0 {
		maxOpenConns = 200
	}
	maxIdleConns := conf.MysqlConfig.MaxIdleConns
	if maxIdleConns <= 0 {
		maxIdleConns = 50
	}
	connMaxLifetime := time.Duration(conf.MysqlConfig.ConnMaxLifetimeMinutes) * time.Minute
	if connMaxLifetime <= 0 {
		connMaxLifetime = 30 * time.Minute
	}
	connMaxIdleTime := time.Duration(conf.MysqlConfig.ConnMaxIdleTimeMinutes) * time.Minute
	if connMaxIdleTime <= 0 {
		connMaxIdleTime = 10 * time.Minute
	}
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)
	// AutoMigrate 只负责补齐/更新表结构，不会清空已有业务数据。
	err = GormDB.AutoMigrate(
		&model.UserInfo{},
		&model.GroupInfo{},
		&model.UserContact{},
		&model.Session{},
		&model.ContactApply{},
		&model.Message{},
		&model.ConversationSequence{},
		&model.DLQMessage{},
		&model.DLQOperationLog{},
	) // 自动迁移，如果没有建表，会自动创建对应的表
	if err != nil {
		zlog.Fatal(err.Error())
	}
	if err := backfillMessageConversationKey(); err != nil {
		zlog.Fatal(err.Error())
	}
	if err := optimizeMessageIndexes(); err != nil {
		zlog.Fatal(err.Error())
	}
}

type messageConversationSeqDuplicate struct {
	ConversationKey string
	SessionSeq      int64
	DuplicateCount  int64
}

func backfillMessageConversationKey() error {
	const batchSize = 5000
	for {
		res := GormDB.Exec(`
UPDATE message
SET conversation_key = CASE
	WHEN receive_id LIKE 'G%' THEN CONCAT('group:', receive_id)
	WHEN send_id < receive_id THEN CONCAT('user:', send_id, ':', receive_id)
	ELSE CONCAT('user:', receive_id, ':', send_id)
END
WHERE conversation_key = ''
LIMIT ?
`, batchSize)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
	}
}

func optimizeMessageIndexes() error {
	migrator := GormDB.Migrator()

	// 写入路径已经收敛到 conversation_key + session_seq，旧的单列索引会明显放大 insert 成本。
	for _, indexName := range []string{
		"idx_message_send_id",
		"idx_message_receive_id",
		"idx_message_session_id",
		"idx_message_session_seq",
	} {
		if migrator.HasIndex(&model.Message{}, indexName) {
			if err := migrator.DropIndex(&model.Message{}, indexName); err != nil {
				return err
			}
		}
	}
	return nil
}

func FindMessageConversationSeqDuplicate() (*messageConversationSeqDuplicate, error) {
	var duplicate messageConversationSeqDuplicate
	err := GormDB.
		Table("message").
		Select("conversation_key, session_seq, COUNT(*) AS duplicate_count").
		Where("session_seq > 0").
		Group("conversation_key, session_seq").
		Having("COUNT(*) > 1").
		Order("duplicate_count DESC, conversation_key ASC, session_seq ASC").
		Limit(1).
		Scan(&duplicate).Error
	if err != nil {
		return nil, err
	}
	if duplicate.DuplicateCount == 0 {
		return nil, nil
	}
	return &duplicate, nil
}

func EnsureMessageConversationSeqConstraint() error {
	duplicate, err := FindMessageConversationSeqDuplicate()
	if err != nil {
		return err
	}
	if duplicate != nil {
		return fmt.Errorf(
			"message table has duplicate conversation_key/session_seq: conversation_key=%s session_seq=%d duplicate_count=%d",
			duplicate.ConversationKey,
			duplicate.SessionSeq,
			duplicate.DuplicateCount,
		)
	}

	migrator := GormDB.Migrator()
	if migrator.HasIndex(&model.Message{}, "uniq_message_conversation_seq") {
		return nil
	}
	if migrator.HasIndex(&model.Message{}, "idx_message_conversation_seq") {
		if err := migrator.DropIndex(&model.Message{}, "idx_message_conversation_seq"); err != nil {
			return err
		}
	}
	return migrator.CreateIndex(&model.Message{}, "uniq_message_conversation_seq")
}
