package numbering

import (
	"errors"
	"fmt"
)

// DocTypeSpec 是单据类型的编号规格。单据类型本身由消费者定义并注册，
// 平台不预置任何业务类型。
type DocTypeSpec struct {
	// Prefix 用于缺省模板；Template 非空时可以不设。
	Prefix string
	// Template 是编号模板；空串使用缺省 "{Prefix}-{yyyy}{mm}-{seq:6}"。
	Template string
	// CheckAlgo 是 {check} 令牌的算法；空值等于 none。
	CheckAlgo CheckAlgo
	// FiscalYearStartMonth 是 {fy}/{fyyy} 的财年起始月，0 视为 1。
	FiscalYearStartMonth int
}

// EffectiveTemplate 返回生效模板（未配置时的月度缺省）。
func (s DocTypeSpec) EffectiveTemplate() string {
	if s.Template != "" {
		return s.Template
	}
	return s.Prefix + "-{yyyy}{mm}-{seq:6}"
}

// ErrInvalidSpec 表示注册的单据类型规格非法。
var ErrInvalidSpec = errors.New("numbering: invalid doc type spec")

// Registry 是经过整体校验的单据类型注册表；模板在构造时解析一次。
type Registry struct {
	specs     map[string]DocTypeSpec
	templates map[string]*Template
}

// NewRegistry 校验并构造注册表：类型码非空、模板可解析、校验算法可用、
// 财年起始月在 0-12。
func NewRegistry(specs map[string]DocTypeSpec) (*Registry, error) {
	r := &Registry{
		specs:     make(map[string]DocTypeSpec, len(specs)),
		templates: make(map[string]*Template, len(specs)),
	}
	for docType, spec := range specs {
		if docType == "" {
			return nil, fmt.Errorf("%w: empty doc type", ErrInvalidSpec)
		}
		if spec.Prefix == "" && spec.Template == "" {
			return nil, fmt.Errorf("%w: %s needs a prefix or a template", ErrInvalidSpec, docType)
		}
		if spec.FiscalYearStartMonth < 0 || spec.FiscalYearStartMonth > 12 {
			return nil, fmt.Errorf(
				"%w: %s fiscal year start month %d",
				ErrInvalidSpec, docType, spec.FiscalYearStartMonth,
			)
		}
		if _, err := ComputeCheck("", spec.CheckAlgo); err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrInvalidSpec, docType, err)
		}
		tmpl, err := Parse(spec.EffectiveTemplate())
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrInvalidSpec, docType, err)
		}
		r.specs[docType] = spec
		r.templates[docType] = tmpl
	}
	return r, nil
}

// Spec 返回注册的规格。
func (r *Registry) Spec(docType string) (DocTypeSpec, bool) {
	spec, ok := r.specs[docType]
	return spec, ok
}

func (r *Registry) template(docType string) (*Template, bool) {
	tmpl, ok := r.templates[docType]
	return tmpl, ok
}
