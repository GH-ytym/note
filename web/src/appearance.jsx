import { useCallback, useContext, useEffect, useLayoutEffect, useMemo, useRef, useState, createContext } from "react";
import { ArrowCounterClockwise, CheckCircle } from "@phosphor-icons/react";

export const DEFAULT_APPEARANCE = Object.freeze({
  backgroundColor: "#000000",
  themeColor: "#F3B51B",
  opacity: 95,
});

const APPEARANCE_STORAGE_KEY = "note.appearance.v1";
const AppearanceContext = createContext(null);

function normalizeHex(value, fallback) {
  const normalized = String(value || "").trim().toUpperCase();
  return /^#[0-9A-F]{6}$/.test(normalized) ? normalized : fallback;
}

function clampOpacity(value) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) return DEFAULT_APPEARANCE.opacity;
  return Math.max(20, Math.min(100, Math.round(parsed)));
}

export function normalizeAppearance(value = {}) {
  return {
    backgroundColor: normalizeHex(value.backgroundColor, DEFAULT_APPEARANCE.backgroundColor),
    themeColor: normalizeHex(value.themeColor, DEFAULT_APPEARANCE.themeColor),
    opacity: clampOpacity(value.opacity),
  };
}

function hexChannels(value) {
  const hex = normalizeHex(value, "#000000").slice(1);
  return [0, 2, 4].map((offset) => Number.parseInt(hex.slice(offset, offset + 2), 16));
}

function mixHex(from, to, amount) {
  const start = hexChannels(from);
  const end = hexChannels(to);
  const channels = start.map((channel, index) => Math.round(channel + (end[index] - channel) * amount));
  return `#${channels.map((channel) => channel.toString(16).padStart(2, "0")).join("")}`.toUpperCase();
}

function relativeLuminance(value) {
  const channels = hexChannels(value).map((channel) => {
    const normalized = channel / 255;
    return normalized <= 0.03928
      ? normalized / 12.92
      : ((normalized + 0.055) / 1.055) ** 2.4;
  });
  return channels[0] * 0.2126 + channels[1] * 0.7152 + channels[2] * 0.0722;
}

function applyAppearance(settings) {
  const root = document.documentElement;
  const background = settings.backgroundColor;
  const theme = settings.themeColor;
  const [themeRed, themeGreen, themeBlue] = hexChannels(theme);
  const lightBackground = relativeLuminance(background) > 0.48;
  const surfaceTarget = lightBackground ? "#000000" : "#FFFFFF";

  root.style.setProperty("--page", background);
  root.style.setProperty("--panel", mixHex(background, surfaceTarget, lightBackground ? 0.055 : 0.065));
  root.style.setProperty("--cell", mixHex(background, surfaceTarget, lightBackground ? 0.075 : 0.085));
  root.style.setProperty("--cell-hover", mixHex(background, surfaceTarget, lightBackground ? 0.11 : 0.12));
  root.style.setProperty("--cell-disabled", mixHex(background, surfaceTarget, lightBackground ? 0.035 : 0.04));
  root.style.setProperty("--text", lightBackground ? "#171A18" : "#F2F4EF");
  root.style.setProperty("--muted", lightBackground ? "#555C56" : "#8B918B");
  root.style.setProperty("--dim", lightBackground ? "#737A74" : "#4F554F");
  root.style.setProperty("--line", lightBackground ? "rgb(12 18 14 / 16%)" : "rgb(225 229 221 / 12%)");
  root.style.setProperty("--yellow", theme);
  root.style.setProperty("--theme-rgb", `${themeRed} ${themeGreen} ${themeBlue}`);
  root.style.setProperty("--on-theme", relativeLuminance(theme) > 0.42 ? "#171100" : "#FFFFFF");
  root.style.setProperty("--window-opacity", String(settings.opacity / 100));
  root.style.colorScheme = lightBackground ? "light" : "dark";
}

function readStoredAppearance() {
  try {
    const stored = window.localStorage.getItem(APPEARANCE_STORAGE_KEY);
    return stored ? normalizeAppearance(JSON.parse(stored)) : DEFAULT_APPEARANCE;
  } catch {
    return DEFAULT_APPEARANCE;
  }
}

function storeAppearance(settings) {
  try {
    window.localStorage.setItem(APPEARANCE_STORAGE_KEY, JSON.stringify(settings));
  } catch {
    // Electron 主进程仍会持久化；浏览器禁用存储时只保留本次会话。
  }
}

