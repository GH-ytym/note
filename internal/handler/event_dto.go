package handler

import "time"

type CreateEventRequest struct {
	Title    string     `json:"title" binding:"required,max=50"`
	Content  *string    `json:"content" binding:"omitempty,max=500"`
	Color    string     `json:"color" binding:"omitempty,len=7"`
	StartsAt *time.Time `json:"starts_at" binding:"required"`
	EndsAt   *time.Time `json:"ends_at" binding:"required"`
}
