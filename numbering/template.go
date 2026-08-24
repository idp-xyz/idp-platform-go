package numbering

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Granularity 是由模板令牌推导出的序列重置粒度。
type Granularity string

const (
	Continuous Granularity = "continuous"
	Yearly     Granularity = "yearly"
	Quarterly  Granularity = "quarterly"
	Monthly    Granularity = "monthly"
	Daily      Granularity = "daily"
)

// ErrInvalidTemplate 表示模板不能被解析（未知令牌等）。
var ErrInvalidTemplate = errors.New("numbering: invalid template")

// ErrRenderInput 表示渲染输入缺少必要信息。
var ErrRenderInput = errors.New("numbering: invalid render input")

var tokenPattern = regexp.MustCompile(`\{([a-z0-9]+)(?::(\d+))?\}`)

var knownTokens = map[string]bool{
	"type": true, "site": true, "owner": true, "region": true, "le": true,
	"yyyy": true, "yy": true, "mm": true, "dd": true, "q": true,
	"fy": true, "fyyy": true,
	"hh": true, "mi": true, "ss": true, "ms": true,
	"seq": true, "seq36": true, "seqx": true, "seqx36": true,
	"rand": true, "check": true,
}

type segment struct {
	literal string // token == "" 时生效
	token   string
	width   int
}

// Template 是解析后的编号模板。令牌词汇表：
//
//	{type} {site} {owner} {region} {le}      上下文值
//	{yyyy} {yy} {mm} {dd} {q}                日期部件/季度
//	{hh} {mi} {ss} {ms}                      时间部件（{mm}=月、{mi}=分）
//	{fy} {fyyy}                              财年（按起始月）
//	{seq} {seq:N} {seq36} {seq36:N}          顺序号（零填充 N / Base36）
//	{seqx} {seqx:N} {seqx36} {seqx36:N}      加扰顺序号（不可猜测）
//	{rand} {rand:N}                          N 位随机十进制数字
//	{check}                                  对已组装数字的校验位
//
// 序列重置粒度由存在的最细日期令牌推导：{dd}→日、{mm}→月、{q}→季、
// 年令牌→年、无→连续。
type Template struct {
	raw      string
	segments []segment
}

// Parse 解析模板；出现未知令牌时返回 ErrInvalidTemplate。
func Parse(raw string) (*Template, error) {
	t := &Template{raw: raw}
	rest := raw
	var unknown []string
	for {
		loc := tokenPattern.FindStringSubmatchIndex(rest)
		if loc == nil {
			break
		}
		if lit := rest[:loc[0]]; lit != "" {
			t.segments = append(t.segments, segment{literal: lit})
		}
		name := rest[loc[2]:loc[3]]
		width := 0
		if loc[4] >= 0 {
			width, _ = strconv.Atoi(rest[loc[4]:loc[5]])
		}
		if !knownTokens[name] {
			unknown = append(unknown, name)
		}
		t.segments = append(t.segments, segment{token: name, width: width})
		rest = rest[loc[1]:]
	}
	if rest != "" {
		t.segments = append(t.segments, segment{literal: rest})
	}
	if len(unknown) > 0 {
		return nil, fmt.Errorf("%w: unknown tokens %v in %q", ErrInvalidTemplate, unknown, raw)
	}
	return t, nil
}

// Raw 返回原始模板字符串。
func (t *Template) Raw() string { return t.raw }

// HasSequence 报告模板是否包含序列令牌（seq/seq36/seqx/seqx36）。
func (t *Template) HasSequence() bool {
	for _, s := range t.segments {
		switch s.token {
		case "seq", "seq36", "seqx", "seqx36":
			return true
		}
	}
	return false
}

// Granularity 返回由最细日期令牌推导的重置粒度。
func (t *Template) Granularity() Granularity {
	present := map[string]bool{}
	for _, s := range t.segments {
		if s.token != "" {
			present[s.token] = true
		}
	}
	switch {
	case present["dd"]:
		return Daily
	case present["mm"]:
		return Monthly
	case present["q"]:
		return Quarterly
	case present["yyyy"] || present["yy"] || present["fy"] || present["fyyy"]:
		return Yearly
	default:
		return Continuous
	}
}

// PeriodKey 返回 at 时刻所在重置周期的键；连续序列返回空串。
// 日期部件取自 at 自身的位置（Location），调用方决定业务日历。
func (t *Template) PeriodKey(at time.Time) string {
	switch t.Granularity() {
	case Daily:
		return at.Format("20060102")
	case Monthly:
		return at.Format("200601")
	case Quarterly:
		return fmt.Sprintf("%dQ%d", at.Year(), (int(at.Month())-1)/3+1)
	case Yearly:
		return at.Format("2006")
	default:
		return ""
	}
}

