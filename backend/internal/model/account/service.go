package account

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"feed-system/internal/pkg/errs"
	"feed-system/internal/pkg/token"
	"feed-system/internal/util/filetype"
	"feed-system/internal/util/password"
	"feed-system/internal/util/username"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// AccountService 用户账号业务层
// 依赖 UserRepository 接口,不直接耦合 GORM,便于测试和替换实现
type AccountService struct {
	userrepo UserRepository
	rdb      *redis.Client // 可选,nil 时跳过登录失败锁定
	devMode  bool          // dev 模式:短信验证码直接返回到响应(仅测试用)
}

// NewAccountService 构造 AccountService
// userrepo 通过接口注入,rdb 可选(nil 表示不启用登录失败锁定),devMode 控制是否在响应里返回验证码
func NewAccountService(userrepo UserRepository, rdb *redis.Client, devMode bool) *AccountService {
	return &AccountService{userrepo: userrepo, rdb: rdb, devMode: devMode}
}

// ============================================================
// 登录失败锁定(Redis 版)
// 计数与锁定用两个独立的 key —— 计数 key 上的 TTL 只是「计数窗口」,
// 不能当成「已锁定」,否则第一次失败就会被判成锁定
// 登录成功后调用 reset 清除
// ============================================================

const (
	loginLockMaxAttempts = 5
	loginLockCountWindow = 15 * time.Minute // 计数窗口:窗口内累计失败次数,到期归零
	loginLockBaseTTL     = 60 * time.Second // 锁的基础时长
	loginLockMaxTTL      = 24 * time.Hour
)

// loginLockCountKey 失败计数器的 key(带计数窗口 TTL)
func (s *AccountService) loginLockCountKey(phone string) string {
	return "feed:fail:" + phone
}

// loginLockFlagKey 锁定标记的 key。只有这个 key 存在才代表「已锁定」
func (s *AccountService) loginLockFlagKey(phone string) string {
	return "feed:fail:lock:" + phone
}

// loginLockCheck 检查是否被锁定,返回剩余时长(0 表示未锁)
func (s *AccountService) loginLockCheck(ctx context.Context, phone string) (time.Duration, error) {
	if s.rdb == nil {
		return 0, nil
	}
	ttl, err := s.rdb.TTL(ctx, s.loginLockFlagKey(phone)).Result()
	if err != nil {
		return 0, err
	}
	if ttl < 0 {
		return 0, nil
	}
	return ttl, nil
}

// loginLockFail 记录一次失败,累计到 MaxAttempts 的倍数时写锁标记
//   - 5、10、15... 次失败 → 锁定,TTL = BaseTTL × 2^((N/5)-1)
//   - 封顶 MaxTTL,达到封顶后不再延长
func (s *AccountService) loginLockFail(ctx context.Context, phone string) (time.Duration, error) {
	if s.rdb == nil {
		return 0, nil
	}
	countKey := s.loginLockCountKey(phone)
	count, err := s.rdb.Incr(ctx, countKey).Result()
	if err != nil {
		return 0, err
	}

	// 第一次失败时开一个计数窗口,避免陈年失败被一直累加
	if count == 1 {
		s.rdb.Expire(ctx, countKey, loginLockCountWindow)
		return 0, nil
	}

	// 未到 5 的倍数,不锁
	if count%int64(loginLockMaxAttempts) != 0 {
		return 0, nil
	}

	// 计算退避:第 N 个 5 倍 → BaseTTL × 2^(N-1)
	tier := count/int64(loginLockMaxAttempts) - 1
	if tier > 30 { // 防溢出
		tier = 30
	}
	ttl := loginLockBaseTTL * time.Duration(1<<uint(tier))
	if ttl > loginLockMaxTTL {
		ttl = loginLockMaxTTL
	}
	// 用 Set 而不是 Expire:一步写入并带 TTL
	if err := s.rdb.Set(ctx, s.loginLockFlagKey(phone), 1, ttl).Err(); err != nil {
		return 0, err
	}
	return ttl, nil
}

