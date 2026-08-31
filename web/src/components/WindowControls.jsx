import { Minus, X } from "@phosphor-icons/react";
import { IS_DESKTOP } from "../lib/calendar";

export default function WindowControls({ variant = "close", closeLabel = "关闭当前窗口" }) {
  if (!IS_DESKTOP) return null;
  const hidesApplication = variant === "hide-app";
  const label = variant === "collapse" ? "收回日历" : hidesApplication ? "隐藏到后台" : closeLabel;

  return (
    <div className="window-controls" aria-label="窗口控制">
      <button
        className={hidesApplication ? "is-close" : ""}
        type="button"
        onClick={() => void window.noteDesktop.closeCurrent()}
        aria-label={label}
        title={label}
      >
        {hidesApplication ? <X size={17} /> : <Minus size={16} weight="bold" />}
      </button>
    </div>
  );
}
