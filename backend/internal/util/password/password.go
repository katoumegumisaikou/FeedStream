// Package password 提供密码相关工具函数(强度校验、哈希等)。
package password

import (
	"fmt"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// 密码长度区间
// ⚠️ 必须与 dto.go 里 RegisterReq / ChangePasswordReq 的 binding tag 保持一致
const (
	MinLen = 8
	MaxLen = 16
)

// 密码允许的字符范围:ASCII 可打印字符,排除空格(0x20)
// 排除空格是为了避免首尾空格导致的「我明明输对了」类问题
const (
	asciiMin = 0x21 // '!'
	asciiMax = 0x7E // '~'
)

// Strong 校验密码强度
// 要求:
//   - 只允许 ASCII 可打印字符(不支持中文、空格等)
//   - 长度 8-16(与 dto.go 的 binding 一致)
//   - 字母、数字、特殊字符 三者中至少满足两类
//
// 限制 ASCII 的原因:规避 Unicode 归一化问题。
// 同一个「é」在不同平台可能是 NFC(U+00E9,2 字节)或 NFD(e + U+0301,3 字节),
// 肉眼完全一样但字节不同,bcrypt 按字节比对 → 用户会遇到「密码明明输对了却登不进去」,
// 且无从排查。另外零宽字符、输入法差异等也会带来同类问题。
//
// 返回 (是否通过, 失败原因)。调用方根据失败原因返回不同的业务错误。
func Strong(pwd string) (bool, string) {
	// 1. 字符集校验(放最前,让用了中文的用户一次就看到真正原因)
	for _, r := range pwd {
		if r < asciiMin || r > asciiMax {
			return false, "密码只能使用字母、数字和常见符号(不支持中文、空格等)"
		}
	}

	// 2. 长度校验
	// 上面已确保全是 ASCII,此时字符数 == 字节数,直接用 len() 即可
	if n := len(pwd); n < MinLen || n > MaxLen {
		return false, fmt.Sprintf("密码长度必须为 %d-%d 位", MinLen, MaxLen)
	}

	// 3. 统计字符种类
	var hasLetter, hasDigit, hasSpecial bool
	for _, r := range pwd {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}

	// 3. 至少满足两类
	kinds := 0
	if hasLetter {
		kinds++
	}
	if hasDigit {
		kinds++
	}
	if hasSpecial {
		kinds++
	}
	if kinds < 2 {
		return false, "密码必须包含字母、数字、特殊字符中的至少两类"
	}

	return true, ""
}

// Hash 用 bcrypt 哈希明文密码(cost=12)
// 返回的哈希可直接存入数据库 Password 字段
func Hash(plain string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), 12)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// Verify 验证明文密码与 bcrypt 哈希是否匹配
func Verify(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
