package event

import (
	"context"
	apperrors "note/internal/errors"
	"note/internal/utils"
	"strings"

	"note/internal/model"
)

// Service declares the Event operations available to the HTTP layer.
type EventService interface {
	Create(ctx context.Context, command CreateCommand) (model.Event, error)
}

type service struct {
	repo EventRepository
}

func NewService(repo EventRepository) EventService {
	return &service{repo: repo}
}

func (s *service) Create(ctx context.Context, command CreateCommand) (model.Event, error) {
	title := strings.TrimSpace(command.Title)
	if title == "" {
		return model.Event{}, apperrors.ErrTitleRequired
	}

	var content *string
	if command.Content != nil {
		trimmedContent := strings.TrimSpace(*command.Content)
		if len([]rune(trimmedContent)) > 500 {
			return model.Event{}, apperrors.ErrEventInvalidContent
		}
		content = &trimmedContent
	}

	color := command.Color
	if color == "" {
		color = utils.RandomColor()
	}
	color1, valid := utils.NormalizeHexColor(color)
	if !valid {
		return model.Event{}, apperrors.ErrEventInvalidColor
	}

	//时间校验
	if command.StartsAt.IsZero() ||
		command.EndsAt.IsZero() ||
		!command.EndsAt.After(command.StartsAt) {
		return model.Event{}, apperrors.ErrInvalidEventTimeRange
	}

	item := model.Event{
		Title:    title,
		Content:  content,
		Color:    color1,
		StartsAt: command.StartsAt,
		EndsAt:   command.EndsAt,
	}
	if err := s.repo.Create(ctx, &item); err != nil {
		return model.Event{}, err
	}
	return item, nil
}
