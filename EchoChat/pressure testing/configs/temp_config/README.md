# temp_config

一次性实验配置统一放在这个目录。

约束：

1. 默认基线配置只保留稳定验证过的口径。
2. 高风险实验口径，例如 `500/1000 会话`、`batch=200/250`、`50 worker/partition`，统一复制到这里单独保存。
3. 不要把临时实验值直接覆盖 `single_chat_pressure.toml`。
