package zlog

// 本文件负责初始化项目级日志组件，统一输出到控制台和日志文件。

import (
	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"echo_chat_server/internal/config"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// logger 是项目中统一复用的 zap 日志实例。
var logger *zap.Logger

// logPath 记录当前日志输出文件位置，供切割器等辅助方法使用。
var logPath string

// init 在包加载时完成日志编码器、输出目标和双写核心的初始化。
func init() {
	// 使用 zap 日志库创建一个适用于生产环境的默认编码器配置。
	encoderConfig := zap.NewProductionEncoderConfig() // 创建 zap 生产环境默认的 EncoderConfig 结构体实例

	// 设置日志记录中时间格式为 ISO8601 标准格式（如 "2006-01-02T15:04:05.000Z0700"）
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder // 覆盖默认时间编码器为 ISO8601 格式

	// 设置日志级别以小写形式输出（如 "info", "error" 而非 "INFO", "ERROR"）
	encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder // 覆盖默认日志级别编码器为小写格式

	// 使用 JSON 编码器将日志格式化为 JSON 格式输出
	encoder := zapcore.NewJSONEncoder(encoderConfig) // 基于上述配置创建 JSON 格式的日志编码器

	// 从项目配置中获取日志文件路径
	conf := config.GetConfig() // 加载全局配置对象

	// 将日志文件路径保存到包级变量 logPath，供后续滚动日志使用
	logPath = conf.LogPath // 记录日志文件的完整路径

	// 确保日志文件所在目录存在，若不存在则递归创建（权限 0755）
	_ = os.MkdirAll(filepath.Dir(logPath), 0755) // 创建日志目录（忽略错误）

	// 获取支持日志滚动切割的文件写入器（基于 lumberjack）
	fileWriteSyncer := getFileLogWriter() // 返回一个可自动切割日志文件的 WriteSyncer

	level := parseLevel(conf.LogConfig.Level)
	cores := []zapcore.Core{
		zapcore.NewCore(encoder, fileWriteSyncer, level),
	}
	if !conf.LogConfig.DisableStdout {
		cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), level))
	}
	core := zapcore.NewTee(cores...)

	// 初始化全局 logger 实例，不启用自动 caller（调用位置由自定义函数手动注入）
	logger = zap.New(core) // 创建最终的 zap.Logger 实例

	// 将标准库的 log 包输出重定向到当前 zap logger，统一日志出口
	_ = zap.RedirectStdLog(logger) // 捕获所有通过 stdlib log.Print 等输出的日志
}

func parseLevel(raw string) zapcore.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return zapcore.DebugLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// getFileLogWriter 构造支持滚动切割的文件输出器，当前实现保留作扩展备用。
func getFileLogWriter() (writeSyncer zapcore.WriteSyncer) {
	lumberJackLogger := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    100, // 单个文件最大 100MB
		MaxBackups: 30,  // 最多保留 30 个历史文件
		MaxAge:     7,   // 最多保留 7 天
		Compress:   true,
	}

	return zapcore.AddSync(lumberJackLogger)
}

// getCallerInfoForLog 提取“真正业务调用点”的位置信息，补到每条日志里。
// 这里要拿的不是 zlog.Info 自己的位置，而是调用 zlog.Info 的那一层业务代码。
func getCallerInfoForLog() (callerFields []zap.Field) {
	// runtime.Caller(skip) 的 skip 含义：
	// 0 = 当前函数(getCallerInfoForLog)
	// 1 = zlog.Info / zlog.Error 这类包装函数
	// 2 = 业务代码中真正调用 zlog.xxx 的位置（我们要的就是这一层）
	pc, file, line, ok := runtime.Caller(2)
	if !ok {
		// 极少数情况下无法拿到调用栈（比如运行时信息不可用），直接返回空字段，避免影响主流程。
		return
	}

	// pc 是程序计数器，通过它可以反查函数名（通常是完整包路径+函数名）。
	funcName := runtime.FuncForPC(pc).Name()
	// path.Base 只保留最后一段，避免函数名过长导致日志可读性变差。
	funcName = path.Base(funcName)

	// 统一写入 func/file/line，后续排查时可以直接从日志跳到源码位置。
	callerFields = append(callerFields, zap.String("func", funcName), zap.String("file", file), zap.Int("line", line))
	return
}

// Info 输出 info 级别日志，并自动补齐调用位置信息。
func Info(message string, fields ...zap.Field) {
	callerFields := getCallerInfoForLog()
	fields = append(fields, callerFields...)
	logger.Info(message, fields...)
}

// Warn 输出 warn 级别日志，并自动补齐调用位置信息。
func Warn(message string, fields ...zap.Field) {
	callerFields := getCallerInfoForLog()
	fields = append(fields, callerFields...)
	logger.Warn(message, fields...)
}

// Error 输出 error 级别日志，并自动补齐调用位置信息。
func Error(message string, fields ...zap.Field) {
	callerFields := getCallerInfoForLog()
	fields = append(fields, callerFields...)
	logger.Error(message, fields...)
}

// Fatal 输出 fatal 级别日志，并在记录后终止进程。
func Fatal(message string, fields ...zap.Field) {
	callerFields := getCallerInfoForLog()
	fields = append(fields, callerFields...)
	logger.Fatal(message, fields...)
}

// Debug 输出 debug 级别日志，并自动补齐调用位置信息。
func Debug(message string, fields ...zap.Field) {
	callerFields := getCallerInfoForLog()
	fields = append(fields, callerFields...)
	logger.Debug(message, fields...)
}
