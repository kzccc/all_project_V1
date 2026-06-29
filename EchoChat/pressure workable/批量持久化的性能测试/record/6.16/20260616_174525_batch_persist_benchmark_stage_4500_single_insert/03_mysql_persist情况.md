# mysql_persist情况

- flush 总次数：`90000`
- flush reason：`{"batch_full": 90000}`
- worker 分布：`{"0": 90000}`
- 平均 flush batch：`1.0`
- 平均 enqueue queue depth：`97.924`

## mysql_persist 细分

- enqueue_block：`0.0ms`
- worker_queue_wait：`78.631ms`
- batch_collect_wait：`本轮未采到`
- sql_exec：`0.791ms`
- flush：`0.0ms`
