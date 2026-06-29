# async_queue情况

- 本轮未启用 fixed-shard partition_async，以下内容为 conversation bucket ready queue 视角。
- avg ready depth per partition：`0.658`
- p95 ready depth per partition：`3.2`
- max ready depth single partition：`68`
- avg ready wait：`2.747ms`
