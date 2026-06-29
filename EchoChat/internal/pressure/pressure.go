package pressure

import (
	"strings"

	"echo_chat_server/internal/config"
)

const benchmarkWSPath = "/bench/wss"

func BenchmarkWSPath() string {
	return benchmarkWSPath
}

func BenchmarkRoutesEnabled() bool {
	return config.GetConfig().PressureTestConfig.EnableBenchmarkRoutes
}

func DisableBenchmarkRequestLog() bool {
	return config.GetConfig().PressureTestConfig.DisableBenchmarkRequestLog
}

func DisableBenchmarkHotPathLog() bool {
	return config.GetConfig().PressureTestConfig.DisableBenchmarkHotPathLog
}

func IsBenchmarkPath(path string) bool {
	path = strings.TrimSpace(path)
	return path == benchmarkWSPath
}

func ShouldSkipRequestLog(path string) bool {
	return IsBenchmarkPath(path) && DisableBenchmarkRequestLog()
}

func ShouldLogHotPath(benchmark bool) bool {
	return !(benchmark && DisableBenchmarkHotPathLog())
}
