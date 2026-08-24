package numbering

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var fixedAt = time.Date(2026, 8, 24, 16, 47, 5, 123_000_000, time.UTC)

func mustParse(t *testing.T, raw string) *Template {
	t.Helper()
	tmpl, err := Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return tmpl
}

func TestParseRejectsUnknownTokens(t *testing.T) {
	if _, err := Parse("X-{foo}-{seq}"); !errors.Is(err, ErrInvalidTemplate) {
		t.Fatalf("expected ErrInvalidTemplate, got %v", err)
	}
}

func TestGranularityAndPeriodKey(t *testing.T) {
	cases := []struct {
		raw         string
		granularity Granularity
		period      string
	}{
		{"{yyyy}{mm}{dd}-{seq}", Daily, "20260824"},
		{"STMT-{yyyy}{mm}-{seq:5}", Monthly, "202608"},
		{"{yy}Q{q}-{seq}", Quarterly, "2026Q3"},
		{"TT-{yyyy}-{seq:4}", Yearly, "2026"},
		{"{fyyy}-{seq}", Yearly, "2026"},
		{"DOC-{seq:6}", Continuous, ""},
	}
	for _, c := range cases {
		tmpl := mustParse(t, c.raw)
		if got := tmpl.Granularity(); got != c.granularity {
			t.Fatalf("%q granularity = %s, want %s", c.raw, got, c.granularity)
		}
		if got := tmpl.PeriodKey(fixedAt); got != c.period {
			t.Fatalf("%q period = %q, want %q", c.raw, got, c.period)
		}
	}
}

func TestHasSequence(t *testing.T) {
	if !mustParse(t, "{seqx36:8}").HasSequence() {
		t.Fatal("seqx36 is a sequence token")
	}
	if mustParse(t, "{yyyy}-{rand:6}").HasSequence() {
		t.Fatal("rand is not a sequence token")
	}
}

func TestRenderConfirmedConsumerTemplates(t *testing.T) {
	// TMS 运输任务编号（年重置）。
	got, err := mustParse(t, "TT-{yyyy}-{seq:4}").Render(RenderInput{Seq: 1, At: fixedAt})
	if err != nil {
		t.Fatal(err)
	}
	if got != "TT-2026-0001" {
		t.Fatalf("got %q, want TT-2026-0001", got)
	}

	// Parcel 对账单号（月重置）。
	got, err = mustParse(t, "STMT-{yyyy}{mm}-{seq:5}").Render(RenderInput{Seq: 42, At: fixedAt})
	if err != nil {
		t.Fatal(err)
	}
	if got != "STMT-202608-00042" {
		t.Fatalf("got %q, want STMT-202608-00042", got)
	}
}

func TestRenderDateTimeAndContextTokens(t *testing.T) {
	tmpl := mustParse(t, "{type}/{site}@{yy}{mm}{dd}T{hh}{mi}{ss}.{ms}Q{q}")
	got, err := tmpl.Render(RenderInput{
		At:      fixedAt,
		Context: map[string]string{"type": "TT", "site": "SH01"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "TT/SH01@260824T164705.123Q3" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderSeqWidthAndBase36(t *testing.T) {
	got, err := mustParse(t, "{seq}").Render(RenderInput{Seq: 12007, At: fixedAt})
	if err != nil {
		t.Fatal(err)
	}
	if got != "12007" {
		t.Fatalf("unpadded seq: got %q", got)
	}

	got, err = mustParse(t, "{seq36:4}").Render(RenderInput{Seq: 46655, At: fixedAt})
	if err != nil {
		t.Fatal(err)
	}
	if got != "0ZZZ" {
		t.Fatalf("base36: got %q, want 0ZZZ", got)
	}
}

func TestRenderCheckDigit(t *testing.T) {
	got, err := mustParse(t, "{seq:8}{check}").Render(RenderInput{
		Seq: 20260001, At: fixedAt, CheckAlgo: CheckMod10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "202600015" {
		t.Fatalf("got %q, want 202600015", got)
	}

	// 无算法时 {check} 渲染为空。
	got, err = mustParse(t, "{seq:4}{check}").Render(RenderInput{Seq: 7, At: fixedAt})
	if err != nil {
		t.Fatal(err)
	}
	if got != "0007" {
		t.Fatalf("got %q, want 0007", got)
	}
}

func TestRenderScrambledSequence(t *testing.T) {
	tmpl := mustParse(t, "{seqx:6}")
	in := RenderInput{Seq: 1, At: fixedAt, ScrambleKey: "secret|t1|TT"}
	first, err := tmpl.Render(in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := tmpl.Render(in)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("deterministic: %q vs %q", first, second)
	}
	if len(first) != 6 || strings.Trim(first, "0123456789") != "" {
		t.Fatalf("expected 6 decimal digits, got %q", first)
	}

	// 同序号不同序列键（不同 scope/类型）应当解相关。
	differs := false
	for seq := int64(1); seq <= 50; seq++ {
		a, err := tmpl.Render(RenderInput{Seq: seq, At: fixedAt, ScrambleKey: "k|a"})
		if err != nil {
			t.Fatal(err)
		}
		b, err := tmpl.Render(RenderInput{Seq: seq, At: fixedAt, ScrambleKey: "k|b"})
		if err != nil {
			t.Fatal(err)
		}
		if a != b {
			differs = true
			break
		}
	}
	if !differs {
		t.Fatal("different scramble keys must decorrelate the series")
	}
}

func TestRenderRandUsesInjectedSource(t *testing.T) {
	tmpl := mustParse(t, "R{rand:4}")
	got, err := tmpl.Render(RenderInput{
		At:         fixedAt,
		RandDigits: func(width int) (string, error) { return strings.Repeat("7", width), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "R7777" {
		t.Fatalf("got %q", got)
	}

	// 默认 crypto/rand：宽度与字符集正确。
	got, err = mustParse(t, "{rand}").Render(RenderInput{At: fixedAt})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 || strings.Trim(got, "0123456789") != "" {
		t.Fatalf("default rand must be 4 decimal digits, got %q", got)
	}
}

func TestRenderFiscalYear(t *testing.T) {
	tmpl := mustParse(t, "{fyyy}/{fy}")
	got, err := tmpl.Render(RenderInput{At: fixedAt, FiscalYearStartMonth: 4})
	if err != nil {
		t.Fatal(err)
	}
	if got != "FY2026/FY26" {
		t.Fatalf("start month 4: got %q", got)
	}
	got, err = tmpl.Render(RenderInput{At: fixedAt, FiscalYearStartMonth: 9})
	if err != nil {
		t.Fatal(err)
	}
	if got != "FY2025/FY25" {
		t.Fatalf("start month 9: got %q", got)
	}
}

func TestRenderRequiresTime(t *testing.T) {
	if _, err := mustParse(t, "{seq}").Render(RenderInput{Seq: 1}); !errors.Is(err, ErrRenderInput) {
		t.Fatalf("expected ErrRenderInput, got %v", err)
	}
}
