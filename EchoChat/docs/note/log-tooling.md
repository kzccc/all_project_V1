# 日志查看工具套件（VSCode + 终端）

## 1. VSCode 插件（已写入工作区推荐）

已在 `.vscode/extensions.json` 推荐：

1. `hojabri.slog-viewer`
2. `boria8.logexpert-log-viewer`
3. `genxs.genxslogfilehighlighter`

打开仓库后，VSCode 会提示安装推荐插件。

## 2. VSCode 一键任务

已在 `.vscode/tasks.json` 预置以下任务（`Ctrl+Shift+P -> Tasks: Run Task`）：

1. `Logs: Tail (color)`：彩色实时跟踪日志
2. `Logs: By actor_id`：按用户筛选
3. `Logs: By request_id`：按请求链路筛选
4. `Logs: Error only`：只看错误
5. `Logs: Keyword grep`：关键字检索
6. `Logs: Open with lnav`：用 lnav 打开（若已安装）

日志默认路径：

`/workspace/czk/Personal/EchoChat/logs/echochat.log`

## 3. 终端常用命令

```bash
# 按用户看日志
jq -c 'select(.actor_id=="17603055719")' /workspace/czk/Personal/EchoChat/logs/echochat.log | jq -C '.'

# 按 request_id 看完整链路
jq -c 'select(.request_id=="login-test-001")' /workspace/czk/Personal/EchoChat/logs/echochat.log | jq -C '.'

# 仅看错误
rg --color=always -n '"level":"error"|"level":"fatal"|panic' /workspace/czk/Personal/EchoChat/logs/echochat.log

# 实时跟踪 + JSON 彩色化
tail -n 200 -f /workspace/czk/Personal/EchoChat/logs/echochat.log | jq -R 'fromjson? // {raw:.}' -C
```

## 4. 建议

为了“所有请求日志都能看到是谁发起”，前端/调用方统一带：

- `X-Request-ID`
- `X-Actor-ID`

这样 `http.request.start` 也会稳定出现 `actor_id`。
