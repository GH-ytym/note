import { useRef } from "react";

export default function MiniDragHandle() {
  const dragging = useRef(false);
  function finish() {
    if (!dragging.current) return;
    dragging.current = false;
    void window.noteDesktop?.endWindowDrag();
  }
  return <div className="mini-drag-handle" title="拖动窗口" aria-label="拖动窗口"
    onPointerDown={event => {
      if (event.button !== 0) return;
      event.preventDefault();
      dragging.current = true;
      event.currentTarget.setPointerCapture(event.pointerId);
      void window.noteDesktop?.startWindowDrag({ x: event.screenX, y: event.screenY });
    }}
    onPointerMove={event => { if (dragging.current) void window.noteDesktop?.moveWindowDrag({ x: event.screenX, y: event.screenY }); }}
    onPointerUp={finish} onPointerCancel={finish} onLostPointerCapture={finish} />;
}
