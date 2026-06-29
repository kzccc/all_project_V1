# mysql_persist情况

- flush 总次数：`30000`
- flush reason：`{"batch_full": 30000}`
- worker 分布：`{"0": 30000}`
- 平均 flush batch：`1.0`
- 平均 enqueue queue depth：`97.183`

## mysql_persist 细分

- enqueue_block：`0.0ms`
- worker_queue_wait：`88.039ms`
- batch_collect_wait：`本轮未采到`
- sql_exec：`0.887ms`
- flush：`0.0ms`
