package handler

// LoginRequest 只接收账密，不直接绑定数据库中的 User。
type LoginRequest struct {
	Account  string `json:"account" binding:"required,max=254"`
	Password string `json:"password" binding:"required,max=20"`
}

// LoginResponse 明确列出允许返回的字段，避免泄露密码哈希。
type LoginResponse struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Suffix      int    `json:"suffix"`
	Account     string `json:"account"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type RegisterRequest struct {
	Name     string `json:"name" binding:"required,max=20"`
	Email    string `json:"email" binding:"required,max=254"`
	Password string `json:"password" binding:"required,max=20"`
}

type RegisterResponse struct {
	ID      uint   `json:"id"`
	Name    string `json:"name"`
	Suffix  int    `json:"suffix"`
	Account string `json:"account"`
}
