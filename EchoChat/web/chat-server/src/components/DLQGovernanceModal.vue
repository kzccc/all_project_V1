<template>
  <div v-if="isVisible" class="dlq-wrap">
    <div class="dlq-toolbar">
      <el-input v-model="filters.message_id" class="toolbar-item" placeholder="消息 ID" clearable />
      <el-select v-model="filters.stage" class="toolbar-item" placeholder="失败阶段" clearable>
        <el-option label="全部阶段" value="" />
        <el-option label="会话顺序号" value="session_seq" />
        <el-option label="MySQL 持久化" value="mysql_persist" />
        <el-option label="WebSocket 分发" value="websocket_dispatch" />
        <el-option label="群成员查询" value="group_member_query" />
        <el-option label="反序列化" value="deserialize" />
        <el-option label="路由" value="route" />
        <el-option label="worker panic" value="conversation_bucket_worker_panic" />
      </el-select>
      <el-select v-model="filters.status" class="toolbar-item" placeholder="自动治理状态" clearable>
        <el-option label="全部状态" value="" />
        <el-option label="待自动重放" value="pending" />
        <el-option label="自动重放中" value="retrying" />
        <el-option label="转人工治理" value="manual" />
        <el-option label="已完成" value="done" />
      </el-select>
      <el-select v-model="filters.manual_status" class="toolbar-item" placeholder="人工状态" clearable>
        <el-option label="全部人工状态" value="" />
        <el-option label="待人工处理" value="open" />
        <el-option label="人工处理中" value="in_progress" />
        <el-option label="已关闭" value="closed" />
      </el-select>
      <el-button type="primary" @click="reloadList">查询</el-button>
      <el-button @click="resetFilters">重置</el-button>
      <el-button @click="loadOverview">刷新</el-button>
    </div>

    <div class="dlq-stats-grid">
      <div class="stats-card">
        <div class="stats-label">总数</div>
        <div class="stats-value">{{ stats.total }}</div>
      </div>
      <div class="stats-card">
        <div class="stats-label">待自动重放</div>
        <div class="stats-value">{{ stats.auto_pending }}</div>
      </div>
      <div class="stats-card">
        <div class="stats-label">自动重放中</div>
        <div class="stats-value">{{ stats.auto_retrying }}</div>
      </div>
      <div class="stats-card">
        <div class="stats-label">人工治理池</div>
        <div class="stats-value">{{ stats.manual_total }}</div>
      </div>
      <div class="stats-card">
        <div class="stats-label">待人工处理</div>
        <div class="stats-value">{{ stats.manual_open }}</div>
      </div>
      <div class="stats-card">
        <div class="stats-label">人工处理中</div>
        <div class="stats-value">{{ stats.manual_in_progress }}</div>
      </div>
      <div class="stats-card">
        <div class="stats-label">人工已关闭</div>
        <div class="stats-value">{{ stats.manual_closed }}</div>
      </div>
      <div class="stats-card">
        <div class="stats-label">自动治理完成</div>
        <div class="stats-value">{{ stats.done_total }}</div>
      </div>
    </div>

    <div class="dlq-stage-summary" v-if="stats.stage_stats && stats.stage_stats.length > 0">
      <div v-for="stageItem in stats.stage_stats" :key="stageItem.stage" class="stage-card">
        <div class="stage-title">{{ stageText(stageItem.stage) }}</div>
        <div class="stage-line">总数：{{ stageItem.total }}</div>
        <div class="stage-line">待重放：{{ stageItem.pending }}</div>
        <div class="stage-line">转人工：{{ stageItem.manual }}</div>
      </div>
    </div>

    <div class="dlq-content">
      <div class="dlq-table-panel">
        <el-table
          :data="list"
          style="width: 100%; height: 100%"
          highlight-current-row
          @row-click="selectRow"
        >
          <el-table-column prop="id" label="ID" width="80" />
          <el-table-column prop="stage" label="阶段" width="150">
            <template #default="scope">
              <span>{{ stageText(scope.row.stage) }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="message_id" label="消息 ID" min-width="180" show-overflow-tooltip />
          <el-table-column prop="error_code" label="错误码" width="180" show-overflow-tooltip />
          <el-table-column prop="status" label="自动状态" width="120">
            <template #default="scope">
              <span>{{ statusText(scope.row.status) }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="manual_status" label="人工状态" width="120">
            <template #default="scope">
              <span>{{ manualStatusText(scope.row) }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="attempt_count" label="已重放" width="90" />
          <el-table-column prop="next_retry_at" label="下次时间" width="170">
            <template #default="scope">
              <span>{{ formatTime(scope.row.next_retry_at) }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="updated_at" label="更新时间" width="170">
            <template #default="scope">
              <span>{{ formatTime(scope.row.updated_at) }}</span>
            </template>
          </el-table-column>
        </el-table>
        <div class="pagination-wrap">
          <el-pagination
            background
            layout="total, sizes, prev, pager, next"
            :total="total"
            :page-size="pagination.page_size"
            :current-page="pagination.page"
            :page-sizes="[10, 20, 50, 100]"
            @current-change="handlePageChange"
            @size-change="handleSizeChange"
          />
        </div>
      </div>

      <div class="dlq-detail-panel">
        <div v-if="detail.id">
          <div class="detail-title">治理详情</div>
          <div class="detail-line"><strong>ID：</strong>{{ detail.id }}</div>
          <div class="detail-line"><strong>阶段：</strong>{{ stageText(detail.stage) }}</div>
          <div class="detail-line"><strong>消息 ID：</strong>{{ detail.message_id }}</div>
          <div class="detail-line"><strong>会话：</strong>{{ detail.conversation_key || "-" }}</div>
          <div class="detail-line"><strong>错误码：</strong>{{ detail.error_code }}</div>
          <div class="detail-line"><strong>最后错误：</strong>{{ detail.last_error || "-" }}</div>
          <div class="detail-line"><strong>自动状态：</strong>{{ statusText(detail.status) }}</div>
          <div class="detail-line"><strong>人工状态：</strong>{{ manualStatusText(detail) }}</div>
          <div class="detail-line"><strong>处理人：</strong>{{ detail.assignee || "-" }}</div>
          <div class="detail-line"><strong>关闭原因：</strong>{{ closeReasonText(detail.close_reason) }}</div>
          <div class="detail-line"><strong>已重放次数：</strong>{{ detail.attempt_count }} / {{ detail.max_attempt_count }}</div>
          <div class="detail-line"><strong>下次重放：</strong>{{ formatTime(detail.next_retry_at) }}</div>

          <div class="action-row">
            <el-button type="primary" :disabled="!canClaim(detail)" @click="claimRecord">接手处理</el-button>
            <el-button :disabled="!canReopen(detail)" @click="reopenRecord">重新打开</el-button>
            <el-button type="danger" :disabled="!canClose(detail)" @click="openCloseDialog">关闭记录</el-button>
          </div>

          <div class="remark-block">
            <div class="detail-subtitle">人工备注</div>
            <el-input
              v-model="remarkDraft"
              type="textarea"
              :rows="4"
              placeholder="记录排查结论、外部修复单号、事故单号等"
            />
            <div class="remark-actions">
              <el-button type="primary" @click="saveRemark">保存备注</el-button>
            </div>
          </div>

          <div class="payload-block">
            <div class="detail-subtitle">原始消息体</div>
            <pre class="payload-pre">{{ detail.raw_payload || "-" }}</pre>
          </div>
          <div class="payload-block">
            <div class="detail-subtitle">业务快照</div>
            <pre class="payload-pre">{{ detail.payload_snapshot || "-" }}</pre>
          </div>
          <div class="payload-block">
            <div class="detail-subtitle">上下文快照</div>
            <pre class="payload-pre">{{ detail.context_snapshot || "-" }}</pre>
          </div>
          <div class="payload-block">
            <div class="detail-subtitle">操作日志</div>
            <div v-if="logs.length === 0" class="empty-text">暂无日志</div>
            <div v-for="log in logs" :key="log.id" class="log-item">
              <div>{{ formatTime(log.created_at) }} / {{ actionText(log.action) }}</div>
              <div>操作人：{{ log.operator || "-" }}</div>
              <div>状态：{{ log.before_manual_status || "-" }} -> {{ log.after_manual_status || "-" }}</div>
              <div>备注：{{ log.remark || "-" }}</div>
            </div>
          </div>
        </div>

        <div v-else class="empty-detail">
          请选择一条 DLQ 记录查看详情。
        </div>
      </div>
    </div>

    <el-dialog v-model="closeDialogVisible" title="关闭 DLQ 记录" width="480px">
      <el-select v-model="closeForm.close_reason" style="width: 100%" placeholder="请选择关闭原因">
        <el-option label="直接放弃" value="discarded" />
        <el-option label="外部已修复" value="externally_fixed" />
        <el-option label="属于预期" value="expected" />
        <el-option label="并入事故单" value="merged_into_incident" />
      </el-select>
      <el-input
        v-model="closeForm.remark"
        style="margin-top: 16px"
        type="textarea"
        :rows="4"
        placeholder="填写关闭结论"
      />
      <template #footer>
        <el-button @click="closeDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="closeRecord">确认关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import { reactive, toRefs, watch } from "vue";
import { useStore } from "vuex";
import axios from "axios";
import { ElMessage } from "element-plus";

export default {
  name: "DLQGovernanceModal",
  props: {
    isVisible: false,
  },
  setup(props) {
    const store = useStore();
    const data = reactive({
      list: [],
      total: 0,
      detail: {},
      logs: [],
      stats: {
        total: 0,
        auto_pending: 0,
        auto_retrying: 0,
        manual_total: 0,
        done_total: 0,
        manual_open: 0,
        manual_in_progress: 0,
        manual_closed: 0,
        stage_stats: [],
      },
      filters: {
        message_id: "",
        stage: "",
        status: "",
        manual_status: "",
      },
      pagination: {
        page: 1,
        page_size: 20,
      },
      remarkDraft: "",
      closeDialogVisible: false,
      closeForm: {
        close_reason: "",
        remark: "",
      },
    });

    const request = async (path, payload) => {
      const rsp = await axios.post(store.state.backendUrl + path, payload || {});
      if (rsp.data.code !== 200) {
        throw new Error(rsp.data.message || "请求失败");
      }
      return rsp.data.data;
    };

    const loadStats = async () => {
      const rsp = await request("/dlq/stats", {});
      data.stats = rsp || data.stats;
    };

    const loadList = async () => {
      const rsp = await request("/dlq/list", {
        message_id: data.filters.message_id,
        stage: data.filters.stage,
        status: data.filters.status,
        manual_status: data.filters.manual_status,
        page: data.pagination.page,
        page_size: data.pagination.page_size,
      });
      data.list = rsp.list || [];
      data.total = rsp.total || 0;
      if (data.detail.id) {
        const matched = data.list.find((item) => item.id === data.detail.id);
        if (matched) {
          await loadDetail(matched.id);
        }
      }
    };

    const loadDetail = async (id) => {
      const detail = await request("/dlq/detail", { id });
      const logs = await request("/dlq/logs", { id });
      data.detail = detail || {};
      data.logs = logs || [];
      data.remarkDraft = data.detail.remark || "";
    };

    const loadOverview = async () => {
      try {
        await loadStats();
        await loadList();
      } catch (error) {
        ElMessage.error(error.message || "加载 DLQ 失败");
      }
    };

    const reloadList = async () => {
      data.pagination.page = 1;
      await loadOverview();
    };

    const resetFilters = async () => {
      data.filters.message_id = "";
      data.filters.stage = "";
      data.filters.status = "";
      data.filters.manual_status = "";
      data.pagination.page = 1;
      await loadOverview();
    };

    const selectRow = async (row) => {
      if (!row || !row.id) {
        return;
      }
      try {
        await loadDetail(row.id);
      } catch (error) {
        ElMessage.error(error.message || "加载详情失败");
      }
    };

    const claimRecord = async () => {
      try {
        await request("/dlq/claim", { id: data.detail.id });
        ElMessage.success("已接手");
        await loadOverview();
        await loadDetail(data.detail.id);
      } catch (error) {
        ElMessage.error(error.message || "接手失败");
      }
    };

    const reopenRecord = async () => {
      try {
        await request("/dlq/reopen", { id: data.detail.id });
        ElMessage.success("已重新打开");
        await loadOverview();
        await loadDetail(data.detail.id);
      } catch (error) {
        ElMessage.error(error.message || "重新打开失败");
      }
    };

    const openCloseDialog = () => {
      data.closeForm.close_reason = "";
      data.closeForm.remark = data.detail.remark || "";
      data.closeDialogVisible = true;
    };

    const closeRecord = async () => {
      try {
        await request("/dlq/close", {
          id: data.detail.id,
          close_reason: data.closeForm.close_reason,
          remark: data.closeForm.remark,
        });
        data.closeDialogVisible = false;
        ElMessage.success("已关闭");
        await loadOverview();
        await loadDetail(data.detail.id);
      } catch (error) {
        ElMessage.error(error.message || "关闭失败");
      }
    };

    const saveRemark = async () => {
      try {
        await request("/dlq/remark", {
          id: data.detail.id,
          remark: data.remarkDraft,
        });
        ElMessage.success("备注已保存");
        await loadDetail(data.detail.id);
      } catch (error) {
        ElMessage.error(error.message || "保存备注失败");
      }
    };

    const handlePageChange = async (page) => {
      data.pagination.page = page;
      await loadList();
    };

    const handleSizeChange = async (pageSize) => {
      data.pagination.page = 1;
      data.pagination.page_size = pageSize;
      await loadList();
    };

    const formatTime = (value) => {
      if (!value) {
        return "-";
      }
      return String(value).replace("T", " ").replace("Z", "");
    };

    const stageText = (value) => {
      switch (value) {
        case "session_seq":
          return "会话顺序号";
        case "mysql_persist":
          return "MySQL 持久化";
        case "websocket_dispatch":
          return "WebSocket 分发";
        case "group_member_query":
          return "群成员查询";
        case "deserialize":
          return "反序列化";
        case "route":
          return "路由";
        case "conversation_bucket_worker_panic":
          return "worker panic";
        default:
          return value || "-";
      }
    };

    const statusText = (value) => {
      switch (value) {
        case "pending":
          return "待自动重放";
        case "retrying":
          return "自动重放中";
        case "manual":
          return "转人工治理";
        case "done":
          return "已完成";
        default:
          return value || "-";
      }
    };

    const manualStatusText = (row) => {
      if (!row || row.status !== "manual") {
        return "-";
      }
      switch (row.manual_status) {
        case "open":
          return "待人工处理";
        case "in_progress":
          return "人工处理中";
        case "closed":
          return "已关闭";
        default:
          return row.manual_status || "-";
      }
    };

    const closeReasonText = (value) => {
      switch (value) {
        case "discarded":
          return "直接放弃";
        case "externally_fixed":
          return "外部已修复";
        case "expected":
          return "属于预期";
        case "merged_into_incident":
          return "并入事故单";
        default:
          return value || "-";
      }
    };

    const actionText = (value) => {
      switch (value) {
        case "create":
          return "创建";
        case "auto_retry":
          return "自动重放";
        case "claim":
          return "接手";
        case "reopen":
          return "重新打开";
        case "close":
          return "关闭";
        case "remark":
          return "备注";
        case "done":
          return "自动完成";
        default:
          return value || "-";
      }
    };

    const canClaim = (row) => row && row.id && row.status === "manual" && row.manual_status !== "closed";
    const canReopen = (row) => row && row.id && row.status === "manual" && row.manual_status === "closed";
    const canClose = (row) => row && row.id && row.status === "manual" && row.manual_status !== "closed";

    watch(
      () => props.isVisible,
      async (visible) => {
        if (!visible) {
          return;
        }
        await loadOverview();
      },
      { immediate: true }
    );

    return {
      ...toRefs(data),
      loadOverview,
      reloadList,
      resetFilters,
      selectRow,
      claimRecord,
      reopenRecord,
      openCloseDialog,
      closeRecord,
      saveRemark,
      handlePageChange,
      handleSizeChange,
      formatTime,
      stageText,
      statusText,
      manualStatusText,
      closeReasonText,
      actionText,
      canClaim,
      canReopen,
      canClose,
    };
  },
};
</script>

<style scoped>
.dlq-wrap {
  height: 100%;
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.dlq-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
}

.toolbar-item {
  width: 180px;
}

.dlq-stats-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.stats-card {
  border: 1px solid #f0d3d3;
  border-radius: 12px;
  background: #fff7f7;
  padding: 12px 14px;
}

.stats-label {
  font-size: 13px;
  color: #666;
}

.stats-value {
  margin-top: 6px;
  font-size: 24px;
  font-weight: 700;
  color: #b44a4a;
}

.dlq-stage-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.stage-card {
  border: 1px solid #ececec;
  border-radius: 12px;
  background: #ffffff;
  padding: 12px 14px;
}

.stage-title {
  font-weight: 700;
  color: #333;
}

.stage-line {
  margin-top: 6px;
  color: #666;
  font-size: 13px;
}

.dlq-content {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: minmax(0, 1.3fr) minmax(320px, 0.9fr);
  gap: 12px;
}

.dlq-table-panel,
.dlq-detail-panel {
  min-height: 0;
  border: 1px solid #f0d3d3;
  border-radius: 14px;
  background: #fff;
  padding: 12px;
}

.pagination-wrap {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}

.dlq-detail-panel {
  overflow-y: auto;
}

.detail-title {
  font-size: 18px;
  font-weight: 700;
  margin-bottom: 12px;
}

.detail-subtitle {
  font-size: 14px;
  font-weight: 700;
  margin-bottom: 8px;
}

.detail-line {
  margin-bottom: 8px;
  color: #444;
  word-break: break-all;
}

.action-row {
  display: flex;
  gap: 12px;
  margin: 16px 0;
}

.remark-block,
.payload-block {
  margin-top: 16px;
}

.remark-actions {
  margin-top: 10px;
  display: flex;
  justify-content: flex-end;
}

.payload-pre {
  background: #fff7f7;
  border: 1px solid #f0d3d3;
  border-radius: 10px;
  padding: 10px;
  white-space: pre-wrap;
  word-break: break-all;
  font-size: 12px;
  line-height: 1.5;
}

.log-item {
  border: 1px solid #f1f1f1;
  border-radius: 10px;
  padding: 10px;
  margin-bottom: 10px;
  font-size: 13px;
  color: #444;
}

.empty-detail,
.empty-text {
  color: #888;
  padding: 24px 0;
}

@media (max-width: 1280px) {
  .dlq-stats-grid,
  .dlq-stage-summary,
  .dlq-content {
    grid-template-columns: 1fr;
  }
}
</style>
