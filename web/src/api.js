const API_BASE = "/api";

export class APIError extends Error {
  constructor(message, status) {
    super(message);
    this.name = "APIError";
    this.status = status;
  }
}

async function request(path, options = {}) {
  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      ...(options.body ? { "Content-Type": "application/json" } : {}),
      ...options.headers,
    },
  });

  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    throw new APIError(body.error || "请求失败，请稍后再试", response.status);
  }

  if (response.status === 204) {
    return null;
  }

  return response.json();
}

export function listTodos(page, pageSize) {
  const params = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  });
  return request(`/todos?${params}`);
}

export function createTodo(todo) {
  return request("/todos", {
    method: "POST",
    body: JSON.stringify(todo),
  });
}

export function getCalendar(from, to) {
  const params = new URLSearchParams({ from, to });
  return request(`/calendar?${params}`);
}

export function patchTodo(id, changes) {
  return request(`/todos/${id}`, {
    method: "PATCH",
    body: JSON.stringify(changes),
  });
}

export function deleteTodo(id) {
  return request(`/todos/${id}`, { method: "DELETE" });
}
