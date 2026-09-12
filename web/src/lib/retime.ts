import { getEvent, getTodo, patchEvent, patchTodo } from "../api";
import type { CalendarItem } from "../types";
import { shanghaiDateTimeParts } from "./calendar";
import { notifyDataChanged } from "../windows/window-utils";

// A drag changes the series anchor, never an individual occurrence.
export async function retimeItem(item: CalendarItem, edge: "start" | "end", minutes: number) {
  const isEvent = item.kind === "event";
  const id = isEvent ? item.eventId : item.todoId;
  if (!id) throw new Error("记录编号无效");
  const record = await (isEvent
    ? getEvent(id)
    : getTodo(id));
  if (record.version !== item.version)
    throw new Error("记录已更新，请刷新后再拖动");
  const field = isEvent && edge === "end" ? "ends_at" : "starts_at";
  const value = new Date(
    new Date(field === "ends_at" && "ends_at" in record ? record.ends_at : record.starts_at).getTime() + minutes * 60000,
  ).toISOString();
  if (
    record.repeat_mode === "custom" &&
    field === "starts_at" &&
    shanghaiDateTimeParts(value).date !==
      shanghaiDateTimeParts(record.starts_at).date
  ) {
    throw new Error("自定义周期暂不支持拖动开始时间跨过午夜");
  }
  if (isEvent && "ends_at" in record) {
    const start = field === "starts_at" ? value : record.starts_at;
    const end = field === "ends_at" ? value : record.ends_at;
    if (new Date(end) <= new Date(start))
      throw new Error("结束时间必须晚于开始时间");
    await patchEvent(id, { [field]: value, version: record.version });
  } else {
    await patchTodo(id, { starts_at: value, version: record.version });
  }
  await notifyDataChanged({
    type: "updated",
    eventId: item.eventId,
    todoId: item.todoId,
  });
}
