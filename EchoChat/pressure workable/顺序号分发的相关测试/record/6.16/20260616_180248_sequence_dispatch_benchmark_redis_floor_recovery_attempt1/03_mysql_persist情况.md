# mysql_persist情况

- flush 总次数：`3973`
- flush reason：`{"timer": 3958, "single": 15}`
- worker 分布：`{"0": 1989, "1": 1984}`
- 平均 flush batch：`12.082`
- 平均 enqueue queue depth：`4.197`

## mysql_persist 细分

- enqueue_block：`0.0ms`
- worker_queue_wait：`2.351ms`
- batch_collect_wait：`0.006ms`
- sql_exec：`3.232ms`
- flush：`0.0ms`
