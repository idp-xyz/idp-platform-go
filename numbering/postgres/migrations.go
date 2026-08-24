package postgres

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed migrations/0001_numbering_sequence.sql
var migration0001 string

// Migration 是一份可执行的迁移资产。已发布迁移只增不改；修正错误必须
// 新增迁移，不能重写历史。
type Migration struct {
	// ID 是稳定标识，消费者迁移历史以它为准。
	ID string
	// SQL 是按 schema 渲染后的语句。
	SQL string
}

// Migrations 按 schema 渲染本包全部迁移资产（按依赖顺序）。消费者用
// 自己的迁移工具执行；应用启动不得自动执行。
func Migrations(schema string) ([]Migration, error) {
	if !schemaPattern.MatchString(schema) {
		return nil, fmt.Errorf("%w: schema %q", ErrInvalidConfig, schema)
	}
	return []Migration{
		{
			ID:  "platform_numbering_0001_numbering_sequence",
			SQL: strings.ReplaceAll(migration0001, "{{schema}}", schema),
		},
	}, nil
}
