#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUNTIME_DIR="${ROOT_DIR}/deploy/kafka5/runtime"
DIST_DIR="${RUNTIME_DIR}/dist"
DATA_DIR="/my_storage/echochat/kafka/runtime-data"
KAFKA_VERSION="3.9.2"
SCALA_VERSION="2.13"
ARCHIVE_NAME="kafka_${SCALA_VERSION}-${KAFKA_VERSION}.tgz"
ARCHIVE_URL="https://mirrors.tuna.tsinghua.edu.cn/apache/kafka/${KAFKA_VERSION}/${ARCHIVE_NAME}"
ARCHIVE_PATH="${DIST_DIR}/${ARCHIVE_NAME}"
KAFKA_HOME="${DIST_DIR}/kafka_${SCALA_VERSION}-${KAFKA_VERSION}"
CLUSTER_ID="MkU3OEVBNTcwNTJENDM2Qk"
BROKER_PORTS=(29092 29093 29094 29095 29096)
CONTROLLER_PORTS=(39092 39093 39094 39095 39096)
PIDS_FILE="${RUNTIME_DIR}/pids"

mkdir -p "${DIST_DIR}" "${RUNTIME_DIR}/configs" "${DATA_DIR}" "${RUNTIME_DIR}/logs"
rm -f "${PIDS_FILE}"

if [[ ! -f "${ARCHIVE_PATH}" ]]; then
  curl -L --fail --output "${ARCHIVE_PATH}" "${ARCHIVE_URL}"
fi

if [[ ! -x "${KAFKA_HOME}/bin/kafka-server-start.sh" ]]; then
  tar -xzf "${ARCHIVE_PATH}" -C "${DIST_DIR}"
fi

for idx in 1 2 3 4 5; do
  broker_port="${BROKER_PORTS[$((idx-1))]}"
  controller_port="${CONTROLLER_PORTS[$((idx-1))]}"
  config_path="${RUNTIME_DIR}/configs/kafka${idx}.properties"
  data_dir="${DATA_DIR}/kafka${idx}"
  log_path="${RUNTIME_DIR}/logs/kafka${idx}.log"

  mkdir -p "${data_dir}"

  cat > "${config_path}" <<EOF
process.roles=broker,controller
node.id=${idx}
controller.quorum.voters=1@127.0.0.1:39092,2@127.0.0.1:39093,3@127.0.0.1:39094,4@127.0.0.1:39095,5@127.0.0.1:39096
listeners=PLAINTEXT://127.0.0.1:${broker_port},CONTROLLER://127.0.0.1:${controller_port}
advertised.listeners=PLAINTEXT://127.0.0.1:${broker_port}
listener.security.protocol.map=CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT
inter.broker.listener.name=PLAINTEXT
controller.listener.names=CONTROLLER
log.dirs=${data_dir}
num.partitions=240
default.replication.factor=3
min.insync.replicas=2
offsets.topic.replication.factor=3
transaction.state.log.replication.factor=3
transaction.state.log.min.isr=2
auto.create.topics.enable=false
delete.topic.enable=true
log.retention.hours=24
group.initial.rebalance.delay.ms=0
socket.request.max.bytes=104857600
EOF

  if [[ ! -f "${data_dir}/meta.properties" ]]; then
    "${KAFKA_HOME}/bin/kafka-storage.sh" format -t "${CLUSTER_ID}" -c "${config_path}" >/dev/null
  fi

  if lsof -Pi :"${broker_port}" -sTCP:LISTEN -t >/dev/null 2>&1; then
    continue
  fi

  nohup "${KAFKA_HOME}/bin/kafka-server-start.sh" "${config_path}" > "${log_path}" 2>&1 &
  echo $! >> "${PIDS_FILE}"
done

sleep 10
"${ROOT_DIR}/deploy/kafka5/status_local.sh"
