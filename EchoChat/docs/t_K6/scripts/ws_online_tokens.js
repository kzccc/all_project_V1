import ws from 'k6/ws';
import { check } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const holdSeconds = Number(__ENV.HOLD_SECONDS || 60);
const pingIntervalSeconds = Number(__ENV.PING_INTERVAL_SECONDS || 30);
const wsUrl = __ENV.WS_URL || 'ws://127.0.0.1:8081';
const wsPath = __ENV.WS_PATH || '/bench/wss';
const tokenFile = __ENV.TOKEN_FILE || './docs/t_K6/baselines/ws_tokens.json';
const userOffset = Number(__ENV.USER_OFFSET || 0);

const tokens = JSON.parse(open(tokenFile));

export const options = {
  thresholds: {
    ws_upgrade_success_rate: ['rate>0.99'],
    ws_error_rate: ['rate<0.01'],
    ws_early_disconnect_rate: ['rate<0.01'],
  },
};

const wsUpgradeSuccessRate = new Rate('ws_upgrade_success_rate');
const wsErrorRate = new Rate('ws_error_rate');
const wsEarlyDisconnectRate = new Rate('ws_early_disconnect_rate');

const wsUpgradeFailureCount = new Counter('ws_upgrade_failure_count');
const wsEarlyDisconnectCount = new Counter('ws_early_disconnect_count');
const wsOpenCount = new Counter('ws_open_count');
const wsCloseCount = new Counter('ws_close_count');

const wsSessionDuration = new Trend('ws_session_duration_ms');

export default function () {
  const accountIndex = userOffset + __VU - 1;
  const tokenRecord = tokens[accountIndex];
  if (!tokenRecord || !tokenRecord.access_token) {
    throw new Error(`missing access token for vu index ${accountIndex}`);
  }

  const holdMs = holdSeconds * 1000;
  const connectStart = Date.now();
  let openAt = 0;
  let closeAt = 0;

  const res = ws.connect(
    `${wsUrl}${wsPath}?token=${encodeURIComponent(tokenRecord.access_token)}`,
    {
      tags: {
        scene: 'ws_online_bench',
        step: 'ws_connect',
      },
    },
    function (socket) {
      socket.on('open', function () {
        openAt = Date.now();
        wsOpenCount.add(1);
        if (pingIntervalSeconds > 0) {
          socket.setInterval(function () {
            socket.ping();
          }, pingIntervalSeconds * 1000);
        }
      });

      socket.on('error', function () {
        wsErrorRate.add(true);
      });

      socket.on('close', function () {
        wsCloseCount.add(1);
        closeAt = Date.now();
        if (openAt > 0) {
          wsSessionDuration.add(closeAt - openAt);
        } else {
          wsSessionDuration.add(closeAt - connectStart);
        }
      });

      socket.setTimeout(function () {
        socket.close();
      }, holdMs);
    }
  );

  const upgradeOk = check(res, {
    'websocket upgrade status is 101': (r) => r && r.status === 101,
  });
  wsUpgradeSuccessRate.add(upgradeOk);

  if (!upgradeOk) {
    wsUpgradeFailureCount.add(1);
    return;
  }

  const actualSessionMs = closeAt > openAt && openAt > 0 ? closeAt - openAt : 0;
  const earlyDisconnect = actualSessionMs > 0 && actualSessionMs < holdMs * 0.95;
  wsEarlyDisconnectRate.add(earlyDisconnect);
  if (earlyDisconnect) {
    wsEarlyDisconnectCount.add(1);
  }

  wsErrorRate.add(false);
}
