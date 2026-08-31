import { ArrowLeft } from "@phosphor-icons/react";
import WindowControls from "../components/WindowControls";

export default function WindowFrame({
  title,
  subtitle,
  children,
  className = "",
  closeLabel = "关闭当前窗口",
  onBack,
}) {
  return (
    <main className={`utility-window ${className}`}>
      <section className="utility-panel">
        <header className="side-header utility-header">
          <div>
            <h1>{title}</h1>
            {subtitle && <small>{subtitle}</small>}
          </div>
          {onBack ? (
            <button className="workspace-back-button" type="button" onClick={onBack} aria-label="返回当天日程">
              <ArrowLeft size={19} weight="bold" aria-hidden="true" />
            </button>
          ) : (
            <WindowControls closeLabel={closeLabel} />
          )}
        </header>
        {children}
      </section>
    </main>
  );
}

export function LoadingWindow({ title, message = "正在读取…", onBack }) {
  return (
    <WindowFrame title={title} onBack={onBack}>
      <div className="utility-state" role="status"><span />{message}</div>
    </WindowFrame>
  );
}
