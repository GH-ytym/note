import {
  useCallback,
  useContext,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  createContext,
} from "react";
import { ArrowCounterClockwise, CheckCircle } from "@phosphor-icons/react";

import {
  DEFAULT_APPEARANCE,
  normalizeAppearance,
  appearanceTokens,
  foreground,
  luminance,
} from "./lib/appearance";
export { DEFAULT_APPEARANCE, normalizeAppearance } from "./lib/appearance";

const APPEARANCE_STORAGE_KEY = "note.appearance.v1";
const AppearanceContext = createContext(null);

function applyAppearance(settings) {
  const root = document.documentElement;
  for (const [name, value] of Object.entries(appearanceTokens(settings)))
    root.style.setProperty(name, value);
  root.style.colorScheme =
    luminance(foreground(settings.backgroundColor)) < 0.5 ? "light" : "dark";
}

function readStoredAppearance() {
  try {
    const stored = window.localStorage.getItem(APPEARANCE_STORAGE_KEY);
    return stored
      ? normalizeAppearance(JSON.parse(stored))
      : DEFAULT_APPEARANCE;
  } catch {
    return DEFAULT_APPEARANCE;
  }
}

function storeAppearance(settings) {
  try {
    window.localStorage.setItem(
      APPEARANCE_STORAGE_KEY,
      JSON.stringify(settings),
    );
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

  const updateAppearance = useCallback(
    (changes) => {
      const next = acceptAppearance({ ...settingsRef.current, ...changes });
      const request = window.noteDesktop?.updateAppearance?.(next);
      request?.catch(() => {
        // 保留本地预览；主进程错误不会让设置界面失去响应。
      });
      return next;
    },
    [acceptAppearance],
  );

  useLayoutEffect(() => {
    applyAppearance(settings);
  }, [settings]);

  useEffect(() => {
    let disposed = false;
    const removeDesktopListener = window.noteDesktop?.onAppearanceChanged?.(
      (next) => {
        if (!disposed) acceptAppearance(next);
      },
    );

    window.noteDesktop
      ?.getAppearance?.()
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

  const value = useMemo(
    () => ({ settings, updateAppearance }),
    [settings, updateAppearance],
  );
  return (
    <AppearanceContext.Provider value={value}>
      {children}
    </AppearanceContext.Provider>
  );
}

export function useAppearance() {
  const context = useContext(AppearanceContext);
  if (!context)
    throw new Error("useAppearance must be used inside AppearanceProvider");
  return context;
}

export function AppearanceSettingsForm({ onDone }) {
  const { settings, updateAppearance } = useAppearance();
  const backgroundInputRef = useRef(null);
  const themeInputRef = useRef(null);
  const [backgroundDraft, setBackgroundDraft] = useState(
    settings.backgroundColor,
  );
  const [themeDraft, setThemeDraft] = useState(settings.themeColor);
  const isDefault =
    settings.backgroundColor === DEFAULT_APPEARANCE.backgroundColor &&
    settings.themeColor === DEFAULT_APPEARANCE.themeColor &&
    settings.opacity === DEFAULT_APPEARANCE.opacity;

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
    updateAppearance({
      backgroundColor: backgroundDraft,
      themeColor: themeDraft,
    });
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
        <label
          className="appearance-color-setting"
          htmlFor="appearance-background-color"
        >
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
            <output htmlFor="appearance-background-color">
              {backgroundDraft}
            </output>
          </span>
        </label>

        <label
          className="appearance-color-setting"
          htmlFor="appearance-theme-color"
        >
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

        <label
          className="appearance-opacity-setting"
          htmlFor="appearance-opacity"
        >
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
            onChange={(event) =>
              updateAppearance({ opacity: event.target.value })
            }
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
        <button
          className="appearance-done-button"
          type="button"
          onClick={finish}
        >
          <CheckCircle size={18} weight="fill" aria-hidden="true" />
          完成
        </button>
      </footer>
    </div>
  );
}
