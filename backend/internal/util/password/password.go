// Package password 提供密码相关工具函数(强度校验、哈希等)。
package password

import (
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// Strong 校验密码强度
// 要求:
//   - 长度 > 6(不含 6)
//   - 字母、数字、特殊字符 三者中至少满足两类
//
// 返回 (是否通过, 失败原因)。调用方根据失败原因返回不同的业务错误。
func Strong(pwd string) (bool, string) {
	// 1. 长度校验
	if len(pwd) <= 6 {
		return false, "密码长度必须大于 6 位"
	}

	// 2. 统计字符种类
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
