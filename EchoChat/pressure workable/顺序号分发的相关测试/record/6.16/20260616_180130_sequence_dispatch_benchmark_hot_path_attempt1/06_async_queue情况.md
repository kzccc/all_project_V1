# async_queue情况

- 本轮未启用 fixed-shard partition_async，以下内容为 conversation bucket ready queue 视角。
- avg ready depth per partition：`0.177`
- p95 ready depth per partition：`0.1`
- max ready depth single partition：`10`
- avg ready wait：`0.74ms`
