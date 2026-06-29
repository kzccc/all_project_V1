# mysql_persist情况

- flush 总次数：`3861`
- flush reason：`{"timer": 3841, "single": 20}`
- worker 分布：`{"0": 1936, "1": 1925}`
- 平均 flush batch：`12.432`
- 平均 enqueue queue depth：`4.749`

## mysql_persist 细分

- enqueue_block：`0.0ms`
- worker_queue_wait：`2.569ms`
- batch_collect_wait：`0.005ms`
- sql_exec：`4.374ms`
- flush：`0.0ms`
