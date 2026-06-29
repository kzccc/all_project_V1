# mysql_persist情况

- flush 总次数：`60000`
- flush reason：`{"batch_full": 60000}`
- worker 分布：`{"0": 60000}`
- 平均 flush batch：`1.0`
- 平均 enqueue queue depth：`97.858`

## mysql_persist 细分

- enqueue_block：`0.0ms`
- worker_queue_wait：`79.371ms`
- batch_collect_wait：`本轮未采到`
- sql_exec：`0.799ms`
- flush：`0.0ms`
