const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const vm = require("node:vm");
const path = require("node:path");
const { EventEmitter } = require("node:events");
const { wakeWorkspace } = require("./workspace-mode.cjs");

function harness() {
  const handlers = new Map();
  const area = { x: 0, y: 0, width: 1920, height: 1040 };
  class Window extends EventEmitter {
    constructor(options) {
      super(); this.bounds = options; this.visible = false; this.destroyed = false;
      this.webContents = new EventEmitter();
      this.webContents.owner = this;
      this.webContents.messages = [];
      this.webContents.isLoadingMainFrame = () => false;
      this.webContents.send = (channel, data) => this.webContents.messages.push({ channel, data });
      this.webContents.setWindowOpenHandler = () => {};
    }
    static fromWebContents(contents) { return contents.owner; }
    getBounds() { const {x,y,width,height} = this.bounds; return {x,y,width,height}; }
    setBounds(bounds) { this.bounds = bounds; }
    setMinimumSize() {} setResizable() {} setOpacity() {} setBackgroundColor() {}
    isDestroyed() { return this.destroyed; } isMinimized() { return false; }
    isMaximized() { return false; } isVisible() { return this.visible; }
    restore() {} show() { this.visible = true; } hide() { this.visible = false; }
    focus() { this.focused = true; } flashFrame() {} loadURL() {}
    close() { let prevented = false; this.emit("close", {preventDefault() {prevented = true;}}); if(!prevented) {this.destroyed = true; this.emit("closed");} }
  }
  const electron = {
    BrowserWindow: Window,
    app: { requestSingleInstanceLock: () => true, on() {}, whenReady: () => new Promise(() => {}), getPath: () => "C:/temp" },
    ipcMain: { handle: (name, handler) => handlers.set(name, handler) },
    screen: { getPrimaryDisplay: () => ({workArea: area}), getDisplayMatching: () => ({workArea: area}) },
  };
  const sandbox = { require: name => name === "electron" ? electron : name === "node:fs" ? {...fs, writeFileSync() {}} : name.startsWith(".") ? require(path.join(__dirname, name)) : require(name), __dirname, process, URL, console, setTimeout, clearTimeout };
  vm.createContext(sandbox);
  vm.runInContext(fs.readFileSync(path.join(__dirname, "main.cjs"), "utf8") + `
    backendURL = "http://127.0.0.1:12345";
    workspaceState = normalizeWorkspace({view:"day"}, todayKey(), appearanceSettings);
    registerIPC();
    globalThis.api = {createCalendarWindow, createCreateWindow, createDetailWindow, createSettingsWindow, createContentEditorWindow, createReminderWindow, windows, hideApplicationWindows, restoreApplicationWindows};
  `, sandbox);
  const invoke = (window, name, payload) => handlers.get(name)({sender: window.webContents, senderFrame: {url:"http://127.0.0.1:12345/"}}, payload);
  return {...sandbox.api, invoke};
}

test("every auxiliary role replaces the previous window, never the main calendar", () => {
  const h = harness(); const calendar = h.createCalendarWindow();
  let previous = h.createCreateWindow("2026-09-10");
  for (const open of [() => h.createDetailWindow(1,"2026-09-10"), () => h.createSettingsWindow(), () => h.createReminderWindow({todoId:1,occursAt:"2026-09-10T10:00:00Z"}), () => h.createCreateWindow("2026-09-10")]) {
    const next = open(); assert.equal(previous.isDestroyed(), true); assert.equal(calendar.isDestroyed(), false); assert.equal(h.windows.size, 2); previous = next;
  }
});

test("focus editor replaces detail and returns to a new detail, without reopening on replacement", () => {
  const h = harness(); h.createCalendarWindow();
  const detail = h.createDetailWindow(1,"2026-09-10");
  const editor = h.createContentEditorWindow(detail,{sourceWindow:detail,todoId:1,date:"2026-09-10",content:"draft",version:1});
  assert.equal(detail.isDestroyed(),true);
  editor.close(); assert.equal(h.windows.size,2); assert.equal([...h.windows.values()].at(-1).noteWindowRole,"detail");
  const nextDetail = [...h.windows.values()].at(-1);
  h.createContentEditorWindow(nextDetail,{sourceWindow:nextDetail,todoId:1,date:"2026-09-10"});
  h.createSettingsWindow(); assert.equal(h.windows.size,2); assert.equal([...h.windows.values()].at(-1).noteWindowRole,"settings");
});

test("date selection is delivered immediately and collapsing leaves its source visible", () => {
  const h = harness(); const calendar = h.createCalendarWindow(); const form = h.createCreateWindow("2026-09-10"); form.show();
  h.invoke(form,"note:date-picker-start",{field:"endDate",repeatMode:"once",date:"2026-09-11"});
  h.invoke(calendar,"note:date-picker-select",{date:"2026-09-12"});
  const selection = form.webContents.messages.find(message=>message.channel==="note:date-picker-selection").data;
  assert.equal(selection.field,"endDate"); assert.equal(selection.date,"2026-09-12");
  h.invoke(calendar,"note:close-window");
  assert.equal(form.isVisible(),true); assert.equal(form.isDestroyed(),false); assert.equal(calendar.isVisible(),false);
  assert.equal(h.windows.size,2);
});

test("mini and restored workspace sizes, last-style wake", () => {
  const h = harness(); const calendar = h.createCalendarWindow(); const original = calendar.getBounds();
  h.invoke(calendar,"note:mini-mode",true);
  h.invoke(calendar,"note:mini-fit",{size:432}); assert.equal(calendar.getBounds().width,432);
  h.invoke(calendar,"note:mini-mode",false); assert.deepEqual(calendar.getBounds(),original);
  const last = {mini:false,view:"week",dayStyle:"timeline",date:"2026-09-09",handMode:"compact"};
  assert.deepEqual(wakeWorkspace("last",last,"2026-09-10",{}),last);
  assert.equal(wakeWorkspace("mini",last,"2026-09-10",{}).date,"2026-09-10");
  assert.equal(wakeWorkspace("mini",last,"2026-09-10",{}).dayStyle,"clock");
});
