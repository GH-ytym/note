import { useCallback, useEffect, useRef, useState } from "react";
import { getCalendar } from "../api";
import { calendarItemsFromResponse } from "../lib/calendar";

export default function useCalendarItems(from, to) {
  const [items, setItems] = useState([]),
    [loading, setLoading] = useState(true),
    [error, setError] = useState("");
  const generation = useRef(0);
  const refresh = useCallback(async () => {
    const id = ++generation.current;
    setLoading(true);
    setError("");
    try {
      const result = calendarItemsFromResponse(await getCalendar(from, to));
      if (id === generation.current) setItems(result);
    } catch (error) {
      if (id === generation.current) setError(error.message);
    } finally {
      if (id === generation.current) setLoading(false);
    }
  }, [from, to]);
  useEffect(() => {
    void refresh();
    const unsubscribe = window.noteDesktop?.onDataChanged?.(refresh);
    return () => {
      ++generation.current;
      unsubscribe?.();
    };
  }, [refresh]);
  return { items, loading, error, refresh };
}
