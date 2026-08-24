package numbering

import (
	"errors"
	"testing"
)

func TestMod10(t *testing.T) {
	// GS1/EAN-13：载荷 400638133393 的校验位是 1。
	if got := Mod10("400638133393"); got != 1 {
		t.Fatalf("mod10(400638133393) = %d, want 1", got)
	}
	// 非数字字符被忽略：TT-2026-0001 的数字是 20260001。
	if got, want := Mod10("TT-2026-0001"), Mod10("20260001"); got != want {
		t.Fatalf("non-digits must be ignored: %d vs %d", got, want)
	}
	if got := Mod10("20260001"); got != 5 {
		t.Fatalf("mod10(20260001) = %d, want 5", got)
	}
}

func TestLuhn(t *testing.T) {
	// 经典 Luhn 例子：载荷 7992739871 的校验位是 3。
	if got := Luhn("7992739871"); got != 3 {
		t.Fatalf("luhn(7992739871) = %d, want 3", got)
	}
	if got := Luhn("20260001"); got != 1 {
		t.Fatalf("luhn(20260001) = %d, want 1", got)
	}
}

func TestMod11(t *testing.T) {
	if got := Mod11("123456789"); got != 2 {
		t.Fatalf("mod11(123456789) = %d, want 2", got)
	}
}

func TestComputeCheck(t *testing.T) {
	cases := []struct {
		algo CheckAlgo
		want string
	}{
		{"", ""},
		{CheckNone, ""},
		{CheckMod10, "5"},
		{CheckSSCC, "5"},
		{CheckLuhn, "1"},
		{"MOD10", "5"}, // 大小写不敏感
	}
	for _, c := range cases {
		got, err := ComputeCheck("20260001", c.algo)
		if err != nil {
			t.Fatalf("%q: %v", c.algo, err)
		}
		if got != c.want {
			t.Fatalf("ComputeCheck(20260001, %q) = %q, want %q", c.algo, got, c.want)
		}
	}
}

func TestComputeCheckMod11MapsTenToX(t *testing.T) {
	// 构造 mod11 = 10 的载荷：单个数字 d，权重 2 → (11-2d%11)%11 = 10 → d=6。
	if got := Mod11("6"); got != 10 {
		t.Fatalf("mod11(6) = %d, want 10", got)
	}
	out, err := ComputeCheck("6", CheckMod11)
	if err != nil {
		t.Fatal(err)
	}
	if out != "X" {
		t.Fatalf("mod11 value 10 must render as X, got %q", out)
	}
}

func TestComputeCheckRejectsUnknownAlgo(t *testing.T) {
	if _, err := ComputeCheck("1", "crc32"); !errors.Is(err, ErrUnknownCheckAlgo) {
		t.Fatalf("expected ErrUnknownCheckAlgo, got %v", err)
	}
}
