import type { BrowserWindow, Rectangle } from "electron";
import type { EditorState, PickerState, ReminderState } from "./contracts.cjs";

export type WindowRole = "calendar" | "create" | "detail" | "reminder" | "settings" | "content-editor";
export interface EditorSession extends EditorState { sourceWindow: BrowserWindow }
export interface PickerSession { id: number; sourceWindow: BrowserWindow; sourceKey: string; state: PickerState }
export interface WindowOptions {
  key: string; role: WindowRole; title: string;
  query?: Record<string, string | number>;
  parent?: BrowserWindow | null; modal?: boolean;
  contentEditorState?: EditorSession | null; reminderState?: ReminderState | null;
  alwaysOnTop?: boolean; maximizable?: boolean;
}
declare global {
  namespace Electron {
    interface BrowserWindow {
      noteWindowKey: string;
      noteWindowRole: WindowRole;
      noteContentEditorState?: EditorSession | null;
      noteReminderState?: ReminderState | null;
      noteReplaced?: boolean;
      noteDate?: string;
      noteMini?: boolean;
      noteNormalBounds?: Rectangle;
    }
  }
}
