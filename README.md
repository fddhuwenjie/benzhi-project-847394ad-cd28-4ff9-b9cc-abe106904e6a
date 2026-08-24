# 受限空间作业许可闭环服务

本项目面向厂区作业负责人、现场安全审核员和退场核验员，提供版本化 JSON HTTP API。服务将一张受限空间作业许可从草拟、提交校验、审核整改、批准激活推进到退场证据核验和关闭，并持久保存 revision、审核轮次、批准快照、退场证据及状态时间线。

## 构建与运行

构建服务：

```bash
go build ./cmd/server
```

使用默认地址 `127.0.0.1:19081` 和默认快照路径 `data/permits.json` 运行：

```bash
go run ./cmd/server
```

可通过参数指定回环监听地址和数据路径：

```bash
go run ./cmd/server -addr=127.0.0.1:19181 -data=data/permits.json
```

也可以设置仅含端口号的 `PORT`。服务会将它解析为 `127.0.0.1:<PORT>`；显式 `-addr` 的优先级更高。服务不会默认监听 `0.0.0.0`，也不会使用 `80`、`3000` 或 `8080` 作为默认端口。

## 自检与测试

完整 HTTP 闭环自检会启动临时服务和临时数据快照，经同一公开 API 执行两轮审核、激活、退场和关闭，然后主动退出：

```bash
go run ./cmd/server -self-check -addr=127.0.0.1:19081
```

运行全部回归测试：

```bash
go test ./...
```

## API 约定

所有写请求使用 `Content-Type: application/json`，请求体上限为 1 MiB，并严格拒绝未知字段。写请求必须包含符合技术标识格式的 `actor_id` 和 `request_id`；除创建外还必须包含大于零的 `expected_revision`。同一动作中相同 `request_id` 和载荷会重放原结果，不同载荷复用该标识会返回 `409 Conflict`。

响应统一使用 `{"data": ...}`；错误使用 `{"error":{"code":"...","message":"...","request_id":"...","issues":[]}}`。完整性或安全规则失败返回 `422 Unprocessable Entity`，状态、revision 或幂等冲突返回 `409 Conflict`，资源不存在返回 `404 Not Found`。

主要路由：

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `POST` | `/api/v1/permits` | 创建完整许可草稿 |
| `GET` | `/api/v1/permits/{permit_id}` | 查询许可、审核轮次和退场证据 |
| `PATCH` | `/api/v1/permits/{permit_id}` | 按 revision 修订草稿或整改内容 |
| `POST` | `/api/v1/permits/{permit_id}/submit` | 校验并提交或重新提交审核 |
| `GET` | `/api/v1/permits/{permit_id}/preflight` | 只读执行提交前完整性、安全预检 |
| `POST` | `/api/v1/permits/{permit_id}/reviews/assign` | 分派新的审核轮次 |
| `POST` | `/api/v1/permits/{permit_id}/reviews/decision` | 退回整改或批准许可 |
| `POST` | `/api/v1/permits/{permit_id}/reviews/responses` | 逐项记录整改回应 |
| `POST` | `/api/v1/permits/{permit_id}/activate` | 现场复核后在有效计划时段内激活许可 |
| `POST` | `/api/v1/permits/{permit_id}/closure` | 提交人员、工具、隔离和照片证据 |
| `POST` | `/api/v1/permits/{permit_id}/closure/verify` | 通过或退回退场证据，核验通过后关闭许可 |
| `GET` | `/api/v1/permits` | 按状态、空间、计划范围或审核员查询分页工作队列 |
| `GET` | `/api/v1/permits/{permit_id}/timeline` | 按操作者、请求、状态和时间筛选统一审计时间线 |

## 数据安全

本地 JSON 快照由 `internal/storage/jsonstore` 管理。每次写入先取得许可粒度锁并校验 revision，随后写入同目录临时文件、同步文件内容、原子替换目标快照并同步目录。服务正常关闭时会再次刷新快照。生产部署应把 `-data` 指向具有持久化和访问控制的数据目录。
