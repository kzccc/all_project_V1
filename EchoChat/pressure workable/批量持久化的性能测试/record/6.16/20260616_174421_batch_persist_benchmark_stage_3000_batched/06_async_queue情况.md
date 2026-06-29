# async_queue情况

- 本轮未启用 fixed-shard partition_async，以下内容为 conversation bucket ready queue 视角。
- avg ready depth per partition：`0.2`
- p95 ready depth per partition：`0.0`
- max ready depth single partition：`26`
- avg ready wait：`0.664ms`
