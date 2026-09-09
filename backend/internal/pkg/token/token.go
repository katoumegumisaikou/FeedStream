// Package token 提供 JWT 签发与解析。
//
// 设计:
//   - SignToken / SignTokenWithVersion:签发 JWT
//   - Parse:解析并校验 JWT
//   - 不依赖任何业务包(token 是纯工具)
package token

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// jwtSecret JWT 签名密钥
// TODO: 从配置(env / config.yaml)读取
var jwtSecret = []byte("katoumegumi")

// Token 有效期
const (
	DefaultAccessTTL  = 2 * time.Hour
	DefaultRefreshTTL = 30 * 24 * time.Hour
)

// Claims 自定义 JWT 载荷
//   - UserID:用户主键
//   - Version:用户版本号(改密码 / 注销时递增),校验时与 DB 中的值比对
type Claims struct {
	UserID  int64 `json:"user_id"`
	Version int64 `json:"version"`
	jwt.RegisteredClaims
}

// SignToken 用 HMAC-SHA256 签发 JWT
// ttl 决定有效期(access 用 2h,refresh 用 30d)
// version 用于强制下线:值变化即 token 失效
func SignToken(userID int64, ttl time.Duration) (string, error) {
	return signToken(userID, 0, ttl)
}

// SignTokenWithVersion 同 SignToken,但带版本号
// 改密码 / 注销账号时,新 token 用更高版本号,旧 token 校验时会失败
func SignTokenWithVersion(userID int64, version int64, ttl time.Duration) (string, error) {
	return signToken(userID, version, ttl)
}

func signToken(userID int64, version int64, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:  userID,
		Version: version,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(jwtSecret)
}

// Parse 解析并校验 token,返回 Claims
func Parse(tokenStr string) (*Claims, error) {
	t, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("非预期的签名算法")
		}
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := t.Claims.(*Claims); ok && t.Valid {
		return claims, nil
	}
	return nil, errors.New("token 无效")
}