package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// claims represents the JWT claims used in the token. It includes the user ID and standard registered claims.
type Claims struct {
	UserID uint `json:"user_id"`
	jwt.RegisteredClaims
}

// 服务端负责签发、验证 Token 的对象。当前应用启动时创建一个实例，所有用户共用它的密钥和有效期配置；每个用户拿到的 Token 则包含各自的 UserID
type TokenManager struct {
	// secret is the secret key used to sign the JWT tokens. It should be at least 32 characters long for security reasons.
	secret []byte
	// ttl is the time-to-live duration for the tokens. It determines how long a token is valid before it expires.
	ttl time.Duration
}

func NewTokenManager(secret string, ttl time.Duration) (*TokenManager, error) {
	if len(secret) < 32 {
		return nil, errors.New("secret must be at least 32 characters long")
	}
	if ttl <= 0 {
		return nil, errors.New("ttl must be greater than 0")
	}
	return &TokenManager{
		secret: []byte(secret),
		ttl:    ttl,
	}, nil
}

func (m *TokenManager) Generate(userID uint) (string, error) {
	if userID == 0 {
		return "", errors.New("userID must be greater than 0")
	}
	now := time.Now()
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "note",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		},
	}
	//此时token是一个JWT对象，包含了用户ID和注册的声明信息
	// 接下来，我们使用HMAC SHA256算法对token进行签名，并返回签名后的字符串。
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	//这个才是最终的JWT字符串，它包含了头部、载荷和签名部分。这个字符串可以在客户端和服务器之间传递，用于身份验证和授权。
	//一般为三段式xxx.yyy.zzz，分别是header、payload和signature
	//header和payload是base64编码的JSON字符串
	//signature是对前两部分进行签名后的结果。
	tokenStr, err := token.SignedString(m.secret)
	if err != nil {
		return "", err
	}
	return tokenStr, nil
}

// 验证前端传回来的token字符串是否合法，合法的话返回数据
// Access JWT 由客户端保存，本方法只校验签名和声明，不查询 Redis。
// Redis 管理刷新会话；撤销刷新会话不会立即使已签发的 JWT 失效。
func (m *TokenManager) Parse(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(
		tokenStr,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			return m.secret, nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer("note"),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)
	if err != nil {
		return nil, err
	}

	if !token.Valid || claims.UserID == 0 {
		return nil, errors.New("无效的 JWT")
	}

	// 只有验证成功，调用方才能信任其中的 UserID。
	return claims, nil
}
