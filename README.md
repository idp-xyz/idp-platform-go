# idp-platform-go

状态：切片一（numbering）已实现，单元测试与真实 PostgreSQL 集成测试通过；切片二（options）未动工

跨应用复用、与具体产品业务无关的**平台能力库**，服务 IDP 的 Go 产品线。参考 Python [`idp-platform`](file:///workspace/idp/idp-platform) 的能力划分，构建在 [`go.idp.xyz/idp-bento-go`](file:///workspace/idp/idp-bento-go) 框架之上，由 IDP-TMS 与 IDP-Parcel 消费（维护者已确认双消费者必用）。

## 分层

```
go.idp.xyz/idp-bento-go   框架 — domain/repository/eventing/application/testkit + postgres 适配器
        ▲
go.idp.xyz/idp-platform-go 平台能力 — numbering（切片一）、options（切片二）
        ▲
idp-tms / idp-parcel       产品 — 注册单据类型/选项组、拥有数据与迁移执行、自己的 API
```

本库从框架视角是普通消费者：只导入公共 API，不获得特殊接口（bento-go ADR-0011）。

## 治理模型

**库拥有代码，应用拥有数据**：本库拥有能力的行为（算法、领域规则、端口）和自己技术表的不可变迁移资产；每个消费者拥有表的迁移执行、权限、历史和数据本身。

## 能力

- **numbering（切片一）**：编号模板 DSL（令牌词汇表与 Python `FormatTemplate` 语义兼容）、重置粒度推导、Feistel FPE 加扰、校验位、序列技术表与原子分配、单据类型注册与 `assign`。已确认用例：TMS 运输任务编号 `TT-{yyyy}-{seq:4}`（年重置）、Parcel 对账单号 `STMT-{yyyy}{mm}-{seq:5}`（月重置）。暂缓：预留/认领、草稿持有 TTL、DB overlay 目录。
- **options（切片二）**：选项/参照数据目录，参照 Python 生产实现移植。2026-08-24 证据核查：两个消费者都把枚举建模为领域闭集，暂无运行时目录需求——保持 `UNBOUND` 不动工，任一消费者书面确认用例后先出专项简报（见[立库简报](./docs/design/founding-decision-brief.md)）。

## 接入（numbering）

```go
import (
    "go.idp.xyz/idp-bento-go/application"
    bentopg "go.idp.xyz/idp-bento-go/postgres"
    "go.idp.xyz/idp-platform-go/numbering"
    numberingpg "go.idp.xyz/idp-platform-go/numbering/postgres"
)

// 1) 迁移：用消费者自己的迁移工具执行本库的不可变资产（启动时不得自动执行）。
migrations, _ := numberingpg.Migrations(numberingpg.DefaultSchema)

// 2) 装配：注册自己的单据类型，构造序列存储与取号入口。
db, _ := bentopg.NewDB(pool)
store, _ := numberingpg.NewSequenceStore(db)
registry, _ := numbering.NewRegistry(map[string]numbering.DocTypeSpec{
    "TRANSPORT_TASK": {Prefix: "TT", Template: "TT-{yyyy}-{seq:4}"},
})
assigner, _ := numbering.NewAssigner(registry, store,
    numbering.WithScrambleSecret(secret)) // 仅 {seqx} 系列需要；密钥归消费者

// 3) 事务内取号：分配与业务写入同生共死，回滚即释放号码。
number, err := application.Transactional(ctx, db.Transactor(),
    func(txCtx context.Context) (string, error) {
        n, err := assigner.Assign(txCtx, "tenant-1", "TRANSPORT_TASK")
        if err != nil {
            return "", err
        }
        // …在同一 txCtx 内写业务行，把 n 存进单据字段…
        return n, err
    })
```

scope 由调用方显式传入（通常是权威租户标识），本库不从 context 推断范围。

## 基线

- module path：`go.idp.xyz/idp-platform-go`（vanity 解析约定与 bento-go 一致）
- Go `1.26.5`；依赖 `go.idp.xyz/idp-bento-go`（开发期本地 `replace`，发布随框架正式版锁定）
- 加扰密钥等凭据由消费者持有，本库只接收参数

## 文档

- [立库决策简报](./docs/design/founding-decision-brief.md)
- [ADR 索引](./docs/adr/README.md)
