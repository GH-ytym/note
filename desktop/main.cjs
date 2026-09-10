const { spawn } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");
const { WAKE_STYLES, normalizeWorkspace, wakeWorkspace } = require("./workspace-mode.cjs");
const { ReminderScheduler, dateKeyAt } = require("./reminder-scheduler.cjs");
const {
  DEFAULT_EDGE_MARGIN,
  DEFAULT_GAP,
  aboveAnchorBounds,
  bottomRightBounds,
  centeredBounds,
  fitBounds,
} = require("./window-layout.cjs");
const {
  app,
  BrowserWindow,
  dialog,
  ipcMain,
  Menu,
  nativeImage,
  Notification,
  screen,
  Tray,
} = require("electron");

// The calendar is the single primary surface; editors and reminders are auxiliary windows.
const PRIMARY_WINDOW_KEY = "calendar";
const WORKSPACE_WINDOW_KEYS = new Set(["calendar"]);
const WINDOW_PROFILES = {
  calendar: { width: 680, height: 460, minWidth: 380, minHeight: 320 },
  create: { width: 340, height: 650, minWidth: 260, minHeight: 240 },
  detail: { width: 360, height: 650, minWidth: 280, minHeight: 240 },
  reminder: { width: 390, height: 250, minWidth: 320, minHeight: 230 },
  settings: { width: 780, height: 600, minWidth: 560, minHeight: 460 },
  "content-editor": { width: 760, height: 560, minWidth: 500, minHeight: 360 },
};
const DEFAULT_APPEARANCE = Object.freeze({
  backgroundColor: "#000000",
  themeColor: "#F3B51B",
  opacity: 95,
  defaultView: "month",
  dayViewMode: "clock",
  handMode: "full",
  weekOrientation: "vertical",
  dayOrientation: "vertical",
  clockTracks: 7,
  wakeStyle: "mini",
});
const TRAY_ICON_DATA_URL = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAACAAAAAgCAYAAABzenr0AAAAAXNSR0IArs4c6QAAAARnQU1BAACxjwv8YQUAAAAJcEhZcwAADsMAAA7DAcdvqGQAAACtSURBVFhH7c7RDQIhFETRbcJEY/8lWJkFaPjkCCw8XLOJ3OR+wZuZbVssBng+7q9evZ3Ggh7NCGFoRDO7MWhGs3cxQK+3S6bvJe1o4rEeOsDDkpEBSbs+8KBmdEDSzgw/1/yfARaNat65B/jx5wMSfjZwVPOGB9TsKappZ4afa64Bhw1IeFAyOsCuKh5qZIAdTTz+hnbsYsCMZndjUEQzQxjaoxnTWNDS28VpeQN+CwQ4E8tohAAAAABJRU5ErkJggg==";

const windows = new Map();
const activeNotifications = new Map();
let backendProcess = null;
let backendURL = "";
let quitting = false;
let focusPrimaryAfterReady = false;
let datePickerSession = null;
let datePickerSequence = 0;
let tray = null;
let hiddenToTray = false;
let calendarExpanded = true;
let appearanceSettings = { ...DEFAULT_APPEARANCE };
let reminderScheduler = null;
let workspaceState = null;
let lastWorkspaceState = null;
let normalWorkspaceState = null;

const hasSingleInstanceLock = app.requestSingleInstanceLock();

if (!hasSingleInstanceLock) {
  app.quit();
} else {
  app.on("second-instance", () => {
    if (!app.isReady() || !backendURL) {
      focusPrimaryAfterReady = true;
      return;
    }
    restoreApplicationWindows();
  });

  app.whenReady().then(startApplication).catch((error) => {
    dialog.showErrorBox("Note 启动失败", error instanceof Error ? error.message : String(error));
    void requestQuit();
  });

  app.on("activate", () => {
    if (backendURL) restoreApplicationWindows();
  });

  app.on("before-quit", (event) => {
    if (quitting) return;
    event.preventDefault();
    void requestQuit();
  });
}

async function startApplication() {
  app.setAppUserModelId("cn.note.calendar");
  app.setAppLogsPath();
  appearanceSettings = loadAppearanceSettings();
  try { lastWorkspaceState = JSON.parse(fs.readFileSync(path.join(app.getPath("userData"), "workspace.json"), "utf8")); } catch {}
  workspaceState = wakeWorkspace(appearanceSettings.wakeStyle, lastWorkspaceState, todayKey(), appearanceSettings);

  backendURL = await startBackend();
  registerIPC();
  installApplicationMenu();
  installTray();
  createWorkspace(todayKey());
  startReminderScheduler();

  if (focusPrimaryAfterReady) {
    focusPrimaryAfterReady = false;
    createWorkspace(todayKey());
  }
}

function resolveRuntimePaths() {
  const backendName = process.platform === "win32" ? "note-api.exe" : "note-api";
  if (app.isPackaged) {
    return {
      backend: path.join(process.resourcesPath, "backend", backendName),
      web: path.join(process.resourcesPath, "web"),
    };
  }

  return {
    backend: path.join(__dirname, "resources", backendName),
    web: path.resolve(__dirname, "..", "web", "dist"),
  };
}

