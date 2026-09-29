package handler

import "time"

type CreateGroupRequest struct {
	Name string `json:"name" binding:"required,max=80"`
}

type GroupResponse struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	OwnerID   uint      `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}
