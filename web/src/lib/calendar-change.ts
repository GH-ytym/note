export interface CalendarChange {
  group_id: number;
  kind: "todo" | "event";
  id: number;
  action: "upsert" | "remove";
}

export function parseCalendarChange(value: unknown): CalendarChange {
  const v = value as Partial<CalendarChange> | null;
  if (!v || !Number.isSafeInteger(v.group_id) || v.group_id! <= 0 ||
      !Number.isSafeInteger(v.id) || v.id! <= 0 ||
      (v.kind !== "todo" && v.kind !== "event") ||
      (v.action !== "upsert" && v.action !== "remove")) {
    throw new Error("日历变化无效");
  }
  return v as CalendarChange;
}
