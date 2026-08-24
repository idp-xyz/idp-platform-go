package numbering

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// SequenceKey 标识一条序列。Scope 由消费者显式提供（通常含权威租户
// 标识，可含更细维度），本包不从 context 推断任何范围。
type SequenceKey struct {
	Scope   string
	DocType string
	// Period 是重置周期键；连续序列为空串。
	Period string
}

// SequenceAllocator 为键原子分配下一个单调值（1 基）。实现必须并发
// 安全；PostgreSQL 实现见子包 numbering/postgres。
type SequenceAllocator interface {
	Next(ctx context.Context, key SequenceKey) (int64, error)
}

var (
	// ErrUnknownDocType 表示单据类型未注册。
	ErrUnknownDocType = errors.New("numbering: unknown doc type")
	// ErrScopeRequired 表示调用方没有提供显式范围。
	ErrScopeRequired = errors.New("numbering: scope is required")
)

// Assigner 把注册表、序列分配和模板渲染编排为取号入口。
type Assigner struct {
	registry  *Registry
	sequences SequenceAllocator
	now       func() time.Time
	secret    string
}

// AssignerOption 配置 Assigner。
type AssignerOption interface{ assignerOption() }

type assignerOption struct {
	now    func() time.Time
	secret *string
}

func (assignerOption) assignerOption() {}

// WithNow 覆盖时间来源（默认 UTC 当前时刻）。
func WithNow(now func() time.Time) AssignerOption {
	return assignerOption{now: now}
}

// WithScrambleSecret 设置 {seqx}/{seqx36} 的加扰密钥。密钥由消费者
// 持有并注入，本库不管理凭据。
func WithScrambleSecret(secret string) AssignerOption {
	return assignerOption{secret: &secret}
}

// NewAssigner 构造取号入口。registry 与 sequences 必填。
func NewAssigner(
	registry *Registry,
	sequences SequenceAllocator,
	options ...AssignerOption,
) (*Assigner, error) {
	if registry == nil {
		return nil, errors.New("numbering: registry is nil")
	}
	if sequences == nil {
		return nil, errors.New("numbering: sequence allocator is nil")
	}
	a := &Assigner{
		registry:  registry,
		sequences: sequences,
		now:       func() time.Time { return time.Now().UTC() },
	}
	for index, candidate := range options {
		opt, ok := candidate.(assignerOption)
		if !ok {
			return nil, fmt.Errorf("numbering: option %d has an unsupported implementation", index)
		}
		if opt.now != nil {
			a.now = opt.now
		}
		if opt.secret != nil {
			a.secret = *opt.secret
		}
	}
	return a, nil
}

// Assign 为 scope 内的单据类型取下一个号。含序列令牌的模板会经
// SequenceAllocator 分配；分配参与调用方事务与否由实现决定（PostgreSQL
// 实现要求事务内调用，回滚即释放该号）。
func (a *Assigner) Assign(ctx context.Context, scope, docType string) (string, error) {
	if scope == "" {
		return "", ErrScopeRequired
	}
	spec, ok := a.registry.Spec(docType)
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownDocType, docType)
	}
	tmpl, _ := a.registry.template(docType)

	at := a.now()
	var seq int64
	if tmpl.HasSequence() {
		value, err := a.sequences.Next(ctx, SequenceKey{
			Scope:   scope,
			DocType: docType,
			Period:  tmpl.PeriodKey(at),
		})
		if err != nil {
			return "", fmt.Errorf("numbering: allocate %s/%s: %w", scope, docType, err)
		}
		seq = value
	}

	return tmpl.Render(RenderInput{
		Seq:                  seq,
		At:                   at,
		Context:              map[string]string{"type": docType},
		CheckAlgo:            spec.CheckAlgo,
		FiscalYearStartMonth: spec.FiscalYearStartMonth,
		// 按系列加扰：不同 scope/类型各自独立、不可互推。
		ScrambleKey: a.secret + "|" + scope + "|" + docType,
	})
}
