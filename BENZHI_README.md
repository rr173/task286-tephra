# BENZHI 评测说明

基于 Go 实现的火山灰层玻璃成分对比 Web 项目，一款后端服务，完成玻璃主量元素观测导入与误差标准化、灰层指纹距离与同源显著性检验、层位可行性约束检查、相关关系裁决与不可变相关快照发布。

## 启动

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./cmd/tephracorr --addr :8080 --db tephra.db
```

## 自检（不启动长驻服务）

```bash
go run ./cmd/tephracorr --smoke-test
```

`--smoke-test` 会真实创建灰层与成分观测、执行误差标准化与指纹比较、裁决相关关系、发布相关快照，关闭并重新打开数据库验证持久化与重启恢复，最后以 0 退出码结束。

## 构建门禁

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test  ./...
go run ./cmd/tephracorr --smoke-test
```

## HTTP API（前缀 /api）

灰层：`POST /api/batches`、`GET /api/batches`、`GET /api/batches/{id}`、`PATCH /api/batches/{id}/status`、`POST /api/batches/{id}/seal`
观测：`POST /api/observations`、`POST /api/observations/batch`、`GET /api/batches/{id}/observations`、`GET /api/observations/{id}`、`POST /api/observations/{id}/standardize`、`POST /api/batches/{id}/standardize`、`POST /api/batches/{id}/screen`、`POST /api/observations/{id}/mark-redeposited`、`.../exclude`、`.../restore`
相关：`POST /api/correlations`、`GET /api/correlations`、`GET /api/correlations/{id}`、`POST /api/correlations/{id}/recompute`、`POST /api/correlations/{id}/adjudicate`、`GET /api/correlations/rank`
快照：`POST /api/snapshots`、`GET /api/snapshots`、`GET /api/snapshots/{id}`、`POST /api/snapshots/{id}/publish`、`POST /api/snapshots/{left}/supersede/{right}`、`GET /api/snapshots/{left}/diff/{right}`
自检：`GET /api/health`、`POST /api/selfcheck`、`GET /api/elements`、`GET /api/audits`

## 持久化

SQLite（modernc.org/sqlite，CGO 无关）。建表：batches、observations、correlation_links、snapshots、snapshot_links、audit_log。观测按内容指纹幂等；发布快照固定标准化版本，关系入快照后不可重算。
