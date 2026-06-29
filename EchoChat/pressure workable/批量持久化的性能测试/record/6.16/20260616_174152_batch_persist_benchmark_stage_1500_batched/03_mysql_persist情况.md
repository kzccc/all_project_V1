# mysql_persist情况

- flush 总次数：`2054`
- flush reason：`{"timer": 2044, "batch_full": 9, "single": 1}`
- worker 分布：`{"0": 2054}`
- 平均 flush batch：`14.606`
- 平均 enqueue queue depth：`2.578`

## mysql_persist 细分

- enqueue_block：`0.001ms`
- worker_queue_wait：`2.122ms`
- batch_collect_wait：`0.007ms`
- sql_exec：`3.058ms`
- flush：`0.0ms`