export function AppearanceProvider({ children }) {
  const [settings, setSettings] = useState(readStoredAppearance);
  const settingsRef = useRef(settings);

  const acceptAppearance = useCallback((value) => {
    const next = normalizeAppearance(value);
    settingsRef.current = next;
    setSettings(next);
    storeAppearance(next);
    applyAppearance(next);
    return next;
  }, []);

  const updateAppearance = useCallback((changes) => {
    const next = acceptAppearance({ ...settingsRef.current, ...changes });
    const request = window.noteDesktop?.updateAppearance?.(next);
    request?.catch(() => {
      // 保留本地预览；主进程错误不会让设置界面失去响应。
    });
    return next;
  }, [acceptAppearance]);

  useLayoutEffect(() => {
    applyAppearance(settings);
  }, [settings]);

  useEffect(() => {
    let disposed = false;
    const removeDesktopListener = window.noteDesktop?.onAppearanceChanged?.((next) => {
      if (!disposed) acceptAppearance(next);
    });

    window.noteDesktop?.getAppearance?.()
      .then((next) => {
        if (!disposed) acceptAppearance(next);
      })
      .catch(() => {});

    function handleStorage(event) {
      if (event.key !== APPEARANCE_STORAGE_KEY || !event.newValue) return;
      try {
        acceptAppearance(JSON.parse(event.newValue));
      } catch {
        // 忽略其他上下文写入的无效值。
      }
    }

    window.addEventListener("storage", handleStorage);
    return () => {
      disposed = true;
      removeDesktopListener?.();
      window.removeEventListener("storage", handleStorage);
    };
  }, [acceptAppearance]);

  const value = useMemo(() => ({ settings, updateAppearance }), [settings, updateAppearance]);
  return <AppearanceContext.Provider value={value}>{children}</AppearanceContext.Provider>;
}

export function useAppearance() {
  const context = useContext(AppearanceContext);
  if (!context) throw new Error("useAppearance must be used inside AppearanceProvider");
  return context;
}

export function AppearanceSettingsForm({ onDone }) {
  const { settings, updateAppearance } = useAppearance();
  const backgroundInputRef = useRef(null);
  const themeInputRef = useRef(null);
  const [backgroundDraft, setBackgroundDraft] = useState(settings.backgroundColor);
  const [themeDraft, setThemeDraft] = useState(settings.themeColor);
  const isDefault = settings.backgroundColor === DEFAULT_APPEARANCE.backgroundColor
    && settings.themeColor === DEFAULT_APPEARANCE.themeColor
    && settings.opacity === DEFAULT_APPEARANCE.opacity;

  useEffect(() => {
    setBackgroundDraft(settings.backgroundColor);
  }, [settings.backgroundColor]);

  useEffect(() => {
    setThemeDraft(settings.themeColor);
  }, [settings.themeColor]);

  useEffect(() => {
    const backgroundInput = backgroundInputRef.current;
    const themeInput = themeInputRef.current;

    const commitBackground = (event) => {
      updateAppearance({ backgroundColor: event.currentTarget.value });
    };
    const commitTheme = (event) => {
      updateAppearance({ themeColor: event.currentTarget.value });
    };

    backgroundInput?.addEventListener("change", commitBackground);
    themeInput?.addEventListener("change", commitTheme);
    return () => {
      backgroundInput?.removeEventListener("change", commitBackground);
      themeInput?.removeEventListener("change", commitTheme);
    };
  }, [updateAppearance]);

  const finish = () => {
    updateAppearance({ backgroundColor: backgroundDraft, themeColor: themeDraft });
    onDone?.();
  };

  const reset = () => {
    setBackgroundDraft(DEFAULT_APPEARANCE.backgroundColor);
    setThemeDraft(DEFAULT_APPEARANCE.themeColor);
    updateAppearance(DEFAULT_APPEARANCE);
  };

  return (
    <div className="appearance-settings-form">
      <div className="appearance-setting-list">
        <label className="appearance-color-setting" htmlFor="appearance-background-color">
          <span>
            <strong>背景颜色</strong>
          </span>
          <span className="appearance-color-control">
            <input
              ref={backgroundInputRef}
              id="appearance-background-color"
              type="color"
              value={backgroundDraft}
              onInput={(event) => setBackgroundDraft(event.currentTarget.value)}
              onChange={() => {}}
            />
            <output htmlFor="appearance-background-color">{backgroundDraft}</output>
          </span>
        </label>

        <label className="appearance-color-setting" htmlFor="appearance-theme-color">
          <span>
            <strong>主题颜色</strong>
          </span>
          <span className="appearance-color-control">
            <input
              ref={themeInputRef}
              id="appearance-theme-color"
              type="color"
              value={themeDraft}
              onInput={(event) => setThemeDraft(event.currentTarget.value)}
              onChange={() => {}}
            />
            <output htmlFor="appearance-theme-color">{themeDraft}</output>
          </span>
        </label>

        <label className="appearance-opacity-setting" htmlFor="appearance-opacity">
          <span>
            <strong>不透明度</strong>
          </span>
          <output htmlFor="appearance-opacity">{settings.opacity}%</output>
          <input
            id="appearance-opacity"
            type="range"
            min="20"
            max="100"
            step="1"
            value={settings.opacity}
            style={{ "--range-value": `${settings.opacity}%` }}
            onChange={(event) => updateAppearance({ opacity: event.target.value })}
          />
        </label>
      </div>

      <footer className="appearance-settings-footer">
        <button
          className="appearance-reset-button"
          type="button"
          disabled={isDefault}
          onClick={reset}
        >
          <ArrowCounterClockwise size={17} aria-hidden="true" />
          恢复默认
        </button>
        <button className="appearance-done-button" type="button" onClick={finish}>
          <CheckCircle size={18} weight="fill" aria-hidden="true" />
          完成
        </button>
      </footer>
    </div>
  );
}
