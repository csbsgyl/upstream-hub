// Package auth 提供单管理员登录：凭据持久化在数据库（admin_users 表），
// 密码以 bcrypt 哈希保存，登录后下发 HMAC-SHA256 签名的短 token。
//
// Token 格式："<base64url(payload)>.<base64url(hmac)>"，payload 是
// {"sub":"<user>","exp":<unix>,"mc":<bool>}。mc=true 表示该账号需要先改密码。
// 服务端无状态，AppSecret 不变的情况下重启 token 仍有效。
//
// 首次登录强制改密：默认账号 admin/admin 由 Seed 播种时打上 mc 标志，
// 中间件在 mc=true 时只放行改密相关端点，前端据此渲染强制改密页。
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/worryzyy/upstream-hub/internal/storage"
	"golang.org/x/crypto/bcrypt"
)

// minPasswordLen 新密码最小长度，避免改密又改回弱密码。
const minPasswordLen = 6

// Service 单管理员登录服务。凭据来自 *storage.AdminUsers，不再写死在 config。
type Service struct {
	users    *storage.AdminUsers
	secret   []byte
	tokenTTL time.Duration
}

// New 构造 Service。secret 推荐 32 字节以上；若为空报错。
// 调用方应在 secret 为空时回退到 APP_SECRET。
func New(users *storage.AdminUsers, secret string, ttl time.Duration) (*Service, error) {
	if users == nil {
		return nil, errors.New("auth user repository is nil")
	}
	if secret == "" {
		return nil, errors.New("auth token secret is empty")
	}
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	return &Service{
		users:    users,
		secret:   []byte(secret),
		tokenTTL: ttl,
	}, nil
}

// LoginResult 登录 / 改密成功后返回给上层的信息。
type LoginResult struct {
	Token              string
	ExpiresAt          time.Time
	Username           string
	MustChangePassword bool
}

// claims 是签发到 token payload 里的最小必要字段。
type claims struct {
	Sub string `json:"sub"`
	Exp int64  `json:"exp"`
	MC  bool   `json:"mc,omitempty"` // must change password
}

// Login 校验账号密码（bcrypt），成功返回新 token 与是否需要强制改密。
//
// 安全：用户不存在时仍跑一次 bcrypt 比较占位哈希，避免通过响应时间区分
// "用户不存在" vs "密码错误"。
func (s *Service) Login(username, password string) (*LoginResult, error) {
	user, err := s.users.FindByUsername(username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		// 恒定时间占位比较，抹平时序差异。
		_ = bcrypt.CompareHashAndPassword(
			[]byte("$2a$10$invalidinvalidinvalidinvalidinvalidinvalidinvalidinv"),
			[]byte(password),
		)
		return nil, errors.New("invalid username or password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("invalid username or password")
	}
	return s.issue(user.Username, user.MustChangePassword)
}

// ChangePassword 校验旧密码后设置新密码，清除强制改密标志，返回一枚干净的新 token。
func (s *Service) ChangePassword(username, oldPassword, newPassword string) (*LoginResult, error) {
	if len(newPassword) < minPasswordLen {
		return nil, fmt.Errorf("new password must be at least %d characters", minPasswordLen)
	}
	user, err := s.users.FindByUsername(username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return nil, errors.New("current password is incorrect")
	}
	if oldPassword == newPassword {
		return nil, errors.New("new password must be different from the current one")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	if err := s.users.UpdatePassword(user.ID, string(hash)); err != nil {
		return nil, err
	}
	return s.issue(user.Username, false)
}

// issue 签发一枚 token，组装 LoginResult。
func (s *Service) issue(username string, mustChange bool) (*LoginResult, error) {
	expiresAt := time.Now().Add(s.tokenTTL)
	tok, err := s.sign(claims{Sub: username, Exp: expiresAt.Unix(), MC: mustChange})
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		Token:              tok,
		ExpiresAt:          expiresAt,
		Username:           username,
		MustChangePassword: mustChange,
	}, nil
}

// Verify 校验 token 合法性并返回 subject + 是否仍需强制改密。
func (s *Service) Verify(token string) (string, bool, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", false, errors.New("malformed token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", false, fmt.Errorf("decode payload: %w", err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", false, fmt.Errorf("decode sig: %w", err)
	}
	expectedSig := s.mac(payload)
	if subtle.ConstantTimeCompare(sig, expectedSig) != 1 {
		return "", false, errors.New("bad signature")
	}
	var c claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return "", false, fmt.Errorf("decode claims: %w", err)
	}
	if time.Now().Unix() > c.Exp {
		return "", false, errors.New("token expired")
	}
	return c.Sub, c.MC, nil
}

