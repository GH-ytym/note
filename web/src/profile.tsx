import { createContext, useCallback, useContext, useEffect, useRef, useState } from "react";
import type { ReactNode } from "react";
import type { Profile } from "./types";
import { myProfile, removeAvatar, uploadAvatar } from "./api";
import { onDataChanged, publishDataChanged } from "./data-events";

const ProfileContext = createContext<{ profile: Profile | null; error: string; reload: () => Promise<void>; upload: (file: File) => Promise<void>; remove: () => Promise<void> } | null>(null);
export function ProfileProvider({ children }: { children: ReactNode }) {
  const [profile, setProfile] = useState<Profile | null>(null);
  const [error, setError] = useState("");
  const generation = useRef(0);
  const reload = useCallback(async () => {
    const run = ++generation.current;
    try { const value = await myProfile(); if (run === generation.current) { setProfile(value); setError(""); } }
    catch (reason) { if (run === generation.current) setError(reason instanceof Error ? reason.message : "读取资料失败"); }
  }, []);
  useEffect(() => { void reload(); const remove = onDataChanged(value => { if (value.type === "profile") void reload(); }); return () => { generation.current++; remove(); }; }, [reload]);
  const changed = async (value: Profile) => { generation.current++; setProfile(value); setError(""); await publishDataChanged({ type: "profile" }); };
  return <ProfileContext.Provider value={{ profile, error, reload, upload: async file => changed(await uploadAvatar(file)), remove: async () => changed(await removeAvatar()) }}>{children}</ProfileContext.Provider>;
}
export function useProfile() { const value = useContext(ProfileContext); if (!value) throw new Error("missing profile provider"); return value; }
