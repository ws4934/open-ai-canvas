# 程序文件更新设计

织影从 `ws4934/open-ai-canvas` 的 GitHub Release 分发完整 Go 程序：Actions 编译前端、Agent 依赖和官方插件并嵌入程序，网页下载、校验、替换、重启。详细使用及接口见[网页更新与程序回退](../content/docs/backend/system-update.mdx)。

## 已确定的实现

- Go 直接提供页面、API、资源与 SSE；外部反代独立管理。
- Agent 使用 Go 管理的本地 Node 子进程，不再远程调用独立 Agent。
- Docker 只运行统一应用、PostgreSQL 和 Redis。
- 持久化程序作为唯一运行入口，容器重建不覆盖网页升级结果。
- 程序资源按构建身份释放，回退程序同时切换对应 Agent 与官方插件。
- 排空前只禁止新工作，允许已有 Agent 派生任务继续完成；任务未结束不强杀。
- 程序回退检查当前数据库兼容性，不自动恢复整库。
- 发布只在质量检查、架构构建与镜像验收完成后公开 Release。