function startBackend() {
  const runtime = resolveRuntimePaths();
  const appDataDirectory = path.join(app.getPath("userData"), "data");
  const databasePath = path.join(appDataDirectory, "note.db");
  const logPath = path.join(app.getPath("logs"), "backend.log");

  fs.mkdirSync(appDataDirectory, { recursive: true });
  fs.mkdirSync(path.dirname(logPath), { recursive: true });

  return new Promise((resolve, reject) => {
    let settled = false;
    const startupTimer = setTimeout(() => fail(new Error("Go 后端启动超时")), 30000);

    backendProcess = spawn(runtime.backend, [], {
      cwd: appDataDirectory,
      windowsHide: true,
      stdio: ["pipe", "pipe", "pipe"],
      env: {
        ...process.env,
        GIN_MODE: "release",
        HTTP_ADDR: "127.0.0.1:0",
        NOTE_DB_PATH: databasePath,
        NOTE_STOP_ON_STDIN_CLOSE: "1",
        NOTE_WEB_DIR: runtime.web,
      },
    });

    const consumeOutput = (chunk) => {
      const output = chunk.toString("utf8");
      fs.appendFileSync(logPath, output);
      const match = output.match(/NOTE_SERVER_URL=(http:\/\/[^\s]+)/);
      if (!match || settled) return;
      settled = true;
      clearTimeout(startupTimer);
      resolve(match[1]);
    };

    const fail = (error) => {
      if (settled) return;
      settled = true;
      clearTimeout(startupTimer);
      reject(error);
    };

    backendProcess.stdout.on("data", consumeOutput);
    backendProcess.stderr.on("data", consumeOutput);
    backendProcess.once("error", fail);
    backendProcess.once("exit", (code) => {
      backendProcess = null;
      if (!settled) fail(new Error(`Go 后端提前退出，代码 ${code ?? "unknown"}`));
    });
  });
}

