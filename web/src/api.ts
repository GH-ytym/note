import type { Todo, TodoDetail, OccurrenceCompletions, CalendarEvent, CreateTodo, CreateEvent, TodoChanges, EventChanges, CalendarResponse, SearchPage } from "./types";
import { auth } from './auth';
export { APIError } from './auth';
const request = <T>(path: string, options: RequestInit = {}): Promise<T> => auth.request<T>(path, options);

export function listTodos(page: number, pageSize: number) {
  const params = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  });
  return request<{ data: Todo[]; page: number; page_size: number; total: number }>(`/todos?${params}`);
}

export function createTodo(todo: CreateTodo) {
  return request<Todo>("/todos", {
    method: "POST",
    body: JSON.stringify(todo),
  });
}

export function createEvent(event: CreateEvent) {
  return request<CalendarEvent>("/events", {
    method: "POST",
    body: JSON.stringify(event),
  });
}

export function getEvent(id: number) { return request<CalendarEvent>(`/events/${id}`); }
export function patchEvent(id: number, changes: EventChanges) { return request<CalendarEvent>(`/events/${id}`, { method: "PATCH", body: JSON.stringify(changes) }); }

export function getTodo(id: number) {
  return request<TodoDetail>(`/todos/${id}`);
}

export function getCalendar(from: string, to: string) {
  const params = new URLSearchParams({ from, to });
  return request<CalendarResponse>(`/calendar?${params}`);
}

export function patchTodo(id: number, changes: TodoChanges) {
  return request<Todo>(`/todos/${id}`, {
    method: "PATCH",
    body: JSON.stringify(changes),
  });
}

export function patchOccurrence(todoId: number, date: string, done: boolean) {
  return request<null>(`/todos/${todoId}/occurrences/${encodeURIComponent(date)}`, {
    method: "PATCH",
    body: JSON.stringify({ done }),
  });
}

export function getOccurrenceCompletions(todoId: number, date: string) {
  return request<OccurrenceCompletions>(`/todos/${todoId}/occurrences/${encodeURIComponent(date)}/completions`);
}

export function deleteTodo(id: number) {
  return request<null>(`/todos/${id}`, { method: "DELETE" });
}

function search(kind: "todos" | "events" | "all", keyword: string, pageSize: number, page = 1, signal?: AbortSignal) {
  const params = new URLSearchParams({ keyword, page: String(page), page_size: String(pageSize) });
  return request<{ page: SearchPage }>(`/search/${kind}?${params}`, { signal }).then(result => result.page);
}
export const searchTodo = (keyword: string, limit: number, page = 1, signal?: AbortSignal) => search("todos", keyword, limit, page, signal);
export const searchEvent = (keyword: string, limit: number, page = 1, signal?: AbortSignal) => search("events", keyword, limit, page, signal);
export const searchAll = (keyword: string, limit: number, page = 1, signal?: AbortSignal) => search("all", keyword, limit, page, signal);