// loginLockReset 清除失败计数与锁定标记(登录成功后调用)
func (s *AccountService) loginLockReset(ctx context.Context, phone string) error {
	if s.rdb == nil {
		return nil
	}
	return s.rdb.Del(ctx, s.loginLockCountKey(phone), s.loginLockFlagKey(phone)).Err()
}

// ============================================================
// 业务方法
// ============================================================

func (s *AccountService) Register(ctx context.Context, req RegisterReq) (*TokenResp, error) {
	// 业务校验:用户名完整性(binding 只校验长度,业务校验字符组成)
	if ok, reason := username.Validate(req.UserName); !ok {
		return nil, errs.ErrInvalidParam.WithMsg(reason)
	}

	// 业务校验:密码强度(binding 只校验长度,业务校验字符种类)
	if ok, reason := password.Strong(req.Password); !ok {
		return nil, errs.ErrInvalidParam.WithMsg(reason)
	}

	// 业务校验:用户名唯一性
	u, err := s.userrepo.FindByUserName(ctx, req.UserName)
	if err != nil && !errors.Is(err, ErrNotFound) {
		slog.ErrorContext(ctx, "注册失败:查询用户名出错", "err", err)
		return nil, errs.ErrInternal.WithMsg("注册失败")
	} else if u != nil {
		return nil, errs.ErrConflict.WithMsg("用户名已被占用")
	}

	// 业务校验:手机号唯一性
	u, err = s.userrepo.FindByPhone(ctx, req.Phone)
	if err != nil && !errors.Is(err, ErrNotFound) {
		slog.ErrorContext(ctx, "注册失败:查询手机号出错", "err", err)
		return nil, errs.ErrInternal.WithMsg("注册失败")
	} else if u != nil {
		return nil, errs.ErrConflict.WithMsg("手机号已被注册")
	}

	// 业务校验:邮箱唯一性(可选)
	if req.Email != nil {
		u, err = s.userrepo.FindByEmail(ctx, *req.Email)
		if err != nil && !errors.Is(err, ErrNotFound) {
			slog.ErrorContext(ctx, "注册失败:查询邮箱出错", "err", err)
			return nil, errs.ErrInternal.WithMsg("注册失败")
		} else if u != nil {
			return nil, errs.ErrConflict.WithMsg("邮箱已被注册")
		}
	}

	// 哈希密码(bcrypt)
	hashed, err := password.Hash(req.Password)
	if err != nil {
		slog.ErrorContext(ctx, "注册失败:密码哈希出错", "err", err)
		return nil, errs.ErrInternal.WithMsg("密码哈希失败")
	}

	// 创建用户
	user := &User{
		UserName: req.UserName,
		Password: hashed,
		Phone:    req.Phone,
		Email:    req.Email,
	}
	if err := s.userrepo.Create(ctx, user); err != nil {
		slog.ErrorContext(ctx, "注册失败:创建用户出错", "err", err)
		return nil, errs.ErrInternal.WithMsg("创建用户失败")
	}

	// 发放 token
	// 必须带上 user.Version:DB 里 version 默认是 1(非 0),
	// 若签发时用 0,鉴权中间件比对 claims.Version != dbVersion 会直接 401
	return &TokenResp{
		AccessToken:  mustSignTokenWithVersion(user.ID, user.Version, token.DefaultAccessTTL),
		RefreshToken: mustSignTokenWithVersion(user.ID, user.Version, token.DefaultRefreshTTL),
	}, nil
}

