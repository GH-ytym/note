import type { Icon } from "@phosphor-icons/react";
import { useState } from "react";
import {
  GearSix,
  Palette,
  SlidersHorizontal,
} from "@phosphor-icons/react";
import {
  AppearanceSettingsForm,
  ConfigurationSettingsForm,
  PreferenceSettingsForm,
} from "../appearance";
import WindowFrame from "../windows/WindowFrame";
import { closeCurrentWindow } from "../windows/window-utils";

export default function SettingsView({ onDone }: { onDone?: () => void }) {
  const [section, setSection] = useState("appearance");
  const finish = onDone || closeCurrentWindow;
  const sections: [string, string, Icon][] = [
    ["appearance", "外观", Palette],
    ["preferences", "首选项", SlidersHorizontal],
    ["configuration", "配置项", GearSix],
  ];
  return (
    <WindowFrame title="设置" className="is-settings-window" onBack={onDone}>
      <div className="settings-layout">
        <nav className="settings-sidebar" aria-label="设置分类">
          {sections.map(([value, label, Icon]) => (
            <button
              type="button"
              key={value}
              className={section === value ? "is-active" : ""}
              aria-current={section === value ? "page" : undefined}
              onClick={() => setSection(value)}
            >
              <Icon size={18} aria-hidden="true" />
              {label}
            </button>
          ))}
        </nav>
        <section className="settings-content" aria-labelledby={`settings-${section}-title`}>
          <h2 id={`settings-${section}-title`}>
            {sections.find(([value]) => value === section)?.[1]}
          </h2>
          {section === "appearance" && <AppearanceSettingsForm onDone={finish} />}
          {section === "preferences" && <PreferenceSettingsForm onDone={finish} />}
          {section === "configuration" && (
            <ConfigurationSettingsForm onDone={finish} />
          )}
        </section>
      </div>
    </WindowFrame>
  );
}
