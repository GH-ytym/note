package handler

import "note/internal/todo"

// TodoHandler translates Todo HTTP requests into Todo service calls.
type TodoHandler struct {
	service todo.Service
}

func NewTodoHandler(service todo.Service) *TodoHandler {
	return &TodoHandler{service: service}
}
