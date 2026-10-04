import type { CalendarItem } from "../types";
import { useCallback, useEffect, useRef, useState } from "react";
import { getCalendar } from "../api";
import { calendarItemsFromResponse } from "../lib/calendar";
import { onDataChanged } from "../data-events";

export default function useCalendarItems(from: string, to: string, groupID?: number | null) {
  const [items, setItems] = useState<CalendarItem[]>([]),
    [loading, setLoading] = useState(true),
    [error, setError] = useState("");
  const generation = useRef(0);
  const refresh = useCallback(async () => {
    const id = ++generation.current;
    setLoading(true);
    setError("");
    if (groupID === null) { setItems([]); setLoading(false); return; }
    try {
      const result = calendarItemsFromResponse(await getCalendar(from, to, groupID));
      if (id === generation.current) setItems(result);
    } catch (error) {
      if (id === generation.current) setError(error instanceof Error ? error.message : "读取失败");
    } finally {
      if (id === generation.current) setLoading(false);
    }
  }, [from, to, groupID]);
  useEffect(() => {
    void refresh();
    const unsubscribe = onDataChanged(() => void refresh());
    return () => {
      ++generation.current;
      unsubscribe?.();
    };
  }, [refresh]);
  useEffect(() => { setItems([]); }, [groupID]);
  return { items, loading, error, refresh };
}
