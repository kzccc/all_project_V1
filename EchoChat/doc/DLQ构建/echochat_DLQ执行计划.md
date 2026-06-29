# EchoChat DLQ 执行计划

## 当前进度

- [x] 明确 DLQ 文档方案
- [x] 明确自动重放与人工治理边界
- [x] 新增 DLQ 执行计划文档
- [x] 新增 DLQ 数据模型与状态定义
- [x] 把 DLQ 表接入 GORM AutoMigrate
- [x] 新增 DLQ 基础 DAO / Service / Writer
- [x] 新增 DLQ 人工治理请求与响应 DTO
- [x] 新增 DLQ 管理后台 HTTP 接口骨架
- [x] 把 DLQ 管理接口注册到 admin 路由
- [x] 实现 DLQ 列表查询
- [x] 实现 DLQ 详情查询
- [x] 实现 DLQ 操作日志查询
- [x] 实现人工治理状态流转：`claim`
- [x] 实现人工治理状态流转：`reopen`
- [x] 实现人工治理状态流转：`close`
- [x] 实现人工治理备注更新
- [x] 实现 DLQ 操作日志写入
- [x] 实现主链路统一 DLQ 写入入口骨架
- [x] 接入 `deserialize` / `route` 永久故障入 DLQ
- [x] 接入 `mysql_persist` 冲突类入 DLQ 决策
- [x] 接入 `conversation bucket worker panic` 入 DLQ
- [x] 接入 `websocket_dispatch` 临时失败入自动重放型 DLQ
- [x] 接入 `group_member_query` 临时失败入自动重放型 DLQ
- [x] 新增自动重放扫描器骨架
- [x] 新增统一重放入口骨架
- [x] 新增 4 类 replay handler 骨架
- [x] 将自动重放调度器接入服务启动入口
- [x] 将 replay 入口连到 chat 业务函数
- [x] 补充 DLQ 唯一源索引与幂等写入
- [x] 补充 DLQ `stats` 统计接口
- [x] 补充自动重放操作日志
- [x] 补充自动重放完成/转人工状态流转
- [x] 补充后台第一版 DLQ 人工治理窗口
- [x] 补充最小单元测试或编译验证
- [x] 更新执行计划勾选状态

## 说明

这份文件用于持续记录代码落地进度。  
后续每完成一项，都直接在这里打勾，保证下次继续开发时能快速定位当前完成位置。
