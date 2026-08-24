// Package numbering 提供业务无关的单据编号原语：编号模板 DSL、Feistel
// 格式保持加扰、校验位，以及把它们与原子序列分配编排起来的 Assigner。
//
// 本包只依赖 Go 标准库。序列分配通过 SequenceAllocator 端口注入，
// PostgreSQL 实现见子包 numbering/postgres。
//
// 模板令牌词汇表与 Python bento FormatTemplate 语义兼容（模板字符串
// 可跨生态迁移）；不承诺 API 形状对等，也不承诺加扰或校验位跨语言
// 输出逐位相等。
package numbering
