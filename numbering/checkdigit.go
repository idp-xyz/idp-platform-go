package numbering

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// CheckAlgo 标识 {check} 令牌使用的校验位算法。
type CheckAlgo string

const (
	CheckNone  CheckAlgo = "none"
	CheckMod10 CheckAlgo = "mod10"
	CheckSSCC  CheckAlgo = "sscc" // mod10 的别名（GS1/SSCC 习惯叫法）
	CheckLuhn  CheckAlgo = "luhn"
	CheckMod11 CheckAlgo = "mod11"
)

// ErrUnknownCheckAlgo 表示校验位算法名不被支持。
var ErrUnknownCheckAlgo = errors.New("numbering: unknown check algorithm")

// ComputeCheck 对已组装字符串中的数字字符计算校验位。algo 为空或 none
// 时返回空串；未知算法返回错误。
func ComputeCheck(s string, algo CheckAlgo) (string, error) {
	switch CheckAlgo(strings.ToLower(string(algo))) {
	case "", CheckNone:
		return "", nil
	case CheckMod10, CheckSSCC:
		return strconv.Itoa(Mod10(s)), nil
	case CheckLuhn:
		return strconv.Itoa(Luhn(s)), nil
	case CheckMod11:
		if v := Mod11(s); v == 10 {
			return "X", nil
		} else {
			return strconv.Itoa(v), nil
		}
	default:
		return "", fmt.Errorf("%w: %q", ErrUnknownCheckAlgo, algo)
	}
}

func digitsOf(s string) []int {
	out := make([]int, 0, len(s))
	for _, r := range s {
		if r >= '0' && r <= '9' {
			out = append(out, int(r-'0'))
		}
	}
	return out
}

// Mod10 计算 GS1 风格 mod10 校验位（自右向左 3-1 加权）。
func Mod10(s string) int {
	ds := digitsOf(s)
	total := 0
	for i, j := 0, len(ds)-1; j >= 0; i, j = i+1, j-1 {
		if i%2 == 0 {
			total += ds[j] * 3
		} else {
			total += ds[j]
		}
	}
	return (10 - total%10) % 10
}

// Luhn 计算 Luhn 校验位；s 是载荷，校验位追加在最右侧。
func Luhn(s string) int {
	ds := digitsOf(s)
	total := 0
	for i, j := 0, len(ds)-1; j >= 0; i, j = i+1, j-1 {
		d := ds[j]
		if i%2 == 0 {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		total += d
	}
	return (10 - total%10) % 10
}

// Mod11 计算 mod11 校验值（自右向左权重 2..7 循环）；结果 10 由调用方
// 决定映射（ComputeCheck 映射为 "X"）。
func Mod11(s string) int {
	ds := digitsOf(s)
	total := 0
	for i, j := 0, len(ds)-1; j >= 0; i, j = i+1, j-1 {
		total += ds[j] * (i%6 + 2)
	}
	return (11 - total%11) % 11
}
