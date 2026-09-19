export type View = "year" | "month" | "week" | "day";
export type DayStyle = "clock" | "timeline" | "tags";
export type RepeatMode = "once" | "daily" | "weekdays" | "weekends" | "weekly" | "monthly" | "custom";
export interface Settings {
  backgroundColor: string; themeColor: string; opacity: number;
  defaultView: View; dayViewMode: DayStyle; miniViewMode: DayStyle;
  searchSplit: boolean; searchLimit: number; handMode: "full" | "compact";
  weekOrientation: "vertical" | "horizontal"; dayOrientation: "vertical" | "horizontal";
  clockTracks: number; wakeStyle: "mini" | "last" | "year" | "month" | "week" | "timeline" | "clock";
}
export interface WorkspaceState { view: View; date: string; dayStyle: DayStyle; handMode: "full" | "compact"; mini: boolean }
export interface PickerState { sessionId?: number; field: string; repeatMode: RepeatMode; date: string; customDates: string[]; color: string }
export interface EditorState { todoId: number; date: string; content: string | null; version: number }
export interface EditorResult { todoId: number; content: string | null; version: number }
export interface ReminderState { todoId: number; title: string; content: string; color: string; occursAt: string; date: string; notifyMode: string }
export interface DataChange { type: string; todoId?: number; eventId?: number; date?: string }
export interface WindowResult { key: string }
type Listener<T> = (callback: (value: T) => void) => () => void;
export interface NoteDesktop {
  isDesktop: boolean;
  openCompose(payload: { date: string }): Promise<{ calendar: WindowResult | null; create: WindowResult }>;
  openDay(payload: { date: string }): Promise<WindowResult>;
  minimizeCurrent(): Promise<void>;
  toggleMaximizeCurrent(): Promise<boolean>;
  getWindowState(): Promise<{ maximized: boolean }>;
  fitDayWindow(payload: { height: number }): Promise<null>;
  cancelDatePicker(): Promise<void>;
  onDayDateChanged: Listener<{ date: string }>;
  onCalendarVisibilityChanged: Listener<{ open: boolean }>;
  onMaximizedChanged: Listener<boolean>;
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
  openCreate(payload: { date: string }): Promise<WindowResult>;
  openDetail(payload: { todoId?: number; eventId?: number; date: string }): Promise<WindowResult>;
  openSettings(): Promise<WindowResult>;
  openContentEditor(payload: EditorState): Promise<WindowResult>;
  getContentEditorState(): Promise<EditorState | null>;
  getReminderState(): Promise<ReminderState | null>;
  finishContentEditor(payload: EditorResult): Promise<EditorResult>;
  getAppearance(): Promise<Settings>;
  updateAppearance(payload: Settings): Promise<Settings>;
  closeCurrent(): Promise<void>;
  notifyDataChanged(payload: DataChange): Promise<void>;
  startDatePicker(payload: Omit<PickerState, "sessionId">): Promise<PickerState | null>;
  updateDatePicker(payload: Omit<PickerState, "sessionId">): Promise<PickerState | null>;
  selectDatePicker(payload: { date: string; customDates: string[] }): Promise<PickerState | null>;
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
