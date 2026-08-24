// Package postgres 提供 numbering.SequenceAllocator 的 PostgreSQL 实现。
//
// 分配是单条条件 upsert（INSERT … ON CONFLICT DO UPDATE … RETURNING），
// 并发调用在行上串行、永不碰撞。分配经由 bento-go 的严格执行器参与
// 调用方事务：事务外调用直接失败，事务回滚即释放已取的号（已提交的
// 号之间无重复）。
package postgres

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	bentopg "go.idp.xyz/idp-bento-go/postgres"

	"go.idp.xyz/idp-platform-go/numbering"
)

// DefaultSchema 是平台技术表的默认 schema。
const DefaultSchema = "platform"

// ErrInvalidConfig 表示序列存储配置非法。
var ErrInvalidConfig = errors.New("numbering/postgres: invalid config")

// ErrKeyRequired 表示序列键缺少必填维度。
var ErrKeyRequired = errors.New("numbering/postgres: scope and doc type are required")

var schemaPattern = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

// SequenceStore 基于一张序列技术表实现 numbering.SequenceAllocator。
type SequenceStore struct {
	db        *bentopg.DB
	schema    string
	upsertSQL string
}

// Option 配置 SequenceStore。
type Option interface{ sequenceOption() }

type option struct{ schema string }

func (option) sequenceOption() {}

// WithSchema 覆盖默认 schema。
func WithSchema(schema string) Option {
	return option{schema: schema}
}

// NewSequenceStore 构造序列存储。db 必填。
func NewSequenceStore(db *bentopg.DB, options ...Option) (*SequenceStore, error) {
	if db == nil {
		return nil, fmt.Errorf("%w: db is nil", ErrInvalidConfig)
	}
	schema := DefaultSchema
	for index, candidate := range options {
		opt, ok := candidate.(option)
		if !ok {
			return nil, fmt.Errorf("%w: option %d has an unsupported implementation", ErrInvalidConfig, index)
		}
		schema = opt.schema
	}
	if !schemaPattern.MatchString(schema) {
		return nil, fmt.Errorf("%w: schema %q", ErrInvalidConfig, schema)
	}
	return &SequenceStore{
		db:     db,
		schema: schema,
		upsertSQL: fmt.Sprintf(
			`INSERT INTO %s.numbering_sequence AS seq (scope, doc_type, period, next_val)
VALUES ($1, $2, $3, 1)
ON CONFLICT (scope, doc_type, period)
DO UPDATE SET next_val = seq.next_val + 1, updated_at = now()
RETURNING seq.next_val`,
			schema,
		),
	}, nil
}

// Schema 返回序列表所在 schema。
func (s *SequenceStore) Schema() string { return s.schema }

// Next 原子分配下一个值（1 基）。必须在 bento-go Transactor 事务内调用，
// 事务外返回包装了 postgres.ErrTransactionRequired 的错误。
func (s *SequenceStore) Next(ctx context.Context, key numbering.SequenceKey) (int64, error) {
	if key.Scope == "" || key.DocType == "" {
		return 0, ErrKeyRequired
	}
	executor, err := s.db.RequireExecutor(ctx)
	if err != nil {
		return 0, fmt.Errorf("numbering/postgres: %w", err)
	}
	var next int64
	if err := executor.QueryRow(ctx, s.upsertSQL, key.Scope, key.DocType, key.Period).Scan(&next); err != nil {
		return 0, fmt.Errorf("numbering/postgres: allocate: %w", err)
	}
	return next, nil
}

var _ numbering.SequenceAllocator = (*SequenceStore)(nil)
