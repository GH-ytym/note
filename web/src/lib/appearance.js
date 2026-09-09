export const DEFAULT_APPEARANCE = Object.freeze({
  backgroundColor: "#000000",
  themeColor: "#F3B51B",
  opacity: 95,
});

export const DEFAULT_PREFERENCES = Object.freeze({
  defaultView: "month",
  dayViewMode: "clock",
  handMode: "full",
  weekOrientation: "vertical",
  dayOrientation: "vertical",
  clockTracks: 7,
});

export const DEFAULT_SETTINGS = Object.freeze({
  ...DEFAULT_APPEARANCE,
  ...DEFAULT_PREFERENCES,
});

export function normalizeHex(value, fallback) {
  const hex = String(value || "")
    .trim()
    .toUpperCase();
  return /^#[0-9A-F]{6}$/.test(hex) ? hex : fallback;
}

export function normalizeAppearance(value = {}) {
  const opacity = Number(value.opacity);
  return {
    backgroundColor: normalizeHex(
      value.backgroundColor,
      DEFAULT_APPEARANCE.backgroundColor,
    ),
    themeColor: normalizeHex(value.themeColor, DEFAULT_APPEARANCE.themeColor),
    opacity: Number.isFinite(opacity)
      ? Math.max(20, Math.min(100, Math.round(opacity)))
      : DEFAULT_APPEARANCE.opacity,
  };
}

function oneOf(value, choices, fallback) {
  return choices.includes(value) ? value : fallback;
}

export function normalizeSettings(value = {}) {
  const appearance = normalizeAppearance(value);
  const clockTracks = Number(value.clockTracks);
  return {
    ...appearance,
    defaultView: oneOf(
      value.defaultView,
      ["year", "month", "week", "day"],
      DEFAULT_PREFERENCES.defaultView,
    ),
    dayViewMode: oneOf(
      value.dayViewMode,
      ["clock", "timeline"],
      DEFAULT_PREFERENCES.dayViewMode,
    ),
    handMode: oneOf(
      value.handMode,
      ["full", "compact"],
      DEFAULT_PREFERENCES.handMode,
    ),
    weekOrientation: oneOf(
      value.weekOrientation,
      ["vertical", "horizontal"],
      DEFAULT_PREFERENCES.weekOrientation,
    ),
    dayOrientation: oneOf(
      value.dayOrientation,
      ["vertical", "horizontal"],
      DEFAULT_PREFERENCES.dayOrientation,
    ),
    clockTracks: Number.isFinite(clockTracks)
      ? Math.max(3, Math.min(10, Math.round(clockTracks)))
      : DEFAULT_PREFERENCES.clockTracks,
  };
}

export function hexChannels(hex) {
  return [1, 3, 5].map((index) => parseInt(hex.slice(index, index + 2), 16));
}
export function mixHex(from, to, amount) {
  const end = hexChannels(to);
  return (
    "#" +
    hexChannels(from)
      .map((start, index) =>
        Math.round(start + (end[index] - start) * amount)
          .toString(16)
          .padStart(2, "0"),
      )
      .join("")
  );
}
export function luminance(hex) {
  const channels = hexChannels(hex).map((value) => {
    const n = value / 255;
    return n <= 0.04045 ? n / 12.92 : ((n + 0.055) / 1.055) ** 2.4;
  });
  return channels[0] * 0.2126 + channels[1] * 0.7152 + channels[2] * 0.0722;
}
export function contrast(a, b) {
  const x = luminance(a),
    y = luminance(b);
  return (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05);
}
export function foreground(background) {
  const ink =
    contrast("#171A18", background) >= contrast("#F2F4EF", background)
      ? "#171A18"
      : "#F2F4EF";
  if (contrast(ink, background) >= 4.5) return ink;
  return contrast("#000000", background) > contrast("#FFFFFF", background)
    ? "#000000"
    : "#FFFFFF";
}
function readable(color, surfaces, ink) {
  for (let step = 0; step <= 100; step++) {
    const candidate = mixHex(color, ink, step / 100);
    if (surfaces.every((surface) => contrast(candidate, surface) >= 4.5))
      return candidate;
  }
  return ink;
}

export function appearanceTokens(settings) {
  const background = settings.backgroundColor,
    ink = foreground(background);
  const light = luminance(ink) < 0.5,
    target = light ? "#FFFFFF" : "#000000";
  // Keep surfaces on the same side of the contrast boundary as the background.
  const panel = mixHex(background, target, 0.04),
    cell = mixHex(background, target, 0.08);
  const hover = mixHex(background, target, 0.14),
    surfaces = [background, panel, cell, hover];
  return {
    "--page": background,
    "--panel": panel,
    "--cell": cell,
    "--cell-hover": hover,
    "--cell-disabled": mixHex(background, target, 0.02),
    "--text": ink,
    "--muted": readable(mixHex(ink, background, 0.42), surfaces, ink),
    "--dim": readable(mixHex(ink, background, 0.5), surfaces, ink),
    "--line": `rgb(${hexChannels(ink).join(" ")} / 18%)`,
    "--yellow": settings.themeColor,
    "--theme-rgb": hexChannels(settings.themeColor).join(" "),
    "--accent-ink": readable(settings.themeColor, surfaces, ink),
    "--on-theme": foreground(settings.themeColor),
    "--danger": readable(light ? "#B42318" : "#FF8A80", surfaces, ink),
    "--window-opacity": String(settings.opacity / 100),
  };
}