function registerIPC() {
  ipcMain.handle("note:workspace-state", (event) => { assertTrustedSender(event); return workspaceState; });
  ipcMain.handle("note:workspace-save", (event, payload) => {
    assertTrustedSender(event);
    if (BrowserWindow.fromWebContents(event.sender)?.noteWindowKey !== "calendar" || datePickerSession) return;
    workspaceState = normalizeWorkspace(payload, todayKey(), appearanceSettings);
  });
  ipcMain.handle("note:mini-mode", (event, enabled) => {
    assertTrustedSender(event);
    if (BrowserWindow.fromWebContents(event.sender)?.noteWindowKey !== "calendar") return;
    if (enabled) normalWorkspaceState = { ...workspaceState, mini: false };
    workspaceState = enabled
      ? normalizeWorkspace({ ...workspaceState, mini: true }, todayKey(), appearanceSettings)
      : normalizeWorkspace(normalWorkspaceState || { view: appearanceSettings.defaultView }, todayKey(), appearanceSettings);
    const target = windows.get("calendar");
    applyWorkspaceSize(target);
    sendToWindow(target, "note:workspace-view-changed", workspaceState);
  });
  ipcMain.handle("note:mini-fit", (event, payload = {}) => {
    assertTrustedSender(event);
    const target = BrowserWindow.fromWebContents(event.sender);
    if (target?.noteWindowKey !== "calendar" || !workspaceState?.mini || datePickerSession) return;
    applyWorkspaceSize(target, Number(payload.size) || 360);
  });
  ipcMain.handle("note:toggle-calendar", (event) => {
    assertTrustedSender(event);
    return toggleCalendarWindow();
  });

  ipcMain.handle("note:calendar-visibility", (event) => {
    assertTrustedSender(event);
    return calendarVisibilityState();
  });

  ipcMain.handle("note:open-compose", (event, payload = {}) => {
    assertTrustedSender(event);
    const date = normalizeDate(payload.date);
    const workspace = createWorkspace(date);
    const editorWindow = createCreateWindow(date);
    return {
      calendar: workspace.calendar ? windowResult(workspace.calendar) : null,
      create: windowResult(editorWindow),
    };
  });

  ipcMain.handle("note:open-create", (event, payload = {}) => {
    assertTrustedSender(event);
    return windowResult(createCreateWindow(normalizeDate(payload.date)));
  });

  ipcMain.handle("note:open-detail", (event, payload = {}) => {
    assertTrustedSender(event);
    const id = normalizeTodoID(payload.eventId || payload.todoId);
    const date = normalizeDate(payload.date);
    return windowResult(payload.eventId ? createWindow({
      key: `detail:event:${id}:${date}`, role: "detail", title: "日程详情 · Note",
      query: { event_id: id, date },
    }) : createDetailWindow(id, date));
  });

  ipcMain.handle("note:open-day", (event, payload = {}) => {
    assertTrustedSender(event);
    return windowResult(showWorkspaceView("day", { date: normalizeDate(payload.date) }));
  });

  ipcMain.handle("note:open-settings", (event) => {
    assertTrustedSender(event);
    return windowResult(createSettingsWindow());
  });

  ipcMain.handle("note:open-content-editor", (event, payload = {}) => {
    assertTrustedSender(event);
    const sourceWindow = BrowserWindow.fromWebContents(event.sender);
    if (!sourceWindow || !["detail", "day"].includes(sourceWindow.noteWindowRole)) {
      throw new Error("content editor must be opened from the todo workspace");
    }

    const state = {
      todoId: normalizeTodoID(payload.todoId),
      date: normalizeDate(payload.date),
      content: normalizeTodoContent(payload.content, true),
      version: normalizeTodoVersion(payload.version),
      sourceWindow,
    };
    return windowResult(createContentEditorWindow(sourceWindow, state));
  });

  ipcMain.handle("note:content-editor-state", (event) => {
    assertTrustedSender(event);
    const target = BrowserWindow.fromWebContents(event.sender);
    const state = target?.noteContentEditorState;
    if (!target || target.noteWindowRole !== "content-editor" || !state) return null;
    return {
      todoId: state.todoId,
      date: state.date,
      content: state.content,
      version: state.version,
    };
  });

  ipcMain.handle("note:reminder-state", (event) => {
    assertTrustedSender(event);
    const target = BrowserWindow.fromWebContents(event.sender);
    if (!target || target.noteWindowRole !== "reminder") return null;
    return target.noteReminderState || null;
  });

  ipcMain.handle("note:content-editor-finish", (event, payload = {}) => {
    assertTrustedSender(event);
    const target = BrowserWindow.fromWebContents(event.sender);
    const state = target?.noteContentEditorState;
    if (!target || target.noteWindowRole !== "content-editor" || !state) {
      throw new Error("content editor session is not active");
    }

    const todoId = normalizeTodoID(payload.todoId);
    if (todoId !== state.todoId) throw new Error("content editor todo does not match");
    const result = {
      todoId,
      content: normalizeTodoContent(payload.content, false),
      version: normalizeTodoVersion(payload.version),
    };
    sendToWindow(state.sourceWindow, "note:content-editor-saved", result);
    target.close();
    return result;
  });

  ipcMain.handle("note:get-appearance", (event) => {
    assertTrustedSender(event);
    return appearanceSettings;
  });

  ipcMain.handle("note:update-appearance", (event, payload = {}) => {
    assertTrustedSender(event);
    appearanceSettings = normalizeAppearance(payload);
    persistAppearanceSettings(appearanceSettings);
    for (const target of windows.values()) applyAppearanceToWindow(target);
    broadcastAppearanceChanged();
    return appearanceSettings;
  });

  ipcMain.handle("note:close-window", (event) => {
    assertTrustedSender(event);
    const target = BrowserWindow.fromWebContents(event.sender);
    if (target?.noteWindowKey === "calendar") {
      hideCalendarWindow({ focusDay: true });
      return;
    }
    if (target?.noteWindowKey === PRIMARY_WINDOW_KEY) {
      hideApplicationWindows();
      return;
    }
    target?.close();
  });

  ipcMain.handle("note:minimize-window", (event) => {
    assertTrustedSender(event);
    const target = BrowserWindow.fromWebContents(event.sender);
    target?.minimize();
  });

  ipcMain.handle("note:toggle-maximize-window", (event) => {
    assertTrustedSender(event);
    const target = BrowserWindow.fromWebContents(event.sender);
    if (!target) return false;
    if (target.isMaximized()) target.unmaximize();
    else target.maximize();
    return target.isMaximized();
  });

  ipcMain.handle("note:window-state", (event) => {
    assertTrustedSender(event);
    const target = BrowserWindow.fromWebContents(event.sender);
    return { maximized: Boolean(target?.isMaximized()) };
  });

  ipcMain.handle("note:fit-day-window", (event, payload = {}) => {
    assertTrustedSender(event);
    const target = BrowserWindow.fromWebContents(event.sender);
    if (!target || target.noteWindowRole !== "day") return null;
    return target.getBounds();
  });

  ipcMain.handle("note:data-changed", (event, payload = {}) => {
    assertTrustedSender(event);
    broadcastDataChanged(payload, event.sender);
    void reminderScheduler?.refresh();
  });

  ipcMain.handle("note:date-picker-start", (event, payload = {}) => {
    assertTrustedSender(event);
    const sourceWindow = requireDatePickerSource(event);
    const state = normalizeDatePickerState(payload);

    if (datePickerSession && datePickerSession.sourceWindow !== sourceWindow) {
      sendToWindow(datePickerSession.sourceWindow, "note:date-picker-finished");
    }

    datePickerSession = {
      id: ++datePickerSequence,
      sourceWindow,
      sourceKey: sourceWindow.noteWindowKey,
      state,
    };

    const calendarWindow = showCalendarWindow({ focus: true });
    applyWorkspaceSize(calendarWindow);
    sendDatePickerState(calendarWindow);
    return publicDatePickerState();
  });

  ipcMain.handle("note:date-picker-update", (event, payload = {}) => {
    assertTrustedSender(event);
    const sourceWindow = requireDatePickerSource(event);
    if (!datePickerSession || datePickerSession.sourceWindow !== sourceWindow) return null;

    datePickerSession.state = normalizeDatePickerState(payload);
    sendDatePickerState(windows.get("calendar"));
    return publicDatePickerState();
  });

  ipcMain.handle("note:date-picker-select", (event, payload = {}) => {
    assertTrustedSender(event);
    const senderWindow = BrowserWindow.fromWebContents(event.sender);
    if (senderWindow?.noteWindowKey !== "calendar") throw new Error("date selection must come from calendar");
    if (!datePickerSession) return null;

    const selection = normalizeDatePickerSelection(payload, datePickerSession.state);
    datePickerSession.state = { ...datePickerSession.state, ...selection };
    const publicState = publicDatePickerState();
    sendToWindow(datePickerSession.sourceWindow, "note:date-picker-selection", publicState);
    return publicState;
  });

  ipcMain.handle("note:date-picker-finish", (event) => {
    assertTrustedSender(event);
    const senderWindow = BrowserWindow.fromWebContents(event.sender);
    if (!datePickerSession) return;
    if (senderWindow?.noteWindowKey !== "calendar" && senderWindow !== datePickerSession.sourceWindow) {
      throw new Error("window does not own date selection");
    }
    finishDatePicker(true);
  });

  ipcMain.handle("note:date-picker-cancel", (event) => {
    assertTrustedSender(event);
    const senderWindow = BrowserWindow.fromWebContents(event.sender);
    if (datePickerSession?.sourceWindow === senderWindow) finishDatePicker(false);
  });

  ipcMain.handle("note:date-picker-state", (event) => {
    assertTrustedSender(event);
    return publicDatePickerState();
  });
}

