// Package username 提供用户名校验工具。
package username

import "unicode"

// Validate 校验用户名字符完整性
// 规则:
//   - 首字符必须是字母(不能以数字或下划线开头)
//   - 仅允许字母、数字、下划线
//   - 至少包含一个字母(防止纯数字用户名)
//
// 长度校验由 dto binding 处理(min=3, max=32),本函数只校验字符组成
func Validate(name string) (bool, string) {
	if len(name) == 0 || !unicode.IsLetter(rune(name[0])) {
		return false, "用户名必须以字母开头"
	}

	var hasLetter bool
	for _, r := range name {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r), r == '_':
			// 合法字符
		default:
			return false, "用户名只能包含字母、数字和下划线"
		}
	}

	if !hasLetter {
		return false, "用户名必须包含至少一个字母"
	}

	return true, ""
}
