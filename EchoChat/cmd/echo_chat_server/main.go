package main

// 本文件是后端服务启动入口，负责组装配置、消息模式、HTTP 服务和退出清理流程。

import (
	"context"
	"errors"
	"fmt"
	"echo_chat_server/internal/config"
	"echo_chat_server/internal/dao"
	"echo_chat_server/internal/https_server"
	"echo_chat_server/internal/service/chat"
	"echo_chat_server/internal/service/dlq"
	"echo_chat_server/internal/service/kafka"
	myredis "echo_chat_server/internal/service/redis"
	"echo_chat_server/pkg/zlog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// main 作为程序启动入口，负责串联配置加载、服务启动和退出清理。
func main() {
	conf := config.GetConfig()
	host := conf.MainConfig.Host
	port := conf.MainConfig.Port
	kafkaConfig := conf.KafkaConfig
	if err := chat.BackfillMessageSessionSeq(); err != nil {
		zlog.Fatal(err.Error())
	}
	if err := dao.EnsureMessageConversationSeqConstraint(); err != nil {
		zlog.Fatal(err.Error())
	}
	// Kafka 模式下需要先准备生产者和消费者，再启动聊天服务器主循环。
	if kafkaConfig.MessageMode == "kafka" {
		if err := kafka.KafkaService.CreateTopic(); err != nil {
			zlog.Fatal(err.Error())
		}
		kafka.KafkaService.KafkaInit()
	}

	// 根据配置决定消息转发走内存 channel 还是 Kafka 驱动的实现。
	if kafkaConfig.MessageMode == "channel" {
		go chat.ChatServer.Start()
	} else {
		go chat.KafkaChatServer.Start()
	}
	dlqReplayCtx, dlqReplayCancel := context.WithCancel(context.Background())
	defer dlqReplayCancel()
	go dlq.NewReplayScheduler(10*time.Second, 100, chat.RunDLQReplay).Start(dlqReplayCtx)

	httpServer := &http.Server{ //创建了一个 *http.Server 实例，用于启动一个 HTTP 服务器。
		Addr:              fmt.Sprintf("%s:%d", host, port),
		Handler:           https_server.GE,
		ReadHeaderTimeout: 5 * time.Second, //设置读取 HTTP 请求头的最大等待时间。
	}

	go func() {
		// HTTP 服务与消息服务解耦，单独放到 goroutine 中运行。
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			zlog.Fatal("server running fault")
			return
		}
	}()

	// 设置信号监听
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 等待信号
	<-quit
	zlog.Info("收到退出信号，开始优雅关机")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 先停止接受新 HTTP / WebSocket 请求，再收口聊天服务。
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		zlog.Error(err.Error())
	}

	if kafkaConfig.MessageMode == "channel" {
		if err := chat.ChatServer.Shutdown(shutdownCtx); err != nil {
			zlog.Error(err.Error())
		}
	} else {
		if err := chat.KafkaChatServer.Shutdown(shutdownCtx); err != nil {
			zlog.Error(err.Error())
		}
	}

	zlog.Info("关闭服务器...")

	// 只关闭 Redis 连接，不再删除整库数据，避免影响其他业务缓存和历史状态。
	if err := myredis.Close(); err != nil {
		zlog.Error(err.Error())
	} else {
		zlog.Info("Redis连接已关闭")
	}

	zlog.Info("服务器已关闭")

}
