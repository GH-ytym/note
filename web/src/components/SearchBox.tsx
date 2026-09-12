import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { ArrowLeft, ArrowUpRight, MagnifyingGlass, X } from "@phosphor-icons/react";
import { createPortal } from "react-dom";
import useWindowDrag from "../hooks/useWindowDrag";
import { searchTodo, searchEvent, searchAll } from "../api";
import type { SearchItem, SearchPage } from "../types";

interface Props {
  open: boolean;
  onOpen: (open: boolean) => void;
  onSelect: (item: SearchItem) => void;
  limit: number;
  defaultSplit: boolean;
}

export default function SearchBox({ open, onOpen, onSelect, limit, defaultSplit }: Props) {
  const drag = useWindowDrag(true);
  const [keyword, setKeyword] = useState("");
  const [expanded, setExpanded] = useState(false);
  const [split, setSplit] = useState(defaultSplit);
  const [pages, setPages] = useState<SearchPage[]>([]);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const input = useRef<HTMLInputElement>(null);
  const panel = useRef<HTMLDivElement>(null);
  const root = useRef<HTMLDivElement>(null);
  const [position, setPosition] = useState({ top: 44, left: 8, width: 460 });
  useLayoutEffect(() => {
    if (!open) return;
    const place = () => {
      const rect = root.current?.getBoundingClientRect();
      setPosition({ top: (rect?.bottom ?? 40) + 6, left: Math.min(rect?.left ?? 8, Math.max(8, window.innerWidth - 468)), width: Math.min(460, window.innerWidth - 16) });
    };
    place();
    window.addEventListener("resize", place);
    return () => window.removeEventListener("resize", place);
  }, [open]);
  useEffect(() => {
    if (!open || !expanded) return;
    const calendar = document.querySelector<HTMLElement>(".calendar-shell");
    if (calendar) calendar.inert = true;
    panel.current?.querySelector<HTMLButtonElement>("button")?.focus();
    return () => { if (calendar) calendar.inert = false; };
  }, [open, expanded]);
  useEffect(() => setSplit(defaultSplit), [defaultSplit]);
  useEffect(() => { if (!open) { setExpanded(false); setPage(1); } }, [open]);
  useEffect(() => {
    if (!open) return;
    function outside(event: PointerEvent) {
      if (event.target instanceof Node && !root.current?.contains(event.target) && !panel.current?.contains(event.target)) onOpen(false);
    }
    document.addEventListener("pointerdown", outside);
    return () => document.removeEventListener("pointerdown", outside);
  }, [open, onOpen]);
  useEffect(() => {
    const controller = new AbortController();
    setPages([]);
    setError("");
    if (!open || !keyword.trim()) { setLoading(false); return; }
    setLoading(true);
    // Query each text change; abort and ignore an obsolete response.
    const request = expanded && !split
      ? searchAll(keyword, limit, page, controller.signal).then(result => [result])
      : Promise.all([searchTodo(keyword, limit, page, controller.signal), searchEvent(keyword, limit, page, controller.signal)]);
    request.then(result => {
      if (!controller.signal.aborted) setPages(result);
    }).catch((reason: unknown) => {
      if (!controller.signal.aborted) setError(reason instanceof Error ? reason.message : "搜索失败");
    }).finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [keyword, open, expanded, split, limit, page]);

  function select(item: SearchItem) { onOpen(false); onSelect(item); }
  const totalPages = Math.max(1, ...pages.map(result => Math.ceil(result.total / limit)));
  function results(result: SearchPage | undefined, title: string) {
    return <section className="search-group" aria-label={title}>
      <h3>{title}<span>{result?.total ?? 0}</span></h3>
      {result?.items.map(item => <button type="button" className="search-result" key={`${item.kind}-${item.id}`} onClick={() => select(item)}>
        <span className="search-kind" style={{ color: item.color }}>{item.kind === "todo" ? "○" : "▰"}</span>
        <span><strong>{item.title}</strong><small>{item.content}</small></span>
        <time>{new Date(item.starts_at).toLocaleDateString("zh-CN", { timeZone: "Asia/Shanghai" })}</time>
      </button>)}
      {!loading && !error && !result?.items.length && <p className="search-empty">没有匹配结果</p>}
    </section>;
  }
  return <div className="workspace-search" ref={root} onKeyDown={event => {
    if (expanded && event.key === "Tab") {
      const focusable = panel.current?.querySelectorAll<HTMLElement>("button:not(:disabled),input");
      const first = focusable?.[0], last = focusable?.[focusable.length - 1];
      if (event.shiftKey && event.target === first) { event.preventDefault(); last?.focus(); }
      else if (!event.shiftKey && event.target === last) { event.preventDefault(); first?.focus(); }
    }
    if (event.key === "Escape") { event.stopPropagation(); input.current?.focus(); onOpen(false); }
    if (event.key === "ArrowDown" && event.target === input.current) {
      event.preventDefault(); panel.current?.querySelector<HTMLButtonElement>(".search-result")?.focus();
    }
  }}>
    <MagnifyingGlass size={16} aria-hidden="true" />
    <input ref={input} aria-label="搜索待办和日程" placeholder="搜索待办和日程" value={keyword}
      aria-expanded={open} aria-controls="search-results" onFocus={() => { if (keyword.trim()) onOpen(true); }}
      onChange={event => { setKeyword(event.target.value); setPage(1); onOpen(Boolean(event.target.value.trim())); }} />
    {keyword && <button aria-label="清空搜索" onClick={() => { setKeyword(""); onOpen(false); input.current?.focus(); }}><X size={14} /></button>}
    {open && createPortal(<div ref={panel} id="search-results" className={`search-panel ${expanded ? "is-expanded" : "is-compact"}`} style={!expanded ? position : undefined} role="dialog" aria-modal={expanded || undefined} aria-label={expanded ? "详细搜索结果" : "搜索结果"} aria-busy={loading}>
      <header {...drag}>{expanded && <button aria-label="返回日历" onClick={() => onOpen(false)}><ArrowLeft size={18} />返回</button>}<strong>{expanded ? "搜索结果" : "快速搜索"}</strong>
        {expanded && <label><input type="checkbox" checked={split} onChange={event => { setSplit(event.target.checked); setPage(1); }} />{split ? "分栏显示" : "合并显示"}</label>}
        <button aria-label="关闭搜索面板" onClick={() => onOpen(false)}><X size={16} /></button>
      </header>
      {loading && <p role="status">正在搜索…</p>}
      {error && <p role="alert">{error}</p>}
      <div className={`search-groups ${expanded && split ? "is-split" : ""}`}>
        {expanded && !split ? results(pages[0], "Todo / Event") : <>{results(pages[0], "Todo")}<hr />{results(pages[1], "Event")}</>}
      </div>
      <footer>
        {expanded ? <><button disabled={loading || page <= 1} onClick={() => setPage(value => value - 1)}>上一页</button><span>{page} / {totalPages}</span><button disabled={loading || page >= totalPages} onClick={() => setPage(value => value + 1)}>下一页</button></>
          : <><span>每类最多 {limit} 条</span><button className="expand-search" aria-label="展开详细搜索结果" title="展开详细搜索结果" onClick={() => { setExpanded(true); setSplit(defaultSplit); setPage(1); }}><ArrowUpRight size={20} /></button></>}
      </footer>
    </div>, document.body)}
  </div>;
}
