import http from 'k6/http';
import ws from 'k6/ws';
import { check } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const holdSeconds = Number(__ENV.HOLD_SECONDS || 60);
const baseUrl = __ENV.BASE_URL || 'http://127.0.0.1:8081';
const wsUrl = __ENV.WS_URL || 'ws://127.0.0.1:8081';
const password = __ENV.PASSWORD || '123456';
const telephoneStart = BigInt(__ENV.TELEPHONE_START || '17620000000');
const userOffset = Number(__ENV.USER_OFFSET || 0);

export const options = {
  thresholds: {
    login_success_rate: ['rate>0.99'],
    ws_upgrade_success_rate: ['rate>0.99'],
    ws_error_rate: ['rate<0.01'],
    ws_early_disconnect_rate: ['rate<0.01'],
  },
};

const loginSuccessRate = new Rate('login_success_rate');
const wsUpgradeSuccessRate = new Rate('ws_upgrade_success_rate');
const wsWelcomeMessageRate = new Rate('ws_welcome_message_rate');
const wsErrorRate = new Rate('ws_error_rate');
const wsEarlyDisconnectRate = new Rate('ws_early_disconnect_rate');

const loginFailureCount = new Counter('login_failure_count');
const wsUpgradeFailureCount = new Counter('ws_upgrade_failure_count');
const wsEarlyDisconnectCount = new Counter('ws_early_disconnect_count');
const wsOpenCount = new Counter('ws_open_count');
const wsCloseCount = new Counter('ws_close_count');

const loginDuration = new Trend('login_duration_ms');
const wsSessionDuration = new Trend('ws_session_duration_ms');

export default function () {
  const accountIndex = userOffset + __VU;
  const telephone = (telephoneStart + BigInt(accountIndex)).toString();

  const loginPayload = JSON.stringify({
    telephone,
    password,
  });

  const loginStart = Date.now();
  const loginRes = http.post(`${baseUrl}/login`, loginPayload, {
    headers: {
      'Content-Type': 'application/json',
    },
    tags: {
      scene: 'ws_online',
      step: 'login',
    },
  });
  loginDuration.add(Date.now() - loginStart);

  let loginBody = null;
  try {
    loginBody = loginRes.json();
  } catch (error) {
    loginBody = null;
  }

  const loginOk =
    loginRes.status === 200 &&
    loginBody &&
    loginBody.code === 200 &&
    loginBody.data &&
    loginBody.data.access_token;

  loginSuccessRate.add(Boolean(loginOk));
  if (!loginOk) {
    loginFailureCount.add(1);
    return;
  }

  const accessToken = loginBody.data.access_token;
  const holdMs = holdSeconds * 1000;
  const connectStart = Date.now();
  let sawWelcome = false;
  let openAt = 0;
  let closeAt = 0;

  const res = ws.connect(
    `${wsUrl}/wss?token=${encodeURIComponent(accessToken)}`,
    {
      tags: {
        scene: 'ws_online',
        step: 'ws_connect',
      },
    },
    function (socket) {
      socket.on('open', function () {
        openAt = Date.now();
        wsOpenCount.add(1);
      });

      socket.on('message', function (message) {
        if (String(message).includes('欢迎来到EchoChat聊天服务器')) {
          sawWelcome = true;
        }
      });

      socket.on('error', function () {
        wsErrorRate.add(true);
      });

      socket.on('close', function (code) {
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
    wsWelcomeMessageRate.add(false);
    return;
  }

  wsWelcomeMessageRate.add(sawWelcome);

  const actualSessionMs = closeAt > openAt && openAt > 0 ? closeAt - openAt : 0;
  const earlyDisconnect = actualSessionMs > 0 && actualSessionMs < holdMs * 0.95;
  wsEarlyDisconnectRate.add(earlyDisconnect);
  if (earlyDisconnect) {
    wsEarlyDisconnectCount.add(1);
  }

  wsErrorRate.add(false);
}
