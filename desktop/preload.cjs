const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("noteDesktop", Object.freeze({
  isDesktop: true,
  openCalendar: () => ipcRenderer.invoke("note:open-calendar"),
  openCompose: (payload) => ipcRenderer.invoke("note:open-compose", payload),
  openCreate: (payload) => ipcRenderer.invoke("note:open-create", payload),
  openDetail: (payload) => ipcRenderer.invoke("note:open-detail", payload),
  openDay: (payload) => ipcRenderer.invoke("note:open-day", payload),
  openSettings: () => ipcRenderer.invoke("note:open-settings"),
  openContentEditor: (payload) => ipcRenderer.invoke("note:open-content-editor", payload),
  getContentEditorState: () => ipcRenderer.invoke("note:content-editor-state"),
  getReminderState: () => ipcRenderer.invoke("note:reminder-state"),
  finishContentEditor: (payload) => ipcRenderer.invoke("note:content-editor-finish", payload),
  getAppearance: () => ipcRenderer.invoke("note:get-appearance"),
  updateAppearance: (payload) => ipcRenderer.invoke("note:update-appearance", payload),
  closeCurrent: () => ipcRenderer.invoke("note:close-window"),
  minimizeCurrent: () => ipcRenderer.invoke("note:minimize-window"),
  toggleMaximizeCurrent: () => ipcRenderer.invoke("note:toggle-maximize-window"),
  getWindowState: () => ipcRenderer.invoke("note:window-state"),
  fitDayWindow: (payload) => ipcRenderer.invoke("note:fit-day-window", payload),
  notifyDataChanged: (payload) => ipcRenderer.invoke("note:data-changed", payload),
  startDatePicker: (payload) => ipcRenderer.invoke("note:date-picker-start", payload),
  updateDatePicker: (payload) => ipcRenderer.invoke("note:date-picker-update", payload),
  selectDatePicker: (payload) => ipcRenderer.invoke("note:date-picker-select", payload),
  finishDatePicker: () => ipcRenderer.invoke("note:date-picker-finish"),
  cancelDatePicker: () => ipcRenderer.invoke("note:date-picker-cancel"),
  getDatePickerState: () => ipcRenderer.invoke("note:date-picker-state"),
  onDataChanged: (callback) => {
    const listener = (_event, payload) => callback(payload);
    ipcRenderer.on("note:data-changed", listener);
    return () => ipcRenderer.removeListener("note:data-changed", listener);
  },
  onContentEditorSaved: (callback) => {
    const listener = (_event, payload) => callback(payload);
    ipcRenderer.on("note:content-editor-saved", listener);
    return () => ipcRenderer.removeListener("note:content-editor-saved", listener);
  },
  onAppearanceChanged: (callback) => {
    const listener = (_event, payload) => callback(payload);
    ipcRenderer.on("note:appearance-changed", listener);
    return () => ipcRenderer.removeListener("note:appearance-changed", listener);
  },
  onDayDateChanged: (callback) => {
    const listener = (_event, payload) => callback(payload);
    ipcRenderer.on("note:day-date-changed", listener);
    return () => ipcRenderer.removeListener("note:day-date-changed", listener);
  },
  onDatePickerStateChanged: (callback) => {
    const listener = (_event, payload) => callback(payload);
    ipcRenderer.on("note:date-picker-state-changed", listener);
    return () => ipcRenderer.removeListener("note:date-picker-state-changed", listener);
  },
  onDatePickerSelection: (callback) => {
    const listener = (_event, payload) => callback(payload);
    ipcRenderer.on("note:date-picker-selection", listener);
    return () => ipcRenderer.removeListener("note:date-picker-selection", listener);
  },
  onDatePickerFinished: (callback) => {
    const listener = () => callback();
    ipcRenderer.on("note:date-picker-finished", listener);
    return () => ipcRenderer.removeListener("note:date-picker-finished", listener);
  },
  onMaximizedChanged: (callback) => {
    const listener = (_event, maximized) => callback(Boolean(maximized));
    ipcRenderer.on("note:window-maximized-changed", listener);
    return () => ipcRenderer.removeListener("note:window-maximized-changed", listener);
  },
}));
