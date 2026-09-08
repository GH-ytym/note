package handler

import "note/internal/todo"

// TodoHandler translates Todo HTTP requests into Todo service calls.
type TodoHandler struct {
	service todo.TodoService
}

func NewTodoHandler(service todo.TodoService) *TodoHandler {
	return &TodoHandler{service: service}
}
