import type { View, DayStyle, RepeatMode } from "./types";
import type { Settings } from "./lib/appearance";

export interface WorkspaceState { view: View; date: string; dayStyle: DayStyle; handMode: "full" | "compact"; mini: boolean }
export interface PickerState { sessionId?: number; field: string; repeatMode: RepeatMode; date: string; customDates: string[]; color: string }
export interface EditorState { todoId: number; date: string; content: string | null; version: number }
export interface EditorResult { todoId: number; content: string | null; version: number }
export interface ReminderState { todoId: number; title: string; content: string; color: string; occursAt: string; date: string; notifyMode: string }
export interface DataChange { type: string; todoId?: number; eventId?: number; date?: string }
type Listener<T> = (callback: (value: T) => void) => () => void;
interface NoteDesktop {
  isDesktop: boolean;
  startWindowDrag(point: { x: number; y: number }): Promise<void>;
  moveWindowDrag(point: { x: number; y: number }): Promise<void>;
  endWindowDrag(): Promise<void>;
  getWorkspaceState(): Promise<WorkspaceState>;
  saveWorkspaceState(payload: WorkspaceState): Promise<void>;
  setMiniMode(enabled: boolean): Promise<void>;
  fitMiniWindow(payload: { size: number }): Promise<void>;
  claimInlinePanel(): Promise<void>;
  onAuxiliaryOpened: Listener<{ role: string }>;
  toggleCalendar(): Promise<{ open: boolean }>;
  getCalendarVisibility(): Promise<{ open: boolean }>;
  openCreate(payload: { date: string }): Promise<void>;
  openDetail(payload: { todoId?: number; eventId?: number; date: string }): Promise<void>;
  openSettings(): Promise<void>;
  openContentEditor(payload: EditorState): Promise<void>;
  getContentEditorState(): Promise<EditorState | null>;
  getReminderState(): Promise<ReminderState | null>;
  finishContentEditor(payload: EditorResult): Promise<void>;
  getAppearance(): Promise<Settings>;
  updateAppearance(payload: Settings): Promise<Settings>;
  closeCurrent(): Promise<void>;
  notifyDataChanged(payload: DataChange): Promise<void>;
  startDatePicker(payload: Omit<PickerState, "sessionId">): Promise<void>;
  updateDatePicker(payload: Omit<PickerState, "sessionId">): Promise<void>;
  selectDatePicker(payload: { date: string; customDates: string[] }): Promise<void>;
  finishDatePicker(): Promise<void>;
  getDatePickerState(): Promise<PickerState | null>;
  onDataChanged: Listener<DataChange>;
  onContentEditorSaved: Listener<EditorResult>;
  onAppearanceChanged: Listener<Settings>;
  onWorkspaceViewChanged: Listener<WorkspaceState>;
  onDatePickerStateChanged: Listener<PickerState | null>;
  onDatePickerSelection: Listener<PickerState>;
  onDatePickerFinished(callback: () => void): () => void;
}
declare global { interface Window { noteDesktop?: NoteDesktop } }
declare module "react" { interface CSSProperties { [key: `--${string}`]: string | number | undefined } }
