import { useEffect, useState } from "react";
import { Minus, Square, CopySimple, X } from "@phosphor-icons/react";
import { IS_DESKTOP } from "../lib/calendar";

export default function WindowControls({ variant = "close", closeLabel = "关闭当前窗口" }: { variant?: "close" | "collapse" | "hide-app"; closeLabel?: string }) {
  const [maximized, setMaximized] = useState(false);
  useEffect(() => {
    const desktop = window.noteDesktop;
    if (!desktop) return;
    let active = true;
    let stateChanged = false;
    const unsubscribe = desktop.onMaximizedChanged(value => {
      stateChanged = true;
      if (active) setMaximized(value);
    });
    void desktop.getWindowState().then(state => {
      if (active && !stateChanged) setMaximized(state.maximized);
    }).catch(() => {});
    return () => { active = false; unsubscribe(); };
  }, []);
  if (!IS_DESKTOP) return null;
  const hidesApplication = variant === "hide-app";
  const label = variant === "collapse" ? "收回日历" : hidesApplication ? "隐藏到后台" : closeLabel;

  return (
    <div className="window-controls" aria-label="窗口控制">
      <button type="button" onClick={() => void window.noteDesktop?.minimizeCurrent()} aria-label="最小化" title="最小化">
        <Minus size={17} />
      </button>
      <button type="button" onClick={() => void window.noteDesktop?.toggleMaximizeCurrent()} aria-label={maximized ? "还原窗口" : "最大化"} title={maximized ? "还原窗口" : "最大化"}>
        {maximized ? <CopySimple size={16} /> : <Square size={16} />}
      </button>
      <button
        className={variant === "collapse" ? "" : "is-close"}
        type="button"
        onClick={() => void window.noteDesktop?.closeCurrent()}
        aria-label={label}
        title={label}
      >
        {variant === "collapse" ? <Minus size={16} weight="bold" /> : <X size={17} />}
      </button>
    </div>
  );
}
