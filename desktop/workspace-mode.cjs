const WAKE_STYLES = ["mini", "last", "year", "month", "week", "timeline", "clock"];

function normalizeWorkspace(value = {}, today, settings = {}) {
  const mini = Boolean(value.mini);
  return {
    mini,
    view: mini ? "day" : ["year", "month", "week", "day"].includes(value.view) ? value.view : settings.defaultView || "month",
    dayStyle: mini ? "clock" : ["clock", "timeline"].includes(value.dayStyle) ? value.dayStyle : settings.dayViewMode || "clock",
    handMode: ["full", "compact"].includes(value.handMode) ? value.handMode : settings.handMode || "full",
    date: mini ? today : /^\d{4}-\d{2}-\d{2}$/.test(value.date || "") ? value.date : today,
  };
}
function wakeWorkspace(style, last, today, settings) {
  if (style === "last" && last) return normalizeWorkspace(last, today, settings);
  if (style === "mini" || !WAKE_STYLES.includes(style)) return normalizeWorkspace({ mini: true }, today, settings);
  if (style === "clock" || style === "timeline") return normalizeWorkspace({ view: "day", dayStyle: style }, today, settings);
  return normalizeWorkspace({ view: style }, today, settings);
}
module.exports = { WAKE_STYLES, normalizeWorkspace, wakeWorkspace };