function assertTrustedSender(event) {
  const senderURL = new URL(event.senderFrame.url);
  if (senderURL.origin !== new URL(backendURL).origin) throw new Error("untrusted renderer");
}

function normalizeDate(value) {
  const date = String(value || "");
  if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) throw new Error("invalid date");
  return date;
}

function normalizeTodoID(value) {
  const todoID = Number(value);
  if (!Number.isSafeInteger(todoID) || todoID < 1) throw new Error("invalid todo id");
  return todoID;
}

function normalizeTodoVersion(value) {
  const version = Number(value);
  if (!Number.isSafeInteger(version) || version < 1) throw new Error("invalid todo version");
  return version;
}

function normalizeTodoContent(value, allowEmpty) {
  const content = String(value ?? "");
  if (content.length > 500 || (!allowEmpty && !content.trim())) throw new Error("invalid todo content");
  return content;
}

function normalizeTodoTitle(value) {
  const title = String(value ?? "").trim();
  if (!title || title.length > 50) throw new Error("invalid todo title");
  return title;
}

function normalizeHexColor(value, fallback) {
  const color = String(value || "").trim().toUpperCase();
  return /^#[0-9A-F]{6}$/.test(color) ? color : fallback;
}

function normalizeAppearance(value = {}) {
  const rawOpacity = Number(value.opacity);
  const rawClockTracks = Number(value.clockTracks);
  const choice = (candidate, allowed, fallback) =>
    allowed.includes(candidate) ? candidate : fallback;
  return {
    backgroundColor: normalizeHexColor(value.backgroundColor, DEFAULT_APPEARANCE.backgroundColor),
    themeColor: normalizeHexColor(value.themeColor, DEFAULT_APPEARANCE.themeColor),
    opacity: Number.isFinite(rawOpacity)
      ? clamp(Math.round(rawOpacity), 20, 100)
      : DEFAULT_APPEARANCE.opacity,
    defaultView: choice(
      value.defaultView,
      ["year", "month", "week", "day"],
      DEFAULT_APPEARANCE.defaultView,
    ),
    dayViewMode: choice(
      value.dayViewMode,
      ["clock", "timeline"],
      DEFAULT_APPEARANCE.dayViewMode,
    ),
    handMode: choice(
      value.handMode,
      ["full", "compact"],
      DEFAULT_APPEARANCE.handMode,
    ),
    weekOrientation: choice(
      value.weekOrientation,
      ["vertical", "horizontal"],
      DEFAULT_APPEARANCE.weekOrientation,
    ),
    dayOrientation: choice(
      value.dayOrientation,
      ["vertical", "horizontal"],
      DEFAULT_APPEARANCE.dayOrientation,
    ),
    clockTracks: Number.isFinite(rawClockTracks)
      ? clamp(Math.round(rawClockTracks), 3, 10)
      : DEFAULT_APPEARANCE.clockTracks,
    wakeStyle: choice(value.wakeStyle, WAKE_STYLES, "mini"),
  };
}

function normalizeReminderOccurrence(value = {}) {
  const occursAt = new Date(value.occurs_at);
  if (Number.isNaN(occursAt.getTime())) throw new Error("invalid reminder time");

  const notifyMode = String(value.notify_mode || "");
  if (!["silent", "popup"].includes(notifyMode)) throw new Error("invalid reminder mode");

  return {
    todoId: normalizeTodoID(value.todo_id),
    title: normalizeTodoTitle(value.title || value.content),
    content: normalizeTodoContent(value.content, false),
    color: normalizeHexColor(value.color, appearanceSettings.themeColor),
    occursAt: occursAt.toISOString(),
    date: dateKeyAt(occursAt),
    notifyMode,
  };
}

