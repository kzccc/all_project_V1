# mysql_persist情况

- flush 总次数：`1611`
- flush reason：`{"timer": 1405, "batch_full": 206}`
- worker 分布：`{"0": 1611}`
- 平均 flush batch：`55.866`
- 平均 enqueue queue depth：`10.835`

## mysql_persist 细分

- enqueue_block：`0.0ms`
- worker_queue_wait：`4.849ms`
- batch_collect_wait：`0.005ms`
- sql_exec：`4.773ms`
- flush：`0.0ms`
