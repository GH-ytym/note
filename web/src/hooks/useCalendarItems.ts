import type { CalendarItem } from "../types";
import { useCallback, useEffect, useRef, useState } from "react";
import { getCalendar } from "../api";
import { calendarItemsFromResponse } from "../lib/calendar";
import { createCalendarSync } from "../lib/calendar-sync";
import { onCalendarSync } from "../calendar-events";
import { onDataChanged } from "../data-events";

export default function useCalendarItems(from: string, to: string, groupID?: number | null, active = true) {
  const [items, setItems] = useState<CalendarItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const current = useRef<ReturnType<typeof createCalendarSync<CalendarItem[]>> | null>(null);
  const activeRef = useRef(active);
  activeRef.current = active;
  const refresh = useCallback(() => current.current?.request() ?? Promise.resolve(), []);

  useEffect(() => {
    setItems([]);
    setError("");
    setLoading(groupID !== null);
    if (groupID === null) return;

    const sync = createCalendarSync({
      load: async signal => calendarItemsFromResponse(await getCalendar(
        from, to, groupID, AbortSignal.any([signal, AbortSignal.timeout(15_000)]),
      )),
      apply: result => { setItems(result); setError(""); },
      fail: reason => setError(reason instanceof Error ? reason.message : "读取日历失败"),
      settled: () => setLoading(false),
    });
    current.current = sync;
    const visible = () => {
      sync.setActive(activeRef.current && document.visibilityState === "visible" && navigator.onLine);
    };
    const wake = () => {
      visible();
      if (activeRef.current && document.visibilityState === "visible" && navigator.onLine) void sync.request();
    };
    const removeData = onDataChanged(() => void sync.request());
    const removeSync = onCalendarSync(change => {
      if (!change || groupID === undefined || change.group_id === groupID) void sync.request();
    });
    document.addEventListener("visibilitychange", visible);
    window.addEventListener("focus", wake);
    window.addEventListener("online", wake);
    window.addEventListener("offline", visible);
    visible();
    return () => {
      sync.stop();
      if (current.current === sync) current.current = null;
      removeData(); removeSync();
      document.removeEventListener("visibilitychange", visible);
      window.removeEventListener("focus", wake);
      window.removeEventListener("online", wake);
      window.removeEventListener("offline", visible);
    };
  }, [from, to, groupID]);

  useEffect(() => {
    current.current?.setActive(active && document.visibilityState === "visible" && navigator.onLine);
  }, [active]);
  return { items, loading, error, refresh };
}
