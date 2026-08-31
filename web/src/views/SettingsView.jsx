import { AppearanceSettingsForm } from "../appearance";
import WindowFrame from "../windows/WindowFrame";
import { closeCurrentWindow } from "../windows/window-utils";

export default function SettingsView({ onDone }) {
  return (
    <WindowFrame title="外观设置" className="is-settings-window" onBack={onDone}>
      <AppearanceSettingsForm onDone={onDone || closeCurrentWindow} />
    </WindowFrame>
  );
}
