# 仓库画像

`flow-*` 技能读这份文件来定位本仓库的文档去处和边界。

## 术语表

路径：`docs/design/domain-language.md`

懒创建——第一个术语敲定时才生成。条目格式：

```md
**术语名**：
一到两句话说明它*是什么*。
_避免_：不推荐使用的近义词
```

跨仓库术语（编号原语、平台能力、技术表）以 bento-go 的 `docs/design/domain-language.md` 为源，本仓库不重复定义。

## ADR

目录：`docs/adr/`

编号连续，文件名 `NNNN-slug.md`，结构参照 `docs/adr/0001-standalone-capability-library.md`（背景/决定/后果/考虑过的替代方案/链接）。写完后在 `docs/adr/README.md` 列表末尾追加一行才算完成。

## 设计文档

`docs/design/` 拥有能力目标、边界、兼容性契约和未决问题。决策简报也放这里。

## 缝

- 纯算法包的缝是其导出面（模板 DSL、加扰、校验位）。
- 数据库能力的缝是它定义的端口；SQL 与迁移资产留在缝的另一侧。
- 对框架的缝是 bento-go 公共 API；不触碰其 `internal`/`testkit`。

## 硬边界

- TMS 与 Parcel 的业务语言、规则、数据和生命周期不得进入本仓库；单据类型、选项组由消费者注册。
- 本库不导入产品包；生产依赖图不得出现框架 `internal`/`testkit`。
- 纯算法包只依赖 Go 标准库。
- 迁移资产只增不改；消费者拥有执行。
- 凭据（加扰密钥等）由消费者持有。
- 未定的选择标成 `Draft` 或 `UNBOUND`。
- 决策简报确认之后再开始能力实现。

## 验证

最小相关：`go test -count=1 ./<包>/`

完整：`gofmt -l . && go vet ./... && IDP_PLATFORM_POSTGRES_DSN=<dsn> go test -count=1 ./...`

集成测试的 DSN 环境变量是 `IDP_PLATFORM_POSTGRES_DSN`；未设置时集成测试跳过而不失败。本工作区的专用测试库：`postgres://platform:platform@127.0.0.1:55499/platform_test?sslmode=disable`（容器 `idp-platform-go-pg`）。
