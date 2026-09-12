import { useRef, useState } from "react";
import type { CalendarItem } from "../types";
import { dragTimeLabel, type TimeEdge } from "../lib/drag-time";

export default function useTimelineDrag(horizontal: boolean, saving: boolean,
  onRetime: (item: CalendarItem, edge: TimeEdge, minutes: number) => Promise<void>,
  onOpen: (item: CalendarItem) => void) {
  const active = useRef<{item: CalendarItem; edge: TimeEdge; start: number; scroll: HTMLElement; offset: number; moved: boolean; delta: number} | null>(null);
  const suppress = useRef(false);
  const [preview, setPreview] = useState<{item: CalendarItem; edge: TimeEdge; delta: number} | null>(null);
  function finish(cancelled = false) {
    const drag = active.current;
    if (!drag) return;
    active.current = null;
    setPreview(null);
    suppress.current = Boolean(drag?.moved);
    if (!cancelled && drag?.moved && drag.delta) void onRetime(drag.item, drag.edge, drag.delta);
  }
  function move(e: React.PointerEvent) {
    const drag = active.current;
    if (!drag) return;
    const coordinate = horizontal ? e.clientX : e.clientY;
    const bounds = drag.scroll.getBoundingClientRect();
    const near = horizontal ? bounds.left : bounds.top;
    const far = horizontal ? bounds.right : bounds.bottom;
    if (Math.abs(coordinate - drag.start) > 5) drag.moved = true;
    if (!drag.moved) return;
    const shift = coordinate < near + 35 ? -15 : coordinate > far - 35 ? 15 : 0;
    if (horizontal) drag.scroll.scrollLeft += shift;
    else drag.scroll.scrollTop += shift;
    const offset = horizontal ? drag.scroll.scrollLeft : drag.scroll.scrollTop;
    drag.delta = Math.round((coordinate - drag.start + offset - drag.offset) / 5) * 5;
    setPreview({item: drag.item, edge: drag.edge, delta: drag.delta});
  }
  const interaction = (item: CalendarItem) => ({
    onPointerDown(e: React.PointerEvent<HTMLElement>) {
      if (saving || e.button !== 0) return;
      const scroll = e.currentTarget.closest<HTMLElement>(".calendar-timeline");
      if (!scroll) return;
      const handle = (e.target as Element).closest<HTMLElement>("[data-time-edge]");
      const edge: TimeEdge = handle?.dataset.timeEdge === "end" ? "end" : "start";
      e.preventDefault();
      e.stopPropagation();
      e.currentTarget.focus({preventScroll: true});
      e.currentTarget.setPointerCapture(e.pointerId);
      suppress.current = false;
      active.current = {item, edge, start: horizontal ? e.clientX : e.clientY, scroll,
        offset: horizontal ? scroll.scrollLeft : scroll.scrollTop, moved: false, delta: 0};
    },
    onPointerMove: move,
    onPointerUp: () => finish(),
    onPointerCancel: () => finish(true),
    onLostPointerCapture: () => { if (active.current) finish(true); },
    onClick() { if (!suppress.current) onOpen(item); suppress.current = false; },
    onKeyDown(e: React.KeyboardEvent) {
      if (e.key === "Escape") { e.preventDefault(); finish(true); }
      else if (!saving && ["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight"].includes(e.key)) {
        e.preventDefault();
        void onRetime(item, e.shiftKey && item.kind === "event" ? "end" : "start", ["ArrowUp", "ArrowLeft"].includes(e.key) ? -5 : 5);
      }
    },
  });
  return {interaction, preview, label: preview ? `${preview.item.title} · ${preview.edge === "end" ? "结束" : "开始"} ${dragTimeLabel(preview.item, preview.edge, preview.delta)}` : ""};
}
