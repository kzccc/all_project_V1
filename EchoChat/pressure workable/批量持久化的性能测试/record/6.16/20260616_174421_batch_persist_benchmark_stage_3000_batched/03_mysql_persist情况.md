# mysql_persist情况

- flush 总次数：`1806`
- flush reason：`{"timer": 1734, "batch_full": 72}`
- worker 分布：`{"0": 1806}`
- 平均 flush batch：`33.223`
- 平均 enqueue queue depth：`5.274`

## mysql_persist 细分

- enqueue_block：`0.0ms`
- worker_queue_wait：`3.308ms`
- batch_collect_wait：`0.003ms`
- sql_exec：`3.792ms`
- flush：`0.0ms`