func (s *AccountService) Login(ctx context.Context, req LoginReq) (*TokenResp, error) {
	// 1. 检查是否被锁
	if ttl, err := s.loginLockCheck(ctx, req.Phone); err != nil {
		return nil, errs.ErrInternal.WithMsg("登录失败")
	} else if ttl > 0 {
		return nil, errs.ErrTooFrequent.WithMsg(fmt.Sprintf("登录失败次数过多,请 %d 秒后再试", int(ttl.Seconds())))
	}

	// 2. 查用户
	user, err := s.userrepo.FindByPhone(ctx, req.Phone)
	if errors.Is(err, ErrNotFound) {
		// 用户不存在也算失败,触发锁定(防枚举)
		_, _ = s.loginLockFail(ctx, req.Phone)
		return nil, errs.ErrUnauthorized.WithMsg("手机号或密码错误")
	}
	if err != nil {
		return nil, errs.ErrInternal.WithMsg("登录失败")
	}

	// 3. 校验密码(hash 在前,plain 在后)
	if !password.Verify(user.Password, req.Password) {
		_, _ = s.loginLockFail(ctx, req.Phone)
		return nil, errs.ErrUnauthorized.WithMsg("手机号或密码错误")
	}

	// 4. 登录成功,清除失败计数
	_ = s.loginLockReset(ctx, req.Phone)

	// 5. 更新最后登录时间
	_ = s.userrepo.UpdateLastLoginAt(ctx, user.ID, time.Now())

	// 6. 发放 token(带上 DB 里的 version,否则中间件校验会 401)
	return &TokenResp{
		AccessToken:  mustSignTokenWithVersion(user.ID, user.Version, token.DefaultAccessTTL),
		RefreshToken: mustSignTokenWithVersion(user.ID, user.Version, token.DefaultRefreshTTL),
	}, nil
}

func (s *AccountService) Logout(ctx context.Context, accessToken, refreshToken string) error {
	if refreshToken == "" {
		return errs.ErrUnauthorized.WithMsg("refresh_token 缺失")
	}

	// refresh_token → 黑名单(30d)
	if claims, err := token.Parse(refreshToken); err == nil && claims.ExpiresAt != nil {
		ttl := time.Until(claims.ExpiresAt.Time)
		if ttl > 0 && s.rdb != nil {
			s.rdb.Set(ctx, "feed:revoked:refresh:"+refreshToken, claims.UserID, ttl)
		}
	}

	// access_token → 黑名单(2h,可选)
	if accessToken != "" {
		if claims, err := token.Parse(accessToken); err == nil && claims.ExpiresAt != nil {
			ttl := time.Until(claims.ExpiresAt.Time)
			if ttl > 0 && s.rdb != nil {
				s.rdb.Set(ctx, "feed:revoked:access:"+accessToken, claims.UserID, ttl)
			}
		}
	}

	return nil
}

// findUserByID 按主键查用户,并把仓储错误翻译成业务错误
func (s *AccountService) findUserByID(ctx context.Context, userID int64) (*User, error) {
	user, err := s.userrepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, errs.ErrNotFound.WithMsg("用户不存在")
		}
		return nil, errs.ErrInternal.WithMsg("查询用户失败")
	}
	return user, nil
}

// GetProfile 查询他人公开资料(走 GET /users/:id)
//
// 只返回 PublicUserResp:登录用户可以看别人的昵称和头像,
// 但不该顺手拿到对方的手机号、邮箱和最近登录时间
func (s *AccountService) GetProfile(ctx context.Context, userID int64) (*PublicUserResp, error) {
	user, err := s.findUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toPublicUserResp(user), nil
}

// GetMyProfile 查询自己的完整资料(走 GET /users/me)
func (s *AccountService) GetMyProfile(ctx context.Context, userID int64) (*UserResp, error) {
	user, err := s.findUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toUserResp(user), nil
}

