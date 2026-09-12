import { useRef } from "react";

/** Drag only the title/background, never inputs, buttons, or result panels. */
export default function useWindowDrag(allowDialog = false) {
  const dragging = useRef(false);
  const finish = () => {
    if (!dragging.current) return;
    dragging.current = false;
    void window.noteDesktop?.endWindowDrag();
  };
  return {
    onPointerDown(event: React.PointerEvent<HTMLElement>) {
      if (!window.noteDesktop || event.button !== 0 || (event.target instanceof Element && event.target.closest("button,input,label,select,textarea,.workspace-search" + (allowDialog ? "" : ",[role=dialog]")))) return;
      event.preventDefault();
      dragging.current = true;
      event.currentTarget.setPointerCapture(event.pointerId);
      void window.noteDesktop.startWindowDrag({ x: event.screenX, y: event.screenY });
    },
    onPointerMove(event: React.PointerEvent<HTMLElement>) {
      if (dragging.current) void window.noteDesktop?.moveWindowDrag({ x: event.screenX, y: event.screenY });
    },
    onPointerUp: finish, onPointerCancel: finish, onLostPointerCapture: finish,
  };
}
