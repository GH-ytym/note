import type { Todo, TodoDetail, OccurrenceCompletions, CalendarEvent, CreateTodo, CreateEvent, TodoChanges, EventChanges, CalendarResponse, SearchPage, Group, GroupMember, Profile } from "./types";
import { auth } from './auth';
export { APIError } from './auth';
const request = <T>(path: string, options: RequestInit = {}): Promise<T> => auth.request<T>(path, options);

export function listTodos(groupID: number, page: number, pageSize: number) {
  const params = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  });
  return request<{ data: Todo[]; page: number; page_size: number; total: number }>(`/groups/${groupID}/todos?${params}`);
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

export function getCalendar(from: string, to: string, groupID?: number) {
  const params = new URLSearchParams({ from, to });
  if (groupID) params.set("group_id", String(groupID));
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

export const deleteEvent = (id: number) => request<null>(`/events/${id}`, { method: "DELETE" });
export const myGroups = () => request<Group[]>("/groups");
export const createGroup = (name: string) => request<Group>("/groups", { method: "POST", body: JSON.stringify({ name }) });
export const joinGroup = (id: number, code: string) => request<null>(`/groups/${id}/join`, { method: "POST", body: JSON.stringify({ code }) });
export const groupMembers = (id: number) => request<GroupMember[]>(`/groups/${id}/members`);
export const groupInvite = (id: number) => request<{group_id: number; code: string}>(`/groups/${id}/invite`);
export const refreshGroupInvite = (id: number) => request<{group_id: number; code: string}>(`/groups/${id}/refresh`, { method: "POST" });
export const quitGroup = (id: number, target?: number) => request<null>(`/groups/${id}/quit`, { method: "POST", body: JSON.stringify(target ? { target } : {}) });
export const dismissGroup = (id: number) => request<null>(`/groups/${id}/dismiss`, { method: "POST" });
export const patchMemberRoles = (kind: "todo" | "event", id: number, userIDs: number[], role: 1 | 2) => request<null>(`/${kind === "todo" ? "todos" : "events"}/${id}/members`, { method: "PATCH", body: JSON.stringify({ user_ids: userIDs, role }) });
export const myProfile = () => request<Profile>("/users/me");
export const uploadAvatar = (file: File) => {
  const body = new FormData(); body.append("avatar", file);
  return request<Profile>("/users/me/avatar", { method: "PUT", body });
};
export const removeAvatar = () => request<Profile>("/users/me/avatar", { method: "DELETE" });

function search(groupID: number, kind: "todos" | "events" | "all", keyword: string, pageSize: number, page = 1, signal?: AbortSignal) {
  const params = new URLSearchParams({ keyword, page: String(page), page_size: String(pageSize) });
  return request<{ page: SearchPage }>(`/groups/${groupID}/search/${kind}?${params}`, { signal }).then(result => result.page);
}
export const searchTodo = (groupID: number, keyword: string, limit: number, page = 1, signal?: AbortSignal) => search(groupID, "todos", keyword, limit, page, signal);
export const searchEvent = (groupID: number, keyword: string, limit: number, page = 1, signal?: AbortSignal) => search(groupID, "events", keyword, limit, page, signal);
export const searchAll = (groupID: number, keyword: string, limit: number, page = 1, signal?: AbortSignal) => search(groupID, "all", keyword, limit, page, signal);