// UpdateProfile 更新用户资料(只允许改 AvatarURL / Email)
// 返回更新后的 UserResp 给前端展示
func (s *AccountService) UpdateProfile(ctx context.Context, userID int64, req UpdateProfileReq) (*UserResp, error) {
	user, err := s.findUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 邮箱唯一性校验(若要改成新邮箱)
	if req.Email != nil && (user.Email == nil || *req.Email != *user.Email) {
		existing, err := s.userrepo.FindByEmail(ctx, *req.Email)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, errs.ErrInternal.WithMsg("查询邮箱失败")
		}
		if existing != nil {
			return nil, errs.ErrConflict.WithMsg("邮箱已被注册")
		}
		user.Email = req.Email
	}

	if req.AvatarURL != nil {
		user.AvatarURL = *req.AvatarURL
	}

	if err := s.userrepo.Update(ctx, user); err != nil {
		return nil, errs.ErrInternal.WithMsg("更新用户失败")
	}
	return toUserResp(user), nil
}

// SendSmsCode 生成 6 位短信验证码,存 Redis(5 分钟)
// dev 模式下返回验证码(给 handler 放进响应方便测试),生产模式返回空字符串
func (s *AccountService) SendSmsCode(ctx context.Context, req SendSmsCodeReq) (string, error) {
	if s.rdb == nil {
		return "", errs.ErrInternal.WithMsg("短信服务未启用")
	}
	code := fmt.Sprintf("%06d", rand.Intn(1000000))
	if err := s.rdb.Set(ctx, smsCodeKey(req.Phone), code, 5*time.Minute).Err(); err != nil {
		return "", errs.ErrInternal.WithMsg("验证码存储失败")
	}
	// dev 模式:打印到日志,方便调试
	fmt.Printf("[SMS-DEV] phone=%s code=%s\n", req.Phone, code)
	// dev 模式返回验证码(供前端调试用),生产模式不返回
	if s.devMode {
		return code, nil
	}
	return "", nil
}

// smsCodeKey 验证码在 Redis 中的 key(写入与校验必须用同一个,避免拼错导致校验恒失败)
func smsCodeKey(phone string) string {
	return "feed:sms:" + phone
}

// errMsgSmsCode 验证码失败的统一文案
const errMsgSmsCode = "验证码错误或已过期"

// readSmsCode 读取验证码。consume=true 时用 GETDEL 原子读取并删除。
func (s *AccountService) readSmsCode(ctx context.Context, phone string, consume bool) (string, error) {
	if s.rdb == nil {
		return "", errs.ErrInternal.WithMsg("短信服务未启用")
	}
	var (
		saved string
		err   error
	)
	if consume {
		saved, err = s.rdb.GetDel(ctx, smsCodeKey(phone)).Result()
	} else {
		saved, err = s.rdb.Get(ctx, smsCodeKey(phone)).Result()
	}
	if errors.Is(err, redis.Nil) {
		return "", errs.ErrInvalidParam.WithMsg(errMsgSmsCode)
	}
	// Redis 故障不能伪装成「验证码错误」:既掩盖了真实故障,也误导用户去重发短信
	if err != nil {
		return "", errs.ErrInternal.WithMsg("校验验证码失败")
	}
	return saved, nil
}

// checkSmsCode 校验验证码但不消费。
//
// 用于「验证码之后还有业务校验」的流程(改密):后续校验失败时用户
// 仍可拿同一个码重试,不必重新等一条短信。
func (s *AccountService) checkSmsCode(ctx context.Context, phone, code string) error {
	saved, err := s.readSmsCode(ctx, phone, false)
	if err != nil {
		return err
	}
	if saved != code {
		return errs.ErrInvalidParam.WithMsg(errMsgSmsCode)
	}
	return nil
}

// consumeSmsCode 校验并一次性消费验证码。
//
// 用 GETDEL 而非 Get+Del:后者两步之间存在窗口,并发请求会读到同一个码
// 且都通过校验,「一次性」就失效了。
func (s *AccountService) consumeSmsCode(ctx context.Context, phone, code string) error {
	saved, err := s.readSmsCode(ctx, phone, true)
	if err != nil {
		return err
	}
	if saved != code {
		return errs.ErrInvalidParam.WithMsg(errMsgSmsCode)
	}
	return nil
}

