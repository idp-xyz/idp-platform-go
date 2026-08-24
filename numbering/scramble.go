package numbering

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// ErrScrambleDomain 表示 radix**width 超出可安全表示的加扰域。
var ErrScrambleDomain = errors.New("numbering: scramble domain overflows int64")

// maxScrambleDomain 限制加扰域大小，保证 side*side 与序号运算不溢出。
const maxScrambleDomain = int64(1) << 62

const defaultScrambleRounds = 6

// Scramble 把 0 基序号映射为固定域内不可猜测的值（Feistel 网络 + cycle
// walking，keyed SHA-256 轮函数）。返回 (值, 有效宽度)：值是 radix**有效宽度
// 域内对序号的双射，不同序号永不碰撞；有效宽度只在计数器超出配置域时增长，
// 与 {seq:N} 超宽后自然变长的行为一致。
func Scramble(index int64, radix, width int, key string, rounds int) (int64, int, error) {
	if radix < 2 {
		return 0, 0, fmt.Errorf("%w: radix %d", ErrScrambleDomain, radix)
	}
	if rounds <= 0 {
		rounds = defaultScrambleRounds
	}
	width = max(width, 1)
	index = max(index, 0)

	effWidth := width
	domain, err := powDomain(radix, effWidth)
	if err != nil {
		return 0, 0, err
	}
	for index >= domain {
		effWidth++
		if domain > maxScrambleDomain/int64(radix) {
			return 0, 0, fmt.Errorf("%w: radix %d width %d", ErrScrambleDomain, radix, effWidth)
		}
		domain *= int64(radix)
	}
	if domain <= 1 {
		return 0, effWidth, nil
	}

	side := sideFor(domain)
	value := feistel(index, side, key, rounds)
	// cycle-walk：side*side >= domain，落在域外的值重新加密直到回到
	// [0, domain)，保持对精确域的双射。
	for value >= domain {
		value = feistel(value, side, key, rounds)
	}
	return value, effWidth, nil
}

func powDomain(radix, width int) (int64, error) {
	domain := int64(1)
	for i := 0; i < width; i++ {
		if domain > maxScrambleDomain/int64(radix) {
			return 0, fmt.Errorf("%w: radix %d width %d", ErrScrambleDomain, radix, width)
		}
		domain *= int64(radix)
	}
	return domain, nil
}

// sideFor 返回满足 side*side >= domain 的最小边长。
func sideFor(domain int64) int64 {
	side := int64(math.Sqrt(float64(domain)))
	for side > 1 && (side-1)*(side-1) >= domain {
		side--
	}
	for side*side < domain {
		side++
	}
	return max(side, 1)
}

// feistel 是 [0, side*side) 上的平衡 Feistel 置换；每轮单独可逆，因此
// 任意轮函数都不会产生碰撞。轮函数输出先对 side 取模再相加，避免溢出
// 破坏模运算。
func feistel(value, side int64, key string, rounds int) int64 {
	left, right := value/side, value%side
	for round := 0; round < rounds; round++ {
		f := int64(prf(key, round, right) % uint64(side))
		left, right = right, (left+f)%side
	}
	return left*side + right
}

func prf(key string, round int, value int64) uint64 {
	digest := sha256.Sum256(fmt.Appendf(nil, "%s|%d|%d", key, round, value))
	return binary.BigEndian.Uint64(digest[:8])
}
