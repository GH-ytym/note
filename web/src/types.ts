export type View = "year" | "month" | "week" | "day";
export type DayStyle = "timeline" | "clock" | "tags";
export type RepeatMode = "once" | "daily" | "weekdays" | "weekends" | "weekly" | "monthly" | "custom";
export type NotifyMode = "none" | "silent" | "popup";

export interface ScheduleRecord {
  id: number;
  title: string;
  content: string | null;
  color: string;
  starts_at: string;
  repeat_mode: RepeatMode;
  custom_dates?: { date: string }[];
  version: number;
  created_at: string;
  updated_at: string;
}
export interface Todo extends ScheduleRecord { notify_mode: NotifyMode }
export interface UserSummary {
  id: number;
  username: string;
  suffix: number;
  nickname: string;
  avatar: string;
}
export interface TodoDetail extends Todo {
  group_id: number;
  creator_id: number;
  creator: UserSummary | null;
  my_role: MemberRole;
  member_roles: MemberPermission[];
}
export type MemberRole = "viewer" | "editor";
export interface MemberPermission { user_id: number; role: MemberRole }
export interface Group { id: number; name: string; owner_id: number; created_at: string }
export interface GroupMember extends UserSummary { joined_at: string }
export interface Profile extends UserSummary { avatar_upload_enabled: boolean }
export interface CompletedUser extends UserSummary { completed_at: string }
export interface OccurrenceCompletions {
  todo_id: number;
  completed_count: number;
  users: CompletedUser[];
}
export interface CalendarEvent extends ScheduleRecord {
  ends_at: string; group_id: number; creator_id: number; creator: UserSummary | null;
  my_role: MemberRole; member_roles: MemberPermission[];
}
export interface CreateSchedule {
  group_id: number;
  title: string;
  starts_at: string;
  repeat_mode: RepeatMode;
  content?: string;
  color?: string;
  custom_dates?: string[];
}
export interface CreateTodo extends CreateSchedule { notify_mode?: NotifyMode }
export interface CreateEvent extends CreateSchedule { ends_at: string }
export type TodoChanges = Partial<Omit<CreateTodo, "group_id">> & { version: number };
export type EventChanges = Partial<Pick<CreateEvent, "title" | "content" | "starts_at" | "ends_at">> & { version: number };

export interface SearchItem {
  kind: "todo" | "event";
  id: number;
  title: string;
  content: string | null;
  color: string;
  starts_at: string;
  ends_at: string | null;
  updated_at: string;
  score: number;
}
export interface SearchPage { items: SearchItem[]; total: number; page: number; page_size: number }

interface Occurrence {
  group_id: number; creator_id: number; my_role: MemberRole;
  title: string; content: string; color: string; starts_at: string;
  repeat_mode: RepeatMode; version: number;
}
export interface TodoOccurrence extends Occurrence {
  todo_id: number; occurs_at: string; notify_mode: NotifyMode;
  occurrence_done: boolean;
}
export interface EventOccurrence extends Occurrence { event_id: number; ends_at: string }
export interface CalendarResponse {
  data: { todos: TodoOccurrence[]; events: EventOccurrence[] };
  from: string; to: string;
}

export interface CalendarItem {
  groupId?: number; creatorId?: number; canEdit?: boolean;
  track?: number;
  id: string; kind: "todo" | "event"; title: string; content: string; color: string;
  date: string; time: string; version: number; repeat: string;
  todoId?: number; eventId?: number; startsAt?: string; endsAt?: string;
  startDate?: string; startTime?: string; endDate?: string; endTime?: string;
  occurrenceDone?: boolean; reminder?: string;
}
export interface ScheduleForm {
  title: string; content: string; repeat: string; date: string; time: string;
  reminder: string; color: string; endDate?: string; endTime?: string;
}
