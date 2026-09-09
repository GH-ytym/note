package handler

import (
	"time"

	"note/internal/model"
)

type CreateEventRequest struct {
	Title       string           `json:"title" binding:"required,max=50"`
	Content     *string          `json:"content" binding:"omitempty,max=500"`
	Color       string           `json:"color" binding:"omitempty,len=7"`
	StartsAt    *time.Time       `json:"starts_at" binding:"required"`
	EndsAt      *time.Time       `json:"ends_at" binding:"required"`
	RepeatMode  model.RepeatMode `json:"repeat_mode" binding:"required,oneof=once daily weekdays weekends weekly monthly custom"`
	CustomDates []string         `json:"custom_dates"`
}