async function loadReminderOccurrences(from, to) {
  const url = new URL("/api/calendar", backendURL);
  url.searchParams.set("from", from);
  url.searchParams.set("to", to);

  const response = await fetch(url);
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(payload.error || `calendar request failed with status ${response.status}`);
  }
	if (Array.isArray(payload.data)) return payload.data;
	return Array.isArray(payload.data?.todos) ? payload.data.todos : [];
}

function startReminderScheduler() {
  reminderScheduler?.stop();
  reminderScheduler = new ReminderScheduler({
    loadOccurrences: loadReminderOccurrences,
    onReminder: deliverReminder,
    onError: (error) => console.error("Reminder scheduler:", error),
  });
  void reminderScheduler.start();
}

function deliverReminder(occurrence) {
  const reminder = normalizeReminderOccurrence(occurrence);
  if (reminder.notifyMode === "silent") {
    showNativeReminder(reminder);
    return;
  }
  createReminderWindow(reminder);
}

function showNativeReminder(reminder) {
  if (!Notification.isSupported()) {
    createReminderWindow(reminder);
    return;
  }

  const key = `${reminder.todoId}:${reminder.occursAt}`;
  if (activeNotifications.has(key)) return;

  const notification = new Notification({
    title: "Note · 日程到点了",
    body: reminder.title,
    silent: true,
  });
  const cleanup = () => activeNotifications.delete(key);
  notification.on("click", () => {
    cleanup();
    createDetailWindow(reminder.todoId, reminder.date);
  });
  notification.on("close", cleanup);
  notification.on("failed", (_event, error) => {
    cleanup();
    console.error("Native reminder failed:", error);
    createReminderWindow(reminder);
  });
  activeNotifications.set(key, notification);
  notification.show();
}

function appearanceSettingsPath() {
  return path.join(app.getPath("userData"), "appearance.json");
}

function loadAppearanceSettings() {
  try {
    return normalizeAppearance(JSON.parse(fs.readFileSync(appearanceSettingsPath(), "utf8")));
  } catch {
    return { ...DEFAULT_APPEARANCE };
  }
}

function persistAppearanceSettings(settings) {
  const settingsPath = appearanceSettingsPath();
  fs.mkdirSync(path.dirname(settingsPath), { recursive: true });
  fs.writeFileSync(settingsPath, `${JSON.stringify(settings, null, 2)}\n`, "utf8");
}

function applyAppearanceToWindow(target) {
  if (!target || target.isDestroyed()) return;
  target.setBackgroundColor(appearanceSettings.backgroundColor);
  target.setOpacity(appearanceSettings.opacity / 100);
}

function broadcastAppearanceChanged() {
  for (const target of windows.values()) {
    sendToWindow(target, "note:appearance-changed", appearanceSettings);
  }
}

function requireDatePickerSource(event) {
  const sourceWindow = BrowserWindow.fromWebContents(event.sender);
  const key = sourceWindow?.noteWindowKey || "";
  if (key !== PRIMARY_WINDOW_KEY && key !== "create" && !key.startsWith("detail:")) {
    throw new Error("date picker must be opened by the todo workspace");
  }
  return sourceWindow;
}