// RefreshToken 用 refresh_token 换新 token
// 流程:
//  1. 解析 refresh_token → 拿 claims(userID, version, exp)
//  2. 查 Redis:refresh_token 是否在黑名单(已用过的)
//  4. 查 DB: user.Version 是否匹配(防改密后旧 refresh)
//  5. 用过的旧 refresh_token 加入黑名单(防重放)
//  6. 用当前 Version 签发新 access_token + refresh_token
func (s *AccountService) RefreshToken(ctx context.Context, refreshToken string) (*TokenResp, error) {
	if refreshToken == "" {
		return nil, errs.ErrUnauthorized.WithMsg("refresh_token 缺失")
	}

	// 1. 解析
	claims, err := token.Parse(refreshToken)
	if err != nil {
		return nil, errs.ErrUnauthorized.WithMsg("refresh_token 无效或已过期")
	}

	// 2. 查黑名单(已使用过的 refresh_token 不能再用)
	if s.rdb != nil {
		blacklisted, err := s.rdb.Exists(ctx, "feed:revoked:refresh:"+refreshToken).Result()
		if err != nil {
			return nil, errs.ErrInternal.WithMsg("校验 refresh_token 失败")
		}
		if blacklisted > 0 {
			return nil, errs.ErrUnauthorized.WithMsg("refresh_token 已使用过,请重新登录")
		}
	}

	// 3. 查 DB 拿当前 Version
	user, err := s.userrepo.FindByID(ctx, claims.UserID)
	if errors.Is(err, ErrNotFound) {
		return nil, errs.ErrNotFound.WithMsg("用户不存在")
	}
	if err != nil {
		return nil, errs.ErrInternal.WithMsg("查询用户失败")
	}

	// 4. Version 必须匹配(改密后旧 refresh_token 失效)
	if claims.Version != user.Version {
		return nil, errs.ErrUnauthorized.WithMsg("token 已失效,请重新登录")
	}

	// 5. 旧 refresh_token 加入黑名单(防重放)
	if s.rdb != nil && claims.ExpiresAt != nil {
		ttl := time.Until(claims.ExpiresAt.Time)
		if ttl > 0 {
			s.rdb.Set(ctx, "feed:revoked:refresh:"+refreshToken, claims.UserID, ttl)
		}
	}

	// 6. 用当前 Version 签发新 token
	return &TokenResp{
		AccessToken:  mustSignTokenWithVersion(user.ID, user.Version, token.DefaultAccessTTL),
		RefreshToken: mustSignTokenWithVersion(user.ID, user.Version, token.DefaultRefreshTTL),
	}, nil
}

