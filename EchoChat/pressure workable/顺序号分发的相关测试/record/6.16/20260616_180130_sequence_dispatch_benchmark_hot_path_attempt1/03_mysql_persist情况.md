# mysql_persist情况

- flush 总次数：`3968`
- flush reason：`{"single": 13, "timer": 3955}`
- worker 分布：`{"0": 1982, "1": 1986}`
- 平均 flush batch：`12.097`
- 平均 enqueue queue depth：`4.007`

## mysql_persist 细分

- enqueue_block：`0.0ms`
- worker_queue_wait：`2.368ms`
- batch_collect_wait：`0.005ms`
- sql_exec：`3.31ms`
- flush：`0.0ms`
