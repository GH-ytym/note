import type { NoteDesktop } from "../../desktop/contracts.cjs";
export type { WorkspaceState, PickerState, EditorState, EditorResult, ReminderState, DataChange } from "../../desktop/contracts.cjs";
declare global { interface Window { noteDesktop?: NoteDesktop } }
declare module "react" { interface CSSProperties { [key: `--${string}`]: string | number | undefined } }