function normalizeDatePickerState(payload) {
  const repeatMode = String(payload.repeatMode || "");
  if (!["once", "daily", "weekdays", "weekends", "weekly", "monthly", "custom"].includes(repeatMode)) {
    throw new Error("invalid repeat mode");
  }

  const color = String(payload.color || "").toUpperCase();
  if (color && !/^#[0-9A-F]{6}$/.test(color)) throw new Error("invalid color");

  return {
    repeatMode,
    field: payload.field === "endDate" ? "endDate" : "date",
    date: normalizeDate(payload.date),
    customDates: normalizeDateList(payload.customDates),
    color,
  };
}

function normalizeDatePickerSelection(payload, currentState) {
  const date = normalizeDate(payload.date || currentState.date);
  const customDates = currentState.repeatMode === "custom"
    ? normalizeDateList(payload.customDates)
    : [];
  return { date, customDates };
}

function normalizeDateList(values) {
  if (!Array.isArray(values)) return [];
  return [...new Set(values.map(normalizeDate))].sort();
}

function windowResult(target) {
  return { key: target.noteWindowKey };
}

function broadcastDataChanged(payload, sourceWebContents = null) {
  for (const target of windows.values()) {
    if (!target.isDestroyed() && target.webContents !== sourceWebContents) {
      target.webContents.send("note:data-changed", payload);
    }
  }
}

function publicDatePickerState() {
  if (!datePickerSession) return null;
  return {
    sessionId: datePickerSession.id,
    sourceKey: datePickerSession.sourceKey,
    ...datePickerSession.state,
  };
}

function sendToWindow(target, channel, payload) {
  if (!target || target.isDestroyed()) return;
  const send = () => {
    if (!target.isDestroyed()) target.webContents.send(channel, payload);
  };
  if (target.webContents.isLoadingMainFrame()) {
    target.webContents.once("did-finish-load", send);
  } else {
    send();
  }
}

function sendDatePickerState(target = windows.get("calendar")) {
  sendToWindow(target, "note:date-picker-state-changed", publicDatePickerState());
}

function finishDatePicker(focusSource) {
  const session = datePickerSession;
  if (!session) return;
  datePickerSession = null;
  sendDatePickerState();
  sendToWindow(session.sourceWindow, "note:date-picker-finished");

  if (!focusSource || session.sourceWindow.isDestroyed()) return;
  if (session.sourceWindow.isMinimized()) session.sourceWindow.restore();
  session.sourceWindow.show();
  session.sourceWindow.focus();
}

function createCalendarWindow() {
  return createWindow({
    key: "calendar",
    role: "calendar",
    title: "Note · 日历",
  });
}

function calendarVisibilityState() {
  const target = windows.get("calendar");
  return { open: Boolean(target && !target.isDestroyed() && target.isVisible()) };
}

function broadcastCalendarVisibility() {
  sendToWindow(windows.get(PRIMARY_WINDOW_KEY), "note:calendar-visibility-changed", calendarVisibilityState());
}



function showCalendarWindow({ focus = true } = {}) {
  calendarExpanded = true;
  const calendar = createCalendarWindow();
  if (calendar.isMinimized()) calendar.restore();
  calendar.show();
  broadcastCalendarVisibility();
  if (focus) calendar.focus();
  return calendar;
}

function hideCalendarWindow() {
  if (datePickerSession) {
    const source = datePickerSession.sourceWindow;
    windows.get("calendar")?.hide();
    finishDatePicker(false);
    if (source && !source.isDestroyed()) { source.show(); source.focus(); }
    return;
  }
  hideApplicationWindows();
}

function toggleCalendarWindow() {
  if (calendarVisibilityState().open) {
    hideCalendarWindow({ focusDay: true });
    return { open: false };
  }
  return { ...windowResult(showCalendarWindow({ focus: true })), open: true };
}

function createCreateWindow(date) {
  return createWindow({ key: "create", role: "create", title: "新建 · Note", query: { date } });
}

function createDetailWindow(todoID, date) {
  return createWindow({ key: `detail:${todoID}:${date}`, role: "detail", title: "待办详情 · Note", query: { todo_id: todoID, date } });
}

function createSettingsWindow() {
  return createWindow({ key: "settings", role: "settings", title: "设置 · Note" });
}

function createContentEditorWindow(sourceWindow, state) {
  return createWindow({
    key: `content-editor:${sourceWindow.noteWindowKey}`,
    role: "content-editor",
    title: "专注编辑 · Note",
    contentEditorState: state,
  });
}

function createReminderWindow(state) {
  return createWindow({
    key: `reminder:${state.todoId}:${state.occursAt}`,
    role: "reminder",
    title: "日程到点了 · Note",
    reminderState: state,
    alwaysOnTop: true,
    maximizable: false,
  });
}

function createDayWindow(date) { return showWorkspaceView("day", { date }); }

function currentWorkspaceDate() {
  const target = windows.get(PRIMARY_WINDOW_KEY);
  return target && !target.isDestroyed() ? target.noteDate || todayKey() : todayKey();
}

function createWorkspace(date = todayKey()) {
  const calendar = showCalendarWindow();
  calendar.noteDate = date;
  return { calendar };
}

function showWorkspaceView(view, payload = {}) {
  const date = normalizeDate(payload.date || todayKey());
  const calendar = showCalendarWindow();
  calendar.noteDate = date;
  workspaceState = normalizeWorkspace({ ...workspaceState, view, date, mini: false }, todayKey(), appearanceSettings);
  applyWorkspaceSize(calendar);
  sendToWindow(calendar, "note:workspace-view-changed", workspaceState);
  return calendar;
}

function createWindow({
  key,
  role,
  title,
  query = {},
  parent = null,
  modal = false,
  contentEditorState = null,
  reminderState = null,
  alwaysOnTop = false,
  maximizable = true,
}) {
  if (key !== PRIMARY_WINDOW_KEY) {
    for (const other of [...windows.values()]) {
      if (other.noteWindowKey !== PRIMARY_WINDOW_KEY && other.noteWindowKey !== key && !other.isDestroyed()) {
        other.noteReplaced = true;
        other.close();
      }
    }
  }
  const existing = windows.get(key);
  if (existing && !existing.isDestroyed()) {
    if (existing.isMinimized()) existing.restore();
    if (role === "reminder") placeWindowAtDefault(existing, role, parent);
    existing.show();
    if (!WORKSPACE_WINDOW_KEYS.has(key)) existing.focus();
    return existing;
  }

  const sizing = windowSizing(role);
  const initialBounds = initialWindowBounds(role, sizing, parent);
  const target = new BrowserWindow({
    ...initialBounds,
    minWidth: sizing.minWidth,
    minHeight: sizing.minHeight,
    show: false,
    // 保留完整的 Windows 原生窗口框架，只隐藏系统标题栏。
    // 这能避开 Electron 41.3+ 的 frameless + thickFrame 边界回归，
    // 页面仍然使用自己的拖动区和窗口按钮。
    titleBarStyle: "hidden",
    ...(process.platform === "darwin" ? { trafficLightPosition: { x: 12, y: 12 } } : {}),
    resizable: true,
    minimizable: true,
    maximizable,
    alwaysOnTop,
    autoHideMenuBar: true,
    backgroundColor: appearanceSettings.backgroundColor,
    opacity: appearanceSettings.opacity / 100,
    icon: path.join(__dirname, "build", "window-icon.png"),
    title,
    ...(parent ? { parent, modal } : {}),
    webPreferences: {
      preload: path.join(__dirname, "preload.cjs"),
      backgroundThrottling: false,
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
    },
  });
  applyAppearanceToWindow(target);
  // The renderer supplies window controls, including in the compact clock.
  if (process.platform === "darwin") target.setWindowButtonVisibility(false);

  target.noteWindowKey = key;
  target.noteWindowRole = role;
  target.noteContentEditorState = contentEditorState;
  target.noteReminderState = reminderState;
  windows.set(key, target);
  if (key === PRIMARY_WINDOW_KEY) applyWorkspaceSize(target);

  const url = new URL(backendURL);
  url.searchParams.set("window", role);
  for (const [name, value] of Object.entries(query)) url.searchParams.set(name, String(value));

  target.loadURL(url.toString());
  target.once("ready-to-show", () => {
    if (role === "reminder" && hiddenToTray) {
      for (const other of windows.values()) {
        if (other !== target && !other.isDestroyed()) other.hide();
      }
    }
    target.show();
    if (key === "calendar") broadcastCalendarVisibility();
    if (modal || role === "reminder") target.focus();
    if (role === "reminder") target.flashFrame(true);
  });

  target.on("maximize", () => sendToWindow(target, "note:window-maximized-changed", true));
  target.on("unmaximize", () => sendToWindow(target, "note:window-maximized-changed", false));
  target.on("close", (event) => {
    if (quitting) return;
    if (key === "calendar") {
      event.preventDefault();
      hideCalendarWindow({ focusDay: true });
      return;
    }
    if (key !== PRIMARY_WINDOW_KEY) return;
    event.preventDefault();
    hideApplicationWindows();
  });

  target.on("closed", () => {
    windows.delete(key);
    if (datePickerSession?.sourceWindow === target) finishDatePicker(false);
    if (contentEditorState && !target.noteReplaced && !quitting) {
      createDetailWindow(contentEditorState.todoId, contentEditorState.date);
    }
  });
  target.webContents.setWindowOpenHandler(() => ({ action: "deny" }));
  return target;
}

function windowSizing(role) {
  const profile = WINDOW_PROFILES[role] || WINDOW_PROFILES.calendar;
  const primary = windows.get(PRIMARY_WINDOW_KEY);
  const area = (primary && !primary.isDestroyed() ? screen.getDisplayMatching(primary.getBounds()) : screen.getPrimaryDisplay()).workArea;
  return {
    width: Math.min(profile.width, area.width - 24), height: Math.min(profile.height, area.height - 24),
    minWidth: Math.min(profile.minWidth, area.width - 24), minHeight: Math.min(profile.minHeight, area.height - 24),
  };
}

function initialWindowBounds(role, sizing, parent = null) {
  const primary = windows.get(PRIMARY_WINDOW_KEY);
  const primaryBounds = primary && !primary.isDestroyed() ? primary.getBounds() : null;
  const parentBounds = parent && !parent.isDestroyed() ? parent.getBounds() : null;
  const anchorBounds = parentBounds || primaryBounds;
  const display = anchorBounds
    ? screen.getDisplayMatching(anchorBounds)
    : screen.getPrimaryDisplay();
  const area = display.workArea;

  if (role === "calendar") return bottomRightBounds(area, sizing, DEFAULT_EDGE_MARGIN);

  if (role === "reminder") {
    return bottomRightBounds(area, sizing, DEFAULT_EDGE_MARGIN);
  }

  if (parentBounds && role === "content-editor") {
    return centeredBounds(area, sizing, parentBounds);
  }

  const dayBounds = primaryBounds || bottomRightBounds(area, windowSizing("calendar"), DEFAULT_EDGE_MARGIN);
  if (role === "create" || role === "settings" || role === "detail") {
    const openDetails = role === "detail"
      ? [...windows.values()].filter((target) => (
        !target.isDestroyed() && target.noteWindowRole === "detail"
      )).length
      : 0;
    return aboveAnchorBounds(area, sizing, dayBounds, {
      edgeMargin: DEFAULT_EDGE_MARGIN,
      gap: DEFAULT_GAP,
      offset: openDetails * 18,
    });
  }

  return centeredBounds(area, sizing);
}

function isWorkspaceWindow(target) {
  return Boolean(target && !target.isDestroyed() && WORKSPACE_WINDOW_KEYS.has(target.noteWindowKey));
}



















function placeWindowAtDefault(target, role, parent = null) {
  if (!target || target.isDestroyed() || target.isMaximized()) return;
  const current = target.getBounds();
  target.setBounds(initialWindowBounds(role, {
    width: current.width,
    height: current.height,
  }, parent), false);
}

function clamp(value, minimum, maximum) {
  return Math.max(minimum, Math.min(value, maximum));
}



function installTray() {
  if (tray) return;
  const icon = nativeImage.createFromDataURL(TRAY_ICON_DATA_URL).resize({ width: 16, height: 16 });
  tray = new Tray(icon);
  tray.setToolTip("Note · 备忘录");
  tray.setContextMenu(Menu.buildFromTemplate([
    { label: "显示 Note", click: restoreApplicationWindows },
    { type: "separator" },
    { label: "退出 Note", click: () => void requestQuit() },
  ]));
  tray.on("click", restoreApplicationWindows);
}

function hideApplicationWindows() {
  lastWorkspaceState = workspaceState;
  fs.writeFileSync(path.join(app.getPath("userData"), "workspace.json"), JSON.stringify(lastWorkspaceState));
  hiddenToTray = true;
  for (const target of windows.values()) {
    if (!target.isDestroyed()) target.hide();
  }
}

function restoreApplicationWindows() {
  hiddenToTray = false;
  workspaceState = wakeWorkspace(appearanceSettings.wakeStyle, lastWorkspaceState, todayKey(), appearanceSettings);
  const { calendar } = createWorkspace(todayKey());
  applyWorkspaceSize(calendar);
  sendToWindow(calendar, "note:workspace-view-changed", workspaceState);
  calendar.focus();
}

function applyWorkspaceSize(target, miniSize = 360) {
  if (!target || target.isDestroyed()) return;
  const mini = workspaceState?.mini && !datePickerSession;
  if (target.isMaximized()) target.unmaximize();
  if (mini) {
    if (!target.noteMini) target.noteNormalBounds = target.getBounds();
    const area = screen.getDisplayMatching(target.getBounds()).workArea;
    const size = Math.round(clamp(miniSize, 280, Math.min(area.width, area.height) - 24));
    target.setMinimumSize(240, 240);
    target.setResizable(false);
    const current = target.getBounds();
    target.setBounds(fitBounds(area, { x: current.x + current.width - size, y: current.y + current.height - size, width: size, height: size }), false);
  } else if (target.noteMini || datePickerSession) {
    target.setResizable(true);
    target.setMinimumSize(WINDOW_PROFILES.calendar.minWidth, WINDOW_PROFILES.calendar.minHeight);
    const area = screen.getDisplayMatching(target.getBounds()).workArea;
    target.setBounds(fitBounds(area, target.noteNormalBounds || bottomRightBounds(area, WINDOW_PROFILES.calendar)), false);
  }
  target.noteMini = Boolean(mini);
}

function installApplicationMenu() {
  const template = [
    ...(process.platform === "darwin" ? [{ role: "appMenu" }] : []),
    {
      label: "日程",
      submenu: [
        {
          label: "新建",
          accelerator: "CmdOrCtrl+N",
          click: () => createCreateWindow(todayKey()),
        },
        {
          label: "展开/收回日历",
          accelerator: "CmdOrCtrl+Shift+C",
          click: () => toggleCalendarWindow(),
        },
        {
          label: "显示当天日程",
          accelerator: "CmdOrCtrl+Shift+T",
          click: () => createDayWindow(todayKey()),
        },
        { type: "separator" },
        { role: "close", label: "关闭当前窗口" },
        { role: "quit", label: "退出 Note" },
      ],
    },
    {
      label: "编辑",
      submenu: [
        { role: "undo", label: "撤销" },
        { role: "redo", label: "重做" },
        { type: "separator" },
        { role: "cut", label: "剪切" },
        { role: "copy", label: "复制" },
        { role: "paste", label: "粘贴" },
        { role: "selectAll", label: "全选" },
      ],
    },
    {
      label: "查看",
      submenu: [
        { role: "reload", label: "重新加载" },
        { role: "togglefullscreen", label: "切换全屏" },
      ],
    },
  ];
  Menu.setApplicationMenu(Menu.buildFromTemplate(template));
}

function todayKey() {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: "Asia/Shanghai",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(new Date());
  const value = (type) => parts.find((part) => part.type === type)?.value;
  return `${value("year")}-${value("month")}-${value("day")}`;
}

function stopBackend() {
  const child = backendProcess;
  if (!child || child.exitCode !== null) return Promise.resolve();

  return new Promise((resolve) => {
    let finished = false;
    const finish = () => {
      if (finished) return;
      finished = true;
      clearTimeout(forceTimer);
      resolve();
    };
    const forceTimer = setTimeout(() => {
      child.kill();
      finish();
    }, 5000);
    child.once("exit", finish);
    child.stdin.end();
  });
}

async function requestQuit() {
  if (quitting) return;
  quitting = true;
  reminderScheduler?.stop();
  reminderScheduler = null;
  for (const notification of activeNotifications.values()) notification.close();
  activeNotifications.clear();
  await stopBackend();
  app.quit();
}
