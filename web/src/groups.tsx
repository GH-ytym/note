import { createContext, useCallback, useContext, useEffect, useRef, useState } from "react";
import type { ReactNode } from "react";
import type { Group } from "./types";
import { myGroups } from "./api";
import { auth } from "./auth";
import { onDataChanged } from "./data-events";

const GroupsContext = createContext<{ groups: Group[]; selected: Group | null; loading: boolean; error: string; select: (id: number) => void; reload: () => Promise<void> } | null>(null);
export function GroupsProvider({ children }: { children: ReactNode }) {
  const key = `note.group.${auth.user?.id}`;
  const [groups, setGroups] = useState<Group[]>([]);
  const [selectedID, setSelectedID] = useState(() => { try { return Number(localStorage.getItem(key)); } catch { return 0; } });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const generation = useRef(0);
  const select = useCallback((id: number) => { setSelectedID(id); try { localStorage.setItem(key, String(id)); } catch { /* 此窗口仍可切换 */ } }, [key]);
  const reload = useCallback(async () => {
    const run = ++generation.current; setError(""); setLoading(true);
    try { const result = await myGroups(); if (run === generation.current) { setGroups(result); setSelectedID(current => result.some(g => g.id === current) ? current : result[0]?.id || 0); } }
    catch (reason) { if (run === generation.current) setError(reason instanceof Error ? reason.message : "读取群组失败"); }
    finally { if (run === generation.current) setLoading(false); }
  }, []);
  useEffect(() => { void reload(); const remove = onDataChanged(value => { if (value.type === "groups") void reload(); }); return () => { generation.current++; remove(); }; }, [reload]);
  useEffect(() => { try { localStorage.setItem(key, String(selectedID)); } catch { /* 可禁用持久化 */ } }, [key, selectedID]);
  useEffect(() => {
    const receive = (event: StorageEvent) => { if (event.key === key && event.newValue) setSelectedID(Number(event.newValue)); };
    window.addEventListener("storage", receive); return () => window.removeEventListener("storage", receive);
  }, [key]);
  return <GroupsContext.Provider value={{ groups, selected: groups.find(group => group.id === selectedID) || null, loading, error, select, reload }}>{children}</GroupsContext.Provider>;
}
export function useGroups() { const value = useContext(GroupsContext); if (!value) throw new Error("missing groups provider"); return value; }
