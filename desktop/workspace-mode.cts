function isChoice<T extends string>(value: unknown, choices: T[]): value is T { return typeof value === "string" && choices.includes(value as T); }
import type { Settings, WorkspaceState } from "./contracts.cjs";
const WAKE_STYLES: Settings["wakeStyle"][] = ["mini", "last", "year", "month", "week", "timeline", "clock"];

function normalizeWorkspace(value: Partial<Record<keyof WorkspaceState, unknown>> = {}, today: string, settings: Partial<Settings> = {}): WorkspaceState {
  const mini = Boolean(value.mini);
  return {
    mini,
    view: mini ? "day" : isChoice(value.view, ["year", "month", "week", "day"]) ? value.view : settings.defaultView || "month",
    dayStyle: mini ? settings.miniViewMode || "tags" : isChoice(value.dayStyle, ["clock", "timeline", "tags"]) ? value.dayStyle : settings.dayViewMode || "tags",
    handMode: isChoice(value.handMode, ["full", "compact"]) ? value.handMode : settings.handMode || "full",
    date: mini ? today : typeof value.date === "string" && /^\d{4}-\d{2}-\d{2}$/.test(value.date) ? value.date : today,
  };
}
function wakeWorkspace(style: string, last: WorkspaceState | null, today: string, settings: Partial<Settings>) {
  if (style === "last" && last) return normalizeWorkspace({ ...last }, today, settings);
  if (style === "mini" || !WAKE_STYLES.some(value => value === style)) return normalizeWorkspace({ mini: true }, today, settings);
  if (style === "clock" || style === "timeline") return normalizeWorkspace({ view: "day", dayStyle: style }, today, settings);
  return normalizeWorkspace({ view: style }, today, settings);
}
export { WAKE_STYLES, normalizeWorkspace, wakeWorkspace };
