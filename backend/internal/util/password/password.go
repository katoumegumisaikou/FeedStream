// Package password 提供密码相关工具函数(强度校验、哈希等)。
package password

import (
	"fmt"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// 密码长度区间,须与 dto.go 里 RegisterReq / ChangePasswordReq 的 binding tag 保持一致
const (
	MinLen = 8
	MaxLen = 16
)

// 密码字符范围:ASCII 可打印,排除空格(0x20),避免首尾空格引发「明明输对了」类问题
const (
	asciiMin = 0x21 // '!'
	asciiMax = 0x7E // '~'
)

// Strong 校验密码强度:仅允许 ASCII 可打印字符(不支持中文、空格),长度 8-16
// (与 dto.go binding 一致),且字母、数字、特殊字符至少满足两类。
//
// 限 ASCII 是为规避 Unicode 归一化:同一个「é」在不同平台可能是 NFC(2 字节)或
// NFD(3 字节),肉眼一样但字节不同,bcrypt 按字节比对 → 用户「密码输对了却登不进去」
// 且无从排查;零宽字符、输入法差异同理。
//
// 返回 (是否通过, 失败原因),调用方据此返回不同业务错误。
func Strong(pwd string) (bool, string) {
	// 1. 字符集校验(放最前,中文用户先看到真正原因)
	for _, r := range pwd {
		if r < asciiMin || r > asciiMax {
			return false, "密码只能使用字母、数字和常见符号(不支持中文、空格等)"
		}
	}

	// 2. 长度校验(上面已确保全 ASCII,字符数 == 字节数,可直接用 len())
	if n := len(pwd); n < MinLen || n > MaxLen {
		return false, fmt.Sprintf("密码长度必须为 %d-%d 位", MinLen, MaxLen)
	}

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

// Hash 用 bcrypt(cost=12)哈希明文密码,结果可直接存入数据库 Password 字段
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