func (s *Service) sign(c claims) (string, error) {
	body, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	sig := s.mac(body)
	return base64.RawURLEncoding.EncodeToString(body) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func (s *Service) mac(payload []byte) []byte {
	m := hmac.New(sha256.New, s.secret)
	m.Write(payload)
	return m.Sum(nil)
}

// TokenTTL 返回登录 token 的有效期。
func (s *Service) TokenTTL() time.Duration { return s.tokenTTL }

// Seed 在 auth 开启且 admin_users 表为空时播种一个默认管理员。
//
//   - password 非空（管理员通过 ADMIN_PASSWORD 显式设了密码）→ 直接用，无需强制改密。
//   - password 为空（默认情形）→ 播种 admin/admin 并打上强制改密标志，首次登录必须改密。
//
// 表非空时什么都不做，避免每次重启覆盖已改的密码。
func Seed(users *storage.AdminUsers, username, password string, log *slog.Logger) error {
	if username == "" {
		username = "admin"
	}
	n, err := users.Count()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	mustChange := false
	if password == "" {
		password = "admin"
		mustChange = true
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u := &storage.AdminUser{
		Username:           username,
		PasswordHash:       string(hash),
		MustChangePassword: mustChange,
	}
	if err := users.Create(u); err != nil {
		return err
	}
	if log != nil {
		if mustChange {
			log.Warn("seeded default admin account — login with admin/admin and change the password on first login",
				"username", username)
		} else {
			log.Info("seeded admin account from ADMIN_PASSWORD", "username", username)
		}
	}
	return nil
}

// Middleware 校验 Authorization 头。不通过返回 401。
//
// 强制改密：token 的 mc=true 时，除"改密相关端点"外一律 403，
// 逼前端先走改密流程。
//
// 路径白名单（完全免鉴权）：
//   - "/healthz"
//   - "/api/version"
//   - "/api/auth/login"
func (s *Service) Middleware() gin.HandlerFunc {
	whitelist := map[string]struct{}{
		"/healthz":        {},
		"/api/version":    {},
		"/api/auth/login": {},
	}
	// mustChange=true 时仍允许访问的端点（已登录但需先改密）。
	allowedDuringMustChange := map[string]struct{}{
		"/api/auth/me":              {},
		"/api/auth/change-password": {},
		"/api/auth/logout":          {},
	}
	inSet := func(set map[string]struct{}, c *gin.Context) bool {
		if _, ok := set[c.FullPath()]; ok {
			return true
		}
		_, ok := set[c.Request.URL.Path]
		return ok
	}
	return func(c *gin.Context) {
		if inSet(whitelist, c) {
			c.Next()
			return
		}
		token := extractToken(c.Request)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		sub, mustChange, err := s.Verify(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		if mustChange && !inSet(allowedDuringMustChange, c) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":                "password change required",
				"must_change_password": true,
			})
			return
		}
		c.Set("authSubject", sub)
		c.Set("authMustChange", mustChange)
		c.Next()
	}
}

func extractToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	if t := r.URL.Query().Get("token"); t != "" {
		return t
	}
	if c, err := r.Cookie("uh_token"); err == nil {
		return c.Value
	}
	return ""
}
