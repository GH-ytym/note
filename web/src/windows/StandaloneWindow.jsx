import { TODAY_KEY, validDate } from "../lib/calendar";
import CreateView from "../views/CreateView";
import DetailView from "../views/DetailView";
import SettingsView from "../views/SettingsView";
import ContentEditorWindow from "./ContentEditorWindow";
import DayWorkspaceWindow from "./DayWorkspaceWindow";
import ReminderWindow from "./ReminderWindow";
import { LoadingWindow } from "./WindowFrame";
import { windowParams } from "./window-utils";

export default function StandaloneWindow({ role }) {
  const params = windowParams();
  if (role === "create") return <CreateView initialDate={validDate(params.get("date") || TODAY_KEY)} />;
  if (role === "detail") {
    return <DetailView todoId={Number(params.get("todo_id"))} date={validDate(params.get("date"))} />;
  }
  if (role === "reminder") return <ReminderWindow />;
  if (role === "day") return <DayWorkspaceWindow />;
  if (role === "settings") return <SettingsView />;
  if (role === "content-editor") return <ContentEditorWindow />;
  return <LoadingWindow title="Note" message="未知窗口" />;
}
