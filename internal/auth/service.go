package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	apperrors "note/internal/errors"
	"note/internal/model"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Service interface {
	Register(
		ctx context.Context,
		name, email, password string,
	) (*model.User, error)

	Login(
		ctx context.Context,
		account, password string,
	) (*model.User, error)

	CurrUser(ctx context.Context, userID uint) (*model.User, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Register(
	ctx context.Context,
	name, email, password string,
) (*model.User, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}

	if err := validatePassword(password); err != nil {
		return nil, err
	}

	email, err := normalizeEmail(email)
	if err != nil {
		return nil, err
	}

	// 先检查，便于尽早给出错误。
	// 真正避免并发重复注册，仍依靠数据库的唯一索引。
	_, err = s.repo.FindByEmail(ctx, email)
	if err == nil {
		return nil, apperrors.ErrEmailTaken
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("查询邮箱失败: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return nil, fmt.Errorf("生成密码哈希失败: %w", err)
	}

	// 先生成 15 个互不重复的候选后缀，再依次尝试入库。
	suffixes, err := generateSuffixes(ctx)
	if err != nil {
		return nil, err
	}
	for _, suffix := range suffixes {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		user := &model.User{
			Username:     name,
			Suffix:       suffix,
			Email:        email,
			PasswordHash: string(hash),
			Nickname:     name,
		}
		err := s.repo.Create(ctx, user)
		if err == nil {
			return user, nil
		}
		//ErrAccountTaken是重复创建错误，如果真是这个错才会进入下一个循环
		if !errors.Is(err, apperrors.ErrAccountTaken) {
			return nil, fmt.Errorf("创建用户失败: %w", err)
		}
	}
	return nil, apperrors.ErrSuffixUnavailable
}

// 数组内不重复；是否与数据库已有账号冲突仍由唯一索引判断。
func generateSuffixes(ctx context.Context) ([15]int, error) {
	var suffixes [15]int
	seen := make(map[int]bool, len(suffixes))
	for i := 0; i < len(suffixes); {
		if err := ctx.Err(); err != nil {
			return suffixes, err
		}
		number, err := rand.Int(rand.Reader, big.NewInt(90000))
		if err != nil {
			return suffixes, fmt.Errorf("生成账号后缀失败: %w", err)
		}
		suffix := int(number.Int64()) + 10000
		if seen[suffix] {
			continue
		}
		seen[suffix] = true
		suffixes[i] = suffix
		i++
	}
	return suffixes, nil
}

func (s *service) Login(
	ctx context.Context,
	account, password string,
) (*model.User, error) {
	account = strings.TrimSpace(account)

	if validatePassword(password) != nil {
		return nil, apperrors.ErrInvalidCredentials
	}

	var user *model.User
	var err error

	if strings.Contains(account, "@") {
		// 邮箱登录。
		email, parseErr := normalizeEmail(account)
		if parseErr != nil {
			return nil, apperrors.ErrInvalidCredentials
		}

		user, err = s.repo.FindByEmail(ctx, email)
	} else {
		// 完整账号登录，例如：小明#12345。
		parts := accountPattern.FindStringSubmatch(account)
		if parts == nil {
			return nil, apperrors.ErrInvalidCredentials
		}

		// parts[0] = "小明#12345"
		// parts[1] = "小明"
		// parts[2] = "12345"
		suffix, parseErr := strconv.Atoi(parts[2])
		if parseErr != nil {
			return nil, apperrors.ErrInvalidCredentials
		}

		user, err = s.repo.FindByAccount(ctx, parts[1], suffix)
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperrors.ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(password),
	)
	if err != nil {
		return nil, apperrors.ErrInvalidCredentials
	}

	return user, nil
}

func (s *service) CurrUser(
	ctx context.Context,
	userID uint,
) (*model.User, error) {
	if userID == 0 {
		return nil, ErrInvalidRefresh
	}

	user, err := s.repo.FindByID(ctx, userID)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrInvalidRefresh
	}
	if err != nil {
		return nil, fmt.Errorf("查询当前用户失败: %w", err)
	}

	return user, nil
}
