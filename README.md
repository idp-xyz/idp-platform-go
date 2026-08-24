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
- **options（切片二）**：选项/参照数据目录，参照 Python 生产实现移植；动工前补消费者书面用例。

## 基线

- module path：`go.idp.xyz/idp-platform-go`（vanity 解析约定与 bento-go 一致）
- Go `1.26.5`；依赖 `go.idp.xyz/idp-bento-go`（开发期本地 `replace`，发布随框架正式版锁定）
- 加扰密钥等凭据由消费者持有，本库只接收参数

## 文档

- [立库决策简报](./docs/design/founding-decision-brief.md)
- [ADR 索引](./docs/adr/README.md)
