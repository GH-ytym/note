import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App.jsx";
import StandaloneWindow from "./StandaloneWindow.jsx";
import { AppearanceProvider } from "./appearance.jsx";
import "./styles.css";

const searchParams = new URLSearchParams(window.location.search);
const windowRole = searchParams.get("window") || "calendar";
const windowTitles = {
  calendar: "Note · 日历",
  create: "新日程 · Note",
  detail: "日程详情 · Note",
  reminder: "日程到点了 · Note",
  day: `${searchParams.get("date") || "当天日程"} · Note`,
  settings: "外观设置 · Note",
  "content-editor": "专注编辑 · Note",
};
document.title = windowTitles[windowRole] || "Note";
const content = ["create", "detail", "reminder", "day", "settings", "content-editor"].includes(windowRole)
  ? <StandaloneWindow role={windowRole} />
  : <App />;

createRoot(document.getElementById("root")).render(
  <StrictMode>
    <AppearanceProvider>
      {content}
    </AppearanceProvider>
  </StrictMode>,
);
