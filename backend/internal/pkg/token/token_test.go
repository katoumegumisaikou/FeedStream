package token

import (
	"testing"
	"time"
)

// 同一秒内用相同参数签发两次,必须得到不同的 token。
//
// 没有 jti 时两者字节完全相同 —— iat / exp 只有秒级精度,而刷新流程是
// 「先拉黑旧的、再签新的」,于是新 token 一签发就已经在黑名单里,用户下次刷新必然 401。
func TestSignToken_同一秒内签发不重复(t *testing.T) {
	a, err := SignTokenWithVersion(1, 1, DefaultRefreshTTL)
	if err != nil {
		t.Fatalf("签发失败: %v", err)
	}
	b, err := SignTokenWithVersion(1, 1, DefaultRefreshTTL)
	if err != nil {
		t.Fatalf("签发失败: %v", err)
	}
	if a == b {
		t.Fatal("同一秒内两次签发的 token 完全相同:jti 缺失或未生效")
	}

	ca, err := Parse(a)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	cb, err := Parse(b)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	// 业务载荷应一致,差异只允许出现在 jti 上
	if ca.UserID != cb.UserID || ca.Version != cb.Version {
		t.Fatalf("业务载荷不该有差异: %+v vs %+v", ca.RegisteredClaims, cb.RegisteredClaims)
	}
	if ca.ID == "" {
		t.Fatal("jti 不应为空")
	}
	if ca.ID == cb.ID {
		t.Fatal("jti 必须每次唯一")
	}

	if got := ca.ExpiresAt.Sub(ca.IssuedAt.Time); got != DefaultRefreshTTL {
		t.Fatalf("有效期应为 %v,实际 %v", DefaultRefreshTTL, got)
	}
	if got := ca.NotBefore.Sub(ca.IssuedAt.Time); got != 0 {
		t.Fatalf("nbf 应等于 iat,实际相差 %v", got)
	}
	if time.Since(ca.IssuedAt.Time) > time.Minute {
		t.Fatalf("iat 偏离当前时间过远: %v", ca.IssuedAt.Time)
	}
}
