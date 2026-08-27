package handler

import "note/internal/todo"

// Handler负责http处理
type Handler struct {
	todoService todo.TodoService
}

func NewHandler(todoService todo.TodoService) *Handler {
	return &Handler{todoService: todoService}
}
