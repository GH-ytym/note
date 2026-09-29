// 用于验证账密和邮箱的正确性
package auth

import (
	"net/mail"
	apperrors "note/internal/errors"
	"regexp"
	"strings"
)

var (
	namePattern     = regexp.MustCompile(`^[\p{Han}A-Za-z0-9_]{2,20}$`) //2-20位，汉字/英文字母/阿拉伯数字/下划线
	passwordPattern = regexp.MustCompile(`^[A-Za-z0-9_]{8,20}$`)        //8-20位，英文字母/阿拉伯数字/下划线
	accountPattern  = regexp.MustCompile(
		`^([\p{Han}A-Za-z0-9_]{2,20})#([1-9][0-9]{4})$`, //#xxxxx后缀
	)
)

func validateName(name string) error {
	if !namePattern.MatchString(name) {
		return apperrors.ErrInvalidName
	}
	switch strings.ToLower(name) {
	case "nil", "null", "undefined":
		return apperrors.ErrReservedName
	}
	return nil
}

func validatePassword(password string) error {
	if !passwordPattern.MatchString(password) {
		return apperrors.ErrInvalidPassword
	}
	return nil
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || len(email) > 254 {
		return "", apperrors.ErrInvalidEmail
	}

	//逐个字符检查
	for _, c := range email {
		if c <= 32 || c >= 127 {
			return "", apperrors.ErrInvalidEmail
		}
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", apperrors.ErrInvalidEmail
	}
	if strings.Count(email, "@") != 1 {
		return "", apperrors.ErrInvalidEmail
	}
	return email, nil
}
