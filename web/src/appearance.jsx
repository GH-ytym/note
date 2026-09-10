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
  DEFAULT_PREFERENCES,
  DEFAULT_SETTINGS,
  normalizeSettings,
  appearanceTokens,
  foreground,
  luminance,
} from "./lib/appearance";
export {
  DEFAULT_APPEARANCE,
  DEFAULT_PREFERENCES,
  DEFAULT_SETTINGS,
  normalizeSettings,
} from "./lib/appearance";

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
      ? normalizeSettings(JSON.parse(stored))
      : DEFAULT_SETTINGS;
  } catch {
    return DEFAULT_SETTINGS;
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
    const next = normalizeSettings(value);
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

const VIEW_OPTIONS = [
  ["year", "年视图"],
  ["month", "月视图"],
  ["week", "周视图"],
  ["day", "日视图"],
];

function SettingsFooter({ isDefault, onReset, onDone }) {
  return (
    <footer className="appearance-settings-footer">
      <button
        className="appearance-reset-button"
        type="button"
        disabled={isDefault}
        onClick={onReset}
      >
        <ArrowCounterClockwise size={17} aria-hidden="true" />
        恢复默认
      </button>
      <button className="appearance-done-button" type="button" onClick={onDone}>
        <CheckCircle size={18} weight="fill" aria-hidden="true" />
        完成
      </button>
    </footer>
  );
}

function SettingSelect({ id, label, value, options, onChange }) {
  return (
    <label className="settings-choice" htmlFor={id}>
      <strong>{label}</strong>
      <select id={id} value={value} onChange={onChange}>
        {options.map(([optionValue, optionLabel]) => (
          <option value={optionValue} key={optionValue}>
            {optionLabel}
          </option>
        ))}
      </select>
    </label>
  );
}

export function PreferenceSettingsForm({ onDone }) {
  const { settings, updateAppearance } = useAppearance();
  const isDefault =
    settings.defaultView === DEFAULT_PREFERENCES.defaultView &&
    settings.dayViewMode === DEFAULT_PREFERENCES.dayViewMode &&
    settings.handMode === DEFAULT_PREFERENCES.handMode;

  return (
    <div className="settings-section-form">
      <div className="settings-choice-list">
        <SettingSelect
          id="preference-default-view"
          label="默认视图"
          value={settings.defaultView}
          options={VIEW_OPTIONS}
          onChange={(event) =>
            updateAppearance({ defaultView: event.target.value })
          }
        />
        <SettingSelect
          id="preference-day-view"
          label="日视图模式"
          value={settings.dayViewMode}
          options={[
            ["clock", "时钟"],
            ["timeline", "时间轴"],
          ]}
          onChange={(event) =>
            updateAppearance({ dayViewMode: event.target.value })
          }
        />
        <SettingSelect
          id="preference-clock-hands"
          label="时针模式"
          value={settings.handMode}
          options={[
            ["full", "完整时针"],
            ["compact", "精简时针"],
          ]}
          onChange={(event) =>
            updateAppearance({ handMode: event.target.value })
          }
        />
      </div>
      <SettingsFooter
        isDefault={isDefault}
        onReset={() => updateAppearance(DEFAULT_PREFERENCES)}
        onDone={onDone}
      />
    </div>
  );
}

export function ConfigurationSettingsForm({ onDone }) {
  const { settings, updateAppearance } = useAppearance();
  const isDefault =
    settings.weekOrientation === DEFAULT_PREFERENCES.weekOrientation &&
    settings.dayOrientation === DEFAULT_PREFERENCES.dayOrientation &&
    settings.clockTracks === DEFAULT_PREFERENCES.clockTracks &&
    settings.wakeStyle === DEFAULT_PREFERENCES.wakeStyle;
  const orientationOptions = [
    ["vertical", "横向"],
    ["horizontal", "纵向"],
  ];

  return (
    <div className="settings-section-form">
      <div className="settings-choice-list">
        <SettingSelect
          id="configuration-week-orientation"
          label="周视图轨道"
          value={settings.weekOrientation}
          options={orientationOptions}
          onChange={(event) =>
            updateAppearance({ weekOrientation: event.target.value })
          }
        />
        <SettingSelect
          id="configuration-wake-style"
          label="后台唤醒样式"
          value={settings.wakeStyle}
          options={[["mini", "小窗模式"], ["last", "上一次关闭时的样式"], ["year", "年视图"], ["month", "月视图"], ["week", "周视图"], ["timeline", "日视图 · 时间轴"], ["clock", "日视图 · 时钟"]]}
          onChange={(event) => updateAppearance({ wakeStyle: event.target.value })}
        />
        <SettingSelect
          id="configuration-day-orientation"
          label="日视图轨道"
          value={settings.dayOrientation}
          options={orientationOptions}
          onChange={(event) =>
            updateAppearance({ dayOrientation: event.target.value })
          }
        />
        <label className="settings-choice settings-track-count" htmlFor="configuration-clock-tracks">
          <strong>轨道数</strong>
          <output htmlFor="configuration-clock-tracks">{settings.clockTracks}</output>
          <input
            id="configuration-clock-tracks"
            type="range"
            min="3"
            max="10"
            step="1"
            value={settings.clockTracks}
            style={{ "--range-value": `${((settings.clockTracks - 3) / 7) * 100}%` }}
            onChange={(event) =>
              updateAppearance({ clockTracks: event.target.value })
            }
          />
        </label>
      </div>
      <SettingsFooter
        isDefault={isDefault}
        onReset={() =>
          updateAppearance({
            weekOrientation: DEFAULT_PREFERENCES.weekOrientation,
            dayOrientation: DEFAULT_PREFERENCES.dayOrientation,
            clockTracks: DEFAULT_PREFERENCES.clockTracks,
            wakeStyle: DEFAULT_PREFERENCES.wakeStyle,
          })
        }
        onDone={onDone}
      />
    </div>
  );
}
