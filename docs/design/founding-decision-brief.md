# `idp-platform-go` 立库决策简报

状态：已确认（2026-08-24）——维护者要求立即立库并书面确认 TMS 与 Parcel 必用；立仓三岔路（原语归属、module path、切片顺序）按维护者授权采纳推荐

日期：2026-08-24

## 目标

参考 Python `idp-platform` 的能力划分，为 Go 产品线建立共享平台能力库。本文不把尚未创建的代码、迁移或测试描述为已实现能力。

## 已确认决定

| 议题 | 结论 |
| --- | --- |
| 立库时机 | 立即（维护者要求；推翻"等真实重复出现"的原建议，代价是提前支付库治理成本） |
| 消费者 | TMS 与 Parcel，维护者书面确认必用（2026-08-24） |
| 原语归属 | 编号原语随能力整体归本库；bento-go 以 [ADR-0011](file:///workspace/idp/idp-bento-go/docs/adr/0011-numbering-primitives-move-to-platform-go.md) 撤回其 `v0.2` 提案 |
| module path | `go.idp.xyz/idp-platform-go`，vanity 解析约定与 bento-go 一致 |
| 切片顺序 | 切片一 numbering；切片二 options（动工前补消费者书面用例） |
| 对框架的身份 | 普通消费者：只导入 bento-go 公共 API，不依赖其 `internal`/`testkit` |
| 兼容约束（承接） | 模板令牌词汇表与 Python `FormatTemplate` 语义兼容；不承诺 API 形状对等或加扰/校验位跨语言输出逐位相等 |
| 用例（承接） | TMS 运输任务编号 `execution.transport_task.reference`，`TT-{yyyy}-{seq:4}`，年重置；Parcel 对账单号 `STMT-{yyyy}{mm}-{seq:5}`，月重置（均于 2026-08-24 书面确认） |
| 凭据边界 | 加扰密钥由消费者持有并传参，本库不管理凭据 |

## 切片一范围（numbering）

进入：

- 编号模板 DSL：`{type}`、日期令牌 `{yyyy}`/`{yy}`/`{q}`/`{mm}`/`{dd}`、时间令牌 `{hh}{mi}{ss}{ms}`、序列 `{seq[:N]}`/`{seq36}`、加扰序列 `{seqx}`/`{seqx36}`、`{rand}`、校验位 `{check}`；重置粒度由最细日期令牌推导（连续 → 年 → 季 → 月 → 日）。纯标准库。
- Feistel FPE 加扰与校验位算法。纯标准库。
- 单据类型注册：代码内 registry（`DocTypeSpec` 等价物），由消费者在装配时注册。
- 序列技术表 + 单条条件 SQL 原子分配（不先读后写）+ `assign`；写路径经由 bento-go `application.Transactor` 的派生 context 与 postgres 执行器参与消费者事务，缺少事务句柄即失败。
- 迁移资产：本库拥有不可变 SQL 资产（只增不改），消费者拥有执行。

暂缓（等真实用例，动工前补简报）：

- 预留/认领（`reserve`/`claim`）、草稿持有 TTL、外部编号强制、DB overlay 单据类型目录。

排除（不做）：

- HTTP API、DTO、管理界面；密钥管理；跨语言编号互认或号段迁移工具。

## 切片二范围（options，`Draft`）

参照 Python 生产实现移植选项/参照数据目录（聚合、作用域继承、变更审计、CQRS、路由工厂的 Go 等价形态待设计）。`UNBOUND`：消费者书面用例、API 形态、与 bento-go repository/eventing 缝的对接方式——动工前出专项简报。

**同日另核查过 S3/对象存储能力：不进本库，也不进框架。** Python 侧 bento/platform 均无此能力；TMS evidence 切片刻意把字节留在外部对象存储、schema 只存 `storage_ref` + 完整性元数据——消费者需要的是自己的窄端口；S3 SDK 薄封装没有可收敛的领域行为。**补充（同日晚）**：组合层面的重复已经真实存在——prism、phi-suite、obs 各自运行独立 MinIO 实例，sagapai/bento compose 各带一份，loms 依赖 boto3。维护者提出以**独立项目**收敛对象存储（统一基座 + 约定 + 多语言薄客户端库），评估已给出、待立项决策；该项目独立于本库，本库不承接。

**2026-08-24 证据核查（options）：动工条件不成立。** TMS 把枚举刻意建模为领域闭集（`ubiquitous-language-gaps.md` 全文即该实践——异常态、风险等级、责任方类型等都落进 CONTEXT.md 的封闭词汇，不进运行时字典）；Parcel 的原因、优先级、责任结论同样是领域概念。两个消费者当前都没有"运行时可配置、作用域继承的选项目录"需求。切片二保持 `UNBOUND`，触发条件不变：任一消费者书面确认真实用例后先出专项简报。Python 侧的需求形态（prism/loms 的租户级下拉字典）不能替代 Go 消费者自己的证据。

## 依赖规则

- 依赖 `go.idp.xyz/idp-bento-go` 公共 API，跟随 TMS/Parcel 锁定的候选版本（当前 `v0.1.0-rc.2`）；开发期本地 `replace` 指向工作区检出，发布时锁定框架正式版本。
- 生产依赖图不得出现框架 `internal`、`testkit`，不得出现产品包。
- 纯算法包（模板/加扰/校验位）只依赖 Go 标准库，可被不需要数据库的消费者单独导入。

## 测试

- 纯算法包：单元测试覆盖令牌集、重置粒度推导、加扰与校验位的确定性输出。
- 序列分配：对真实 PostgreSQL 的集成测试（并发竞争、约束、事务参与），对齐生态"真实端口"惯例。

## 链接

- [bento-go ADR-0010：能力边界](file:///workspace/idp/idp-bento-go/docs/adr/0010-numbering-primitives-platform-boundary.md)
- [bento-go ADR-0011：原语转移](file:///workspace/idp/idp-bento-go/docs/adr/0011-numbering-primitives-move-to-platform-go.md)
- [Python idp-platform](file:///workspace/idp/idp-platform)