func (s *AccountService) ChangePassword(ctx context.Context, req ChangePasswordReq) (*TokenResp, error) {
	// 0. 校验验证码:既放最前,又不消费。
	//
	// 放最前 —— 若排在业务校验之后,攻击者拿弱密码去试码,收到
	//            「密码强度不足」而不是「验证码错误」,就等于确认码猜中了。
	// 不消费 —— 后面的业务校验失败时用户还能用同一个码重试,不必重发短信。
	if err := s.checkSmsCode(ctx, req.Phone, req.SmsCode); err != nil {
		return nil, err
	}

	// 1. 密码强度是否够
	if ok, reason := password.Strong(req.Password); !ok {
		return nil, errs.ErrInvalidParam.WithMsg(reason)
	}

	// 2. 查找用户是否存在
	user, err := s.userrepo.FindByPhone(ctx, req.Phone)
	if errors.Is(err, ErrNotFound) {
		return nil, errs.ErrNotFound.WithMsg("用户不存在")
	}
	if err != nil {
		return nil, errs.ErrInternal.WithMsg("查询用户失败")
	}

	// 3. 密码是否重复(新密码不能等于旧密码)
	if password.Verify(user.Password, req.Password) {
		return nil, errs.ErrInvalidParam.WithMsg("新密码不能与旧密码相同")
	}

	// 4. 先算哈希 —— bcrypt 是这里唯一还会失败的步骤,放在消费验证码之前,
	//    免得哈希出错把用户的验证码白白烧掉
	hashed, err := password.Hash(req.Password)
	if err != nil {
		return nil, errs.ErrInternal.WithMsg("密码哈希失败")
	}

	// 5. 业务校验全过了,到这里才真正消费验证码
	if err := s.consumeSmsCode(ctx, req.Phone, req.SmsCode); err != nil {
		return nil, err
	}

	// 6. 更新 user.Version + 新 bcrypt 哈希
	user.Password = hashed
	user.Version++ // ★ 让该用户所有旧 token 立刻失效
	if err := s.userrepo.Update(ctx, user); err != nil {
		return nil, errs.ErrInternal.WithMsg("更新用户失败")
	}

	// 7. 发放新的 token(用新 version,旧 token 失效)
	return &TokenResp{
		AccessToken:  mustSignTokenWithVersion(user.ID, user.Version, token.DefaultAccessTTL),
		RefreshToken: mustSignTokenWithVersion(user.ID, user.Version, token.DefaultRefreshTTL),
	}, nil
}

// toUserResp User → UserResp 转换(放在 service 层,业务语义)
// 显式列出字段,Password / DeletedAt 永不暴露
func toUserResp(u *User) *UserResp {
	if u == nil {
		return nil
	}
	return &UserResp{
		ID:          u.ID,
		UserName:    u.UserName,
		Phone:       u.Phone,
		Email:       u.Email,
		AvatarURL:   u.AvatarURL,
		CreatedAt:   u.CreatedAt,
		LastLoginAt: u.LastLoginAt,
	}
}

// toPublicUserResp User → PublicUserResp 转换
// 显式列出字段:将来往 User 加新字段时,不会自动泄漏到公开接口
func toPublicUserResp(u *User) *PublicUserResp {
	if u == nil {
		return nil
	}
	return &PublicUserResp{
		ID:        u.ID,
		UserName:  u.UserName,
		AvatarURL: u.AvatarURL,
		CreatedAt: u.CreatedAt,
	}
}

// mustSignTokenWithVersion 签发带版本号的 token
func mustSignTokenWithVersion(userID int64, version int64, ttl time.Duration) string {
	t, err := token.SignTokenWithVersion(userID, version, ttl)
	if err != nil {
		return ""
	}
	return t
}

// CheckUserVersion 实现 token.VersionChecker 接口
// 中间件校验 token 时,会查 DB 当前 Version 与 token 中的 Version 对比
func (s *AccountService) CheckUserVersion(ctx context.Context, userID int64) (int64, error) {
	user, err := s.findUserByID(ctx, userID)
	if err != nil {
		return 0, err
	}
	return user.Version, nil
}

// newFileName 生成随机文件名(不含扩展名),调用方自行拼接后缀。
//
// 不能用 fileheader.Filename:客户端可控,可含 "../" 造成路径穿越,
// 也可带 .php/.html 后缀,被静态伺服时造成 XSS。
//
// 用 UUID v4(crypto/rand)而非 math/rand —— 后者输出可被反推,
// 攻击者能据此预测出他人的文件名并遍历下载。
func newFileName() string {
	return uuid.NewString()
}

// AvatarURLPrefix 头像对外 URL 前缀,由 main.go 的静态路由挂载。
// 必须与 AvatarStorageDir 对应,否则写进库的 URL 会 404
const AvatarURLPrefix = "/avatars"

// AvatarStorageDir 头像在磁盘上的存储根目录(换机器/进容器会失效,应改为配置)
const AvatarStorageDir = "/home/megumi/gocodehub/feed_system/avatars"