// RenderInput 是一次渲染的全部输入。
type RenderInput struct {
	// Seq 是本次分配到的 1 基顺序号；模板无序列令牌时忽略。
	Seq int64
	// At 是渲染时刻，必填；日期/时间部件取自其所在 Location。
	At time.Time
	// Context 提供 {type}/{site}/{owner}/{region}/{le} 的取值，缺省为空串。
	Context map[string]string
	// CheckAlgo 是 {check} 的算法；空值等于 none。
	CheckAlgo CheckAlgo
	// FiscalYearStartMonth 是财年起始月（1-12），0 视为 1。
	FiscalYearStartMonth int
	// ScrambleKey 是 {seqx}/{seqx36} 的按系列密钥；渲染时会追加周期键
	// 做周期去相关（同一序号在不同周期加扰为不同值）。
	ScrambleKey string
	// ScrambleRounds 是 Feistel 轮数，0 视为默认 6。
	ScrambleRounds int
	// RandDigits 生成 {rand} 的 N 位十进制数字；nil 使用 crypto/rand。
	RandDigits func(width int) (string, error)
}

const checkPlaceholder = "\x00"

// Render 按输入渲染模板。
func (t *Template) Render(in RenderInput) (string, error) {
	if in.At.IsZero() {
		return "", fmt.Errorf("%w: At is zero", ErrRenderInput)
	}
	effKey := in.ScrambleKey + "|" + t.PeriodKey(in.At)
	index := int64(0)
	if in.Seq > 0 {
		index = in.Seq - 1
	}
	randDigits := in.RandDigits
	if randDigits == nil {
		randDigits = cryptoRandDigits
	}

	var b strings.Builder
	hasCheck := false
	for _, s := range t.segments {
		if s.token == "" {
			b.WriteString(s.literal)
			continue
		}
		part, err := t.renderToken(s, in, effKey, index, randDigits)
		if err != nil {
			return "", err
		}
		if part == checkPlaceholder {
			hasCheck = true
		}
		b.WriteString(part)
	}
	out := b.String()
	if hasCheck {
		base := strings.ReplaceAll(out, checkPlaceholder, "")
		digit, err := ComputeCheck(base, in.CheckAlgo)
		if err != nil {
			return "", err
		}
		out = strings.ReplaceAll(out, checkPlaceholder, digit)
	}
	return out, nil
}

func (t *Template) renderToken(
	s segment,
	in RenderInput,
	effKey string,
	index int64,
	randDigits func(int) (string, error),
) (string, error) {
	at := in.At
	switch s.token {
	case "type", "site", "owner", "region", "le":
		return in.Context[s.token], nil
	case "yyyy":
		return at.Format("2006"), nil
	case "yy":
		return at.Format("06"), nil
	case "mm":
		return at.Format("01"), nil
	case "dd":
		return at.Format("02"), nil
	case "q":
		return strconv.Itoa((int(at.Month())-1)/3 + 1), nil
	case "hh":
		return at.Format("15"), nil
	case "mi":
		return at.Format("04"), nil
	case "ss":
		return at.Format("05"), nil
	case "ms":
		return fmt.Sprintf("%03d", at.Nanosecond()/1e6), nil
	case "fy", "fyyy":
		return fiscalYear(at, in.FiscalYearStartMonth, s.token), nil
	case "seq":
		if s.width > 0 {
			return fmt.Sprintf("%0*d", s.width, in.Seq), nil
		}
		return strconv.FormatInt(in.Seq, 10), nil
	case "seq36":
		out := base36(in.Seq)
		if s.width > 0 && len(out) < s.width {
			out = strings.Repeat("0", s.width-len(out)) + out
		}
		return out, nil
	case "seqx":
		width := s.width
		if width == 0 {
			width = 6
		}
		value, effWidth, err := Scramble(index, 10, width, effKey, in.ScrambleRounds)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%0*d", effWidth, value), nil
	case "seqx36":
		width := s.width
		if width == 0 {
			width = 6
		}
		value, effWidth, err := Scramble(index, 36, width, effKey, in.ScrambleRounds)
		if err != nil {
			return "", err
		}
		out := base36(value)
		if len(out) < effWidth {
			out = strings.Repeat("0", effWidth-len(out)) + out
		}
		return out, nil
	case "rand":
		width := s.width
		if width == 0 {
			width = 4
		}
		return randDigits(width)
	case "check":
		return checkPlaceholder, nil
	default:
		// Parse 已拒绝未知令牌。
		return "", fmt.Errorf("%w: token %q", ErrInvalidTemplate, s.token)
	}
}

func fiscalYear(at time.Time, startMonth int, token string) string {
	if startMonth <= 0 {
		startMonth = 1
	}
	year := at.Year()
	if int(at.Month()) < startMonth {
		year--
	}
	if token == "fy" {
		return fmt.Sprintf("FY%02d", year%100)
	}
	return fmt.Sprintf("FY%d", year)
}

const base36Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

func base36(n int64) string {
	if n <= 0 {
		return "0"
	}
	buf := make([]byte, 0, 13)
	for n > 0 {
		buf = append(buf, base36Alphabet[n%36])
		n /= 36
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}

func cryptoRandDigits(width int) (string, error) {
	limit := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(width)), nil)
	n, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return "", fmt.Errorf("numbering: rand token: %w", err)
	}
	return fmt.Sprintf("%0*d", width, n), nil
}