// 头像文件大小上限:10 MiB
const maxAvatarSize = 10 << 20

// 允许的头像格式白名单(魔数判定后使用)
var allowedAvatarMIME = map[string]bool{
	filetype.MIMEJPEG: true,
	filetype.MIMEPNG:  true,
	filetype.MIMEGIF:  true,
	filetype.MIMEWebP: true,
}

func (s *AccountService) UploadAvatar(ctx context.Context, fileheader *multipart.FileHeader, userID int64) error {
	if fileheader.Size <= 0 || fileheader.Size > maxAvatarSize {
		return errs.ErrInvalidParam.WithMsg("头像文件不能为空,且大小不能超过 10MB")
	}

	multiFile, err := fileheader.Open()
	if err != nil {
		return errs.ErrInternal.WithMsg("读取上传文件失败")
	}
	defer multiFile.Close()

	// 后缀和 Content-Type 都由客户端提供,只能用文件头魔数判断真实类型。
	header := make([]byte, filetype.HeaderSize)
	n, readErr := io.ReadFull(multiFile, header)
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		return errs.ErrInternal.WithMsg("读取上传文件失败")
	}
	mime, ok := filetype.DetectImage(header[:n])
	if !ok || !allowedAvatarMIME[mime] {
		return errs.ErrInvalidParam.WithMsg("头像只支持 JPG / PNG / GIF / WebP 格式")
	}
	ext, ok := filetype.ExtForMIME(mime)
	if !ok {
		return errs.ErrInvalidParam.WithMsg("头像只支持 JPG / PNG / GIF / WebP 格式")
	}
	// 魔数读取移动了文件偏移,保存前必须回到文件开头,否则会丢失前 12 字节。
	if _, err := multiFile.Seek(0, io.SeekStart); err != nil {
		return errs.ErrInternal.WithMsg("读取上传文件失败")
	}

	// 权限 0o755:属主可读写执行,其他人可读可执行 ——
	// 目录必须带 x 位才能被进入,不能用 0o644
	dir := filepath.Join(AvatarStorageDir, fmt.Sprintf("%d", userID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.ErrorContext(ctx, "创建头像目录失败", "user_id", userID, "dir", dir, "err", err)
		return err
	}

	// ext 由 handler 从魔数检测结果推导后传入,不用 fileheader.Filename(客户端可伪造)
	fileName := newFileName() + ext
	filePath := filepath.Join(dir, fileName)

	osFile, err := os.Create(filePath)
	if err != nil {
		slog.ErrorContext(ctx, "创建头像文件失败", "user_id", userID, "path", filePath, "err", err)
		return err
	}
	// Close 的错误单独记:写入型文件的 close 可能携带延迟写入的错误
	// (NFS、ext4 延迟分配下的 ENOSPC),不能吞掉
	defer func() {
		if cerr := osFile.Close(); cerr != nil {
			slog.ErrorContext(ctx, "关闭头像文件失败", "user_id", userID, "path", filePath, "err", cerr)
		}
	}()

	if _, err := io.Copy(osFile, multiFile); err != nil {
		_ = os.Remove(filePath) // 写盘失败,清掉半个文件
		slog.ErrorContext(ctx, "写入头像文件失败", "user_id", userID, "path", filePath, "err", err)
		return err
	}

	// 写库:存对外 URL(相对路径,前端同源访问;生产由 Nginx 反代到存储目录)
	avatarURL := fmt.Sprintf("%s/%d/%s", AvatarURLPrefix, userID, fileName)
	if err := s.userrepo.UpdateAvatarURL(ctx, userID, avatarURL); err != nil {
		_ = os.Remove(filePath) // 写库失败,删掉刚存的文件,避免留下孤儿
		slog.ErrorContext(ctx, "更新头像 URL 失败", "user_id", userID, "path", filePath, "err", err)
		return err
	}

	return nil
}
