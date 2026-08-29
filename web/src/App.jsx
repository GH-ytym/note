import { useEffect, useMemo, useRef, useState } from "react";
import {
  CalendarBlank,
  CaretDown,
  CaretUp,
  Check,
  Clock,
  DotsThree,
  GearSix,
  Minus,
  Palette,
  Plus,
  Sparkle,
  Trash,
  X,
} from "@phosphor-icons/react";
import { createTodo, deleteTodo, getCalendar, patchOccurrence, patchTodo } from "./api";

const WEEKDAYS = ["一", "二", "三", "四", "五", "六", "日"];
const SHANGHAI_TIME_ZONE = "Asia/Shanghai";

const REPEAT_VALUES = {
  仅一次: "once",
  每天: "daily",
  工作日: "weekdays",
  周末: "weekends",
  每周: "weekly",
  每月: "monthly",
  自定义: "custom",
};

const REPEAT_LABELS = Object.fromEntries(
  Object.entries(REPEAT_VALUES).map(([label, value]) => [value, label]),
);

const REMINDER_VALUES = {
  静默提醒: "silent",
  弹窗提醒: "popup",
};

const REMINDER_LABELS = {
  silent: "静默提醒",
  popup: "弹窗提醒",
};

const EVENT_COLORS = [
  { value: "#F3B51B", label: "琥珀" },
  { value: "#F47C48", label: "珊瑚" },
  { value: "#E95B78", label: "莓红" },
  { value: "#A879F2", label: "鸢尾" },
  { value: "#5B8DEF", label: "钴蓝" },
  { value: "#35B7A0", label: "薄荷" },
  { value: "#82B94B", label: "青柠" },
];

const START_DATE_REPEATS = new Set(["仅一次", "每周", "每月"]);

const EMPTY_FORM = {
  content: "",
  reminder: "弹窗提醒",
  repeat: "仅一次",
  color: "",
  date: "",
  time: "09:00",
};

function pad(value) {
  return String(value).padStart(2, "0");
}

function dateKeyFromUTC(date) {
  return `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(date.getUTCDate())}`;
}

function dateFromKey(key) {
  const [year, month, day] = key.split("-").map(Number);
  return new Date(Date.UTC(year, month - 1, day));
}

function monthFromKey(key) {
  const date = dateFromKey(key);
  return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), 1));
}

function addDays(date, amount) {
  return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate() + amount));
}

function addMonths(date, amount) {
  return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth() + amount, 1));
}

function getShanghaiTodayKey() {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: SHANGHAI_TIME_ZONE,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(new Date());
  const value = (type) => parts.find((part) => part.type === type)?.value;
  return `${value("year")}-${value("month")}-${value("day")}`;
}

const TODAY_KEY = getShanghaiTodayKey();
const IS_DESKTOP = Boolean(window.noteDesktop?.isDesktop);

function isPastDateKey(dateKey) {
  return dateKey < TODAY_KEY;
}

const LUNAR_FORMATTER = new Intl.DateTimeFormat("zh-CN-u-ca-chinese", {
  month: "long",
  day: "numeric",
  timeZone: "UTC",
});

const LUNAR_DAY_LABELS = [
  "",
  "初一", "初二", "初三", "初四", "初五", "初六", "初七", "初八", "初九", "初十",
  "十一", "十二", "十三", "十四", "十五", "十六", "十七", "十八", "十九", "二十",
  "廿一", "廿二", "廿三", "廿四", "廿五", "廿六", "廿七", "廿八", "廿九", "三十",
];

const SOLAR_LABELS = {
  "01-01": "元旦",
  "02-14": "情人节",
  "03-08": "妇女节",
  "05-01": "劳动节",
  "06-01": "儿童节",
  "08-01": "建军节",
  "10-01": "国庆节",
  "12-25": "圣诞节",
};

const SOLAR_TERM_LABELS = {
  "01-05": "小寒", "01-20": "大寒",
  "02-04": "立春", "02-19": "雨水",
  "03-05": "惊蛰", "03-20": "春分",
  "04-05": "清明", "04-20": "谷雨",
  "05-05": "立夏", "05-21": "小满",
  "06-05": "芒种", "06-21": "夏至",
  "07-07": "小暑", "07-23": "大暑",
  "08-07": "立秋", "08-23": "处暑",
  "09-07": "白露", "09-23": "秋分",
  "10-08": "寒露", "10-23": "霜降",
  "11-07": "立冬", "11-22": "小雪",
  "12-07": "大雪", "12-22": "冬至",
};

const LUNAR_FESTIVALS = {
  "正月-1": "春节",
  "正月-15": "元宵节",
  "五月-5": "端午节",
  "七月-7": "七夕节",
  "七月-15": "中元节",
  "八月-15": "中秋节",
  "九月-9": "重阳节",
  "腊月-8": "腊八节",
  "腊月-23": "小年",
};

function monthLabel(month) {
  return `${month.getUTCFullYear()}年${month.getUTCMonth() + 1}月`;
}

function preciseDateLabel(dateKey) {
  const date = dateFromKey(dateKey);
  return `${date.getUTCFullYear()}年${date.getUTCMonth() + 1}月${date.getUTCDate()}日`;
}

function calendarNote(dateKey) {
  const solarKey = dateKey.slice(5);
  if (SOLAR_LABELS[solarKey]) return SOLAR_LABELS[solarKey];
  if (SOLAR_TERM_LABELS[solarKey]) return SOLAR_TERM_LABELS[solarKey];

  const parts = LUNAR_FORMATTER.formatToParts(dateFromKey(dateKey));
  const month = parts.find((part) => part.type === "month")?.value || "";
  const day = Number(parts.find((part) => part.type === "day")?.value || 0);
  const festival = LUNAR_FESTIVALS[`${month}-${day}`];
  if (festival) return festival;
  return day === 1 ? month : LUNAR_DAY_LABELS[day] || "";
}

function buildGridOutlinePath(indices) {
  const cells = new Set(indices);
  const paths = [];

  for (const index of cells) {
    const row = Math.floor(index / 7);
    const column = index % 7;

    if (row === 0 || !cells.has(index - 7)) paths.push(`M ${column} ${row} H ${column + 1}`);
    if (column === 6 || !cells.has(index + 1)) paths.push(`M ${column + 1} ${row} V ${row + 1}`);
    if (row === 5 || !cells.has(index + 7)) paths.push(`M ${column + 1} ${row + 1} H ${column}`);
    if (column === 0 || !cells.has(index - 1)) paths.push(`M ${column} ${row + 1} V ${row}`);
  }

  return paths.join(" ");
}

function splitTime(value) {
  const [hour = "00", minute = "00"] = (value || "00:00").split(":");
  return { hour: Number(hour), minute: Number(minute) };
}

function joinTime(hour, minute) {
  return `${pad((hour + 24) % 24)}:${pad((minute + 60) % 60)}`;
}

function shortMonthLabel(month) {
  return `${month.getUTCFullYear()} / ${pad(month.getUTCMonth() + 1)}`;
}

function calendarRange(month) {
  return {
    from: dateKeyFromUTC(addMonths(month, -1)),
    to: dateKeyFromUTC(addMonths(month, 2)),
  };
}

function buildMonthDays(month) {
  const firstDay = new Date(Date.UTC(month.getUTCFullYear(), month.getUTCMonth(), 1));
  const mondayOffset = (firstDay.getUTCDay() + 6) % 7;
  const firstCell = addDays(firstDay, -mondayOffset);

  return Array.from({ length: 42 }, (_, index) => {
    const date = addDays(firstCell, index);
    return {
      key: dateKeyFromUTC(date),
      day: date.getUTCDate(),
      isCurrentMonth:
        date.getUTCFullYear() === month.getUTCFullYear() &&
        date.getUTCMonth() === month.getUTCMonth(),
    };
  });
}

function buildPeekDays(month, edge) {
  if (edge === "end") {
    return Array.from({ length: 7 }, (_, index) => addDays(month, index).getUTCDate());
  }

  const currentMonth = addMonths(month, 1);
  return Array.from({ length: 7 }, (_, index) => addDays(currentMonth, index - 7).getUTCDate());
}

function dateTimeAt(dateKey, time = "00:00") {
  return new Date(`${dateKey}T${time}:00+08:00`).toISOString();
}

function shanghaiDateTimeParts(value) {
  const date = new Date(value);
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: SHANGHAI_TIME_ZONE,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
  }).formatToParts(date);
  const part = (type) => parts.find((itemPart) => itemPart.type === type)?.value;

  return {
    date: `${part("year")}-${part("month")}-${part("day")}`,
    time: `${part("hour")}:${part("minute")}`,
  };
}

function calendarEventFromOccurrence(item) {
  const occurrence = shanghaiDateTimeParts(item.occurs_at);
  const start = shanghaiDateTimeParts(item.starts_at || item.occurs_at);

  return {
    id: `${item.todo_id}-${item.occurs_at}`,
    todoId: item.todo_id,
    content: item.content,
    color: item.color,
    date: occurrence.date,
    time: occurrence.time,
    startDate: start.date,
    startTime: start.time,
    reminder: REMINDER_LABELS[item.notify_mode] || "弹窗提醒",
    repeat: REPEAT_LABELS[item.repeat_mode] || item.repeat_mode,
    occurrenceDone: Boolean(item.occurrence_done),
    allDone: Boolean(item.all_done),
    version: item.version,
  };
}

function isEventDone(item) {
  return Boolean(item?.allDone || item?.occurrenceDone);
}

function colorWithAlpha(color, alpha) {
  const hex = (color || "#F3B51B").replace("#", "");
  const value = Number.parseInt(hex, 16);
  return `rgb(${(value >> 16) & 255} ${(value >> 8) & 255} ${value & 255} / ${alpha})`;
}

function readableSelectionInk(color) {
  const hex = (color || "#F3B51B").replace("#", "");
  const value = Number.parseInt(hex, 16);
  const red = (value >> 16) & 255;
  const green = (value >> 8) & 255;
  const blue = value & 255;
  const brightness = (red * 299 + green * 587 + blue * 114) / 255000;

  return brightness > 0.58 ? "#171100" : "#F7F9F5";
}

function EventDot({ item, selected, onSelect }) {
  const done = isEventDone(item);

  return (
    <button
      className={`event-dot-button ${done ? "is-complete" : "is-pending"}`}
      type="button"
      style={{
        "--event-color": item.color,
        "--event-soft": colorWithAlpha(item.color, 0.17),
      }}
      aria-label={`${item.content}，${item.time}，${done ? "已完成" : "待完成"}`}
      aria-pressed={selected}
      onClick={(event) => {
        event.stopPropagation();
        onSelect(item);
      }}
    >
      <span className="event-dot" aria-hidden="true" />
      <span className="event-preview" role="tooltip">
        <b>{item.time}</b>
        <span>{item.content}</span>
      </span>
    </button>
  );
}

function CustomColorSwatch({ value, selected, onChange }) {
  return (
    <label className={`palette-swatch custom-color-swatch ${selected ? "is-selected" : ""}`}>
      <Palette size={15} weight="bold" aria-hidden="true" />
      <input
        type="color"
        value={value || "#F3B51B"}
        aria-label="打开调色板"
        onChange={(event) => onChange(event.target.value.toUpperCase())}
      />
    </label>
  );
}

function WindowControls({ primary = false }) {
  if (!IS_DESKTOP) return null;

  return (
    <div className="window-controls" aria-label="窗口控制">
      {primary ? (
        <button className="is-close" type="button" onClick={() => void window.noteDesktop.closeCurrent()} aria-label="隐藏到后台">
          <X size={17} />
        </button>
      ) : (
        <button type="button" onClick={() => void window.noteDesktop.closeCurrent()} aria-label="收起当前区域">
          <Minus size={16} weight="bold" />
        </button>
      )}
    </div>
  );
}

function SelectField({ label, value, options, onChange }) {
  const [open, setOpen] = useState(false);

  return (
    <div
      className="select-field"
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget)) setOpen(false);
      }}
      onKeyDown={(event) => {
        if (event.key !== "Escape" || !open) return;
        event.stopPropagation();
        setOpen(false);
      }}
    >
      <span className="field-label">{label}</span>
      <button
        className="select-trigger"
        type="button"
        aria-label={label}
        aria-haspopup="listbox"
        aria-expanded={open}
        onClick={() => setOpen((current) => !current)}
      >
        <span>{value}</span>
        <CaretDown size={16} weight="bold" aria-hidden="true" />
      </button>

      {open && (
        <div className="select-menu" role="listbox" aria-label={label}>
          {options.map((option) => (
            <button
              type="button"
              role="option"
              aria-selected={option === value}
              onClick={() => {
                onChange(option);
                setOpen(false);
              }}
              key={option}
            >
              <span>{option}</span>
              {option === value && <Check size={14} weight="bold" aria-hidden="true" />}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

function TimeField({ label = "时间", value, onChange, compact = false }) {
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState(value || "00:00");

  useEffect(() => {
    if (!open) setDraft(value || "00:00");
  }, [open, value]);

  function adjust(unit, amount) {
    setDraft((current) => {
      const { hour, minute } = splitTime(current);
      return unit === "hour"
        ? joinTime(hour + amount, minute)
        : joinTime(hour, minute + amount);
    });
  }

  function handleWheel(unit, event) {
    event.preventDefault();
    adjust(unit, event.deltaY > 0 ? 1 : -1);
  }

  const draftParts = splitTime(draft);

  return (
    <div
      className={`time-field ${compact ? "is-compact" : ""}`}
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget)) setOpen(false);
      }}
      onKeyDown={(event) => {
        if (event.key !== "Escape" || !open) return;
        event.stopPropagation();
        setOpen(false);
      }}
    >
      {!compact && <span className="field-label">{label}</span>}
      <button
        className="time-trigger"
        type="button"
        aria-label={label}
        aria-haspopup="dialog"
        aria-expanded={open}
        onClick={() => {
          setDraft(value || "00:00");
          setOpen((current) => !current);
        }}
      >
        <Clock size={16} aria-hidden="true" />
        <span>{value || "00:00"}</span>
        <CaretDown size={14} weight="bold" aria-hidden="true" />
      </button>

      {open && (
        <div className="time-popover" role="dialog" aria-label={`${label}选择器`}>
          <div className="time-dials">
            <div className="time-unit" onWheel={(event) => handleWheel("hour", event)}>
              <button type="button" onClick={() => adjust("hour", 1)} aria-label="小时加一">
                <CaretUp size={16} weight="bold" />
              </button>
              <output>{pad(draftParts.hour)}</output>
              <small>时</small>
              <button type="button" onClick={() => adjust("hour", -1)} aria-label="小时减一">
                <CaretDown size={16} weight="bold" />
              </button>
            </div>

            <span className="time-colon" aria-hidden="true">:</span>

            <div className="time-unit" onWheel={(event) => handleWheel("minute", event)}>
              <button type="button" onClick={() => adjust("minute", 1)} aria-label="分钟加一">
                <CaretUp size={16} weight="bold" />
              </button>
              <output>{pad(draftParts.minute)}</output>
              <small>分</small>
              <button type="button" onClick={() => adjust("minute", -1)} aria-label="分钟减一">
                <CaretDown size={16} weight="bold" />
              </button>
            </div>
          </div>
          <button
            className="time-apply"
            type="button"
            onClick={() => {
              onChange(draft);
              setOpen(false);
            }}
          >
            确定
          </button>
        </div>
      )}
    </div>
  );
}

function DayAgendaItem({ item, onSave, onOpenDetails, onComplete, onDelete }) {
  const [content, setContent] = useState(item.content);
  const [time, setTime] = useState(item.time);
  const [saving, setSaving] = useState(false);
  const [action, setAction] = useState(null);
  const [error, setError] = useState("");
  const done = isEventDone(item);
  const busy = saving || action !== null;

  useEffect(() => {
    setContent(item.content);
    setTime(item.time);
  }, [item.content, item.time]);

  async function commit(changes) {
    setSaving(true);
    setError("");
    try {
      await onSave(item, changes);
    } catch (saveError) {
      setError(saveError.message);
      setContent(item.content);
      setTime(item.time);
    } finally {
      setSaving(false);
    }
  }

  function commitContent() {
    const nextContent = content.trim();
    if (!nextContent) {
      setError("内容不能为空");
      setContent(item.content);
      return;
    }
    if (nextContent !== item.content) void commit({ content: nextContent });
  }

  function commitTime(nextTime) {
    setTime(nextTime);
    if (nextTime !== item.time) void commit({ time: nextTime });
  }

  async function completeForDay() {
    if (done || !onComplete || busy) return;

    setAction("complete");
    setError("");
    try {
      await onComplete(item);
    } catch (completeError) {
      setError(completeError.message);
    } finally {
      setAction(null);
    }
  }

  async function removeItem() {
    if (!onDelete || busy) return;

    setAction("delete");
    setError("");
    try {
      await onDelete(item);
    } catch (deleteError) {
      setError(deleteError.message);
    } finally {
      setAction(null);
    }
  }

  return (
    <article
      className={`agenda-item ${done ? "is-complete" : "is-pending"} ${busy ? "is-saving" : ""} ${onDelete ? "has-delete" : ""}`}
      style={{
        "--agenda-color": item.color,
        "--agenda-soft": colorWithAlpha(item.color, 0.18),
      }}
    >
      <button
        className={`agenda-dot-button ${action === "complete" ? "is-confirming" : ""}`}
        type="button"
        disabled={done || !onComplete || busy}
        onClick={completeForDay}
        aria-label={done ? `${item.content}当天已完成` : `标记${item.content}为当天完成`}
      >
        {action === "complete"
          ? <Check className="agenda-complete-check" size={13} weight="bold" aria-hidden="true" />
          : <span className="agenda-dot" aria-hidden="true" />}
      </button>
      <input
        className="agenda-content"
        value={content}
        maxLength={500}
        disabled={busy}
        aria-label={`${item.content}的内容`}
        onChange={(event) => setContent(event.target.value)}
        onBlur={commitContent}
        onKeyDown={(event) => {
          if (event.key === "Enter") event.currentTarget.blur();
          if (event.key === "Escape") {
            setContent(item.content);
            event.currentTarget.blur();
          }
        }}
      />
      <button
        className="agenda-detail-chevron"
        type="button"
        disabled={busy}
        onClick={() => onOpenDetails(item)}
        aria-label={`进入${item.content}的详情`}
      >
        <span aria-hidden="true">&gt;</span>
      </button>
      <TimeField compact label={`${item.content}的时间`} value={time} onChange={commitTime} />
      {onDelete && (
        <button
          className="agenda-delete-button"
          type="button"
          disabled={busy}
          onClick={removeItem}
          aria-label={`删除${item.content}`}
          title="删除日程"
        >
          <Trash size={16} weight="regular" aria-hidden="true" />
        </button>
      )}
      {error && <small className="agenda-error">{error}</small>}
    </article>
  );
}

function DayAgendaPanel({ dateKey, items, onAdd, onClear, onSave, onOpenCalendar, onOpenDetails, onOpenSettings, onComplete, onDelete, primaryWindow = false, windowControls = false }) {
  const groups = [
    { key: "pending", label: "未完成", items: items.filter((item) => !isEventDone(item)) },
    { key: "complete", label: "已完成", items: items.filter(isEventDone) },
  ].filter((group) => group.items.length > 0);

  return (
    <section
      className={`day-agenda-panel ${dateKey ? "has-selection" : ""}`}
      aria-label={dateKey ? `${preciseDateLabel(dateKey)}的日程` : "日期日程"}
    >
      <header className="day-agenda-panel-header">
        <div className="day-agenda-panel-title">
          <span className="panel-accent" aria-hidden="true" />
          <div>
            <strong>{dateKey ? preciseDateLabel(dateKey) : "日程"}</strong>
            {onOpenCalendar && (
              <button
                className="day-agenda-calendar-button"
                type="button"
                onClick={onOpenCalendar}
                aria-label="打开日历"
              >
                <CalendarBlank size={17} weight="regular" aria-hidden="true" />
              </button>
            )}
            <small>{dateKey ? `${items.length} 条` : "选择一个日期"}</small>
            {onAdd && (
              <button className="day-agenda-inline-add" type="button" onClick={onAdd} aria-label="添加日程">
                <Plus size={18} weight="regular" aria-hidden="true" />
              </button>
            )}
          </div>
        </div>

        <div className="day-agenda-header-actions">
          {onOpenSettings && (
            <button className="day-agenda-settings" type="button" onClick={onOpenSettings} aria-label="打开外观设置" title="外观设置">
              <GearSix size={18} weight="regular" aria-hidden="true" />
            </button>
          )}
          {windowControls ? (
            <WindowControls primary={primaryWindow} />
          ) : dateKey && (
            <button type="button" onClick={onClear} aria-label="清空选中日期">
              <X size={18} />
            </button>
          )}
        </div>
      </header>

      <div className="day-agenda-panel-body">
        {dateKey && items.length > 0 ? (
          <div className="day-agenda-list">
            {groups.map((group) => (
              <section className={`agenda-section is-${group.key}`} key={group.key} aria-label={group.label}>
                <header className="agenda-section-heading">
                  <span>{group.label}</span>
                  <small>{group.items.length}</small>
                </header>
                <div className="agenda-section-items">
                  {group.items.map((item) => (
                    <DayAgendaItem
                      item={item}
                      onSave={onSave}
                      onOpenDetails={onOpenDetails}
                      onComplete={onComplete}
                      onDelete={onDelete}
                      key={item.id}
                    />
                  ))}
                </div>
              </section>
            ))}
          </div>
        ) : (
          <div className="day-agenda-panel-empty" aria-hidden="true">
            <span />
          </div>
        )}
      </div>
    </section>
  );
}

function MonthPeek({ month, direction }) {
  const isPrevious = direction === "previous";
  const days = buildPeekDays(month, isPrevious ? "start" : "end");

  return (
    <div className={`month-peek is-${direction}`}>
      <span>{shortMonthLabel(month)}</span>
      <div className="peek-days" aria-hidden="true">
        {days.map((day, index) => (
          <i key={`${day}-${index}`}>{day}</i>
        ))}
      </div>
    </div>
  );
}

function App() {
  const [currentMonth, setCurrentMonth] = useState(() => monthFromKey(TODAY_KEY));
  const [events, setEvents] = useState([]);
  const [calendarLoading, setCalendarLoading] = useState(false);
  const [expandedDayKey, setExpandedDayKey] = useState(null);
  const [dayViewKey, setDayViewKey] = useState(null);
  const [selectedEventId, setSelectedEventId] = useState(null);
  const [editorOpen, setEditorOpen] = useState(false);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [detailPaletteOpen, setDetailPaletteOpen] = useState(false);
  const [form, setForm] = useState(EMPTY_FORM);
  const [detailForm, setDetailForm] = useState(EMPTY_FORM);
  const [customDates, setCustomDates] = useState([]);
  const [formError, setFormError] = useState("");
  const [detailError, setDetailError] = useState("");
  const [notice, setNotice] = useState("");
  const [saving, setSaving] = useState(false);
  const [detailSaving, setDetailSaving] = useState(false);
  const [completionSaving, setCompletionSaving] = useState(null);
  const [detailDeleting, setDetailDeleting] = useState(false);
  const [deleteConfirming, setDeleteConfirming] = useState(false);
  const [hoveredStartDate, setHoveredStartDate] = useState(null);
  const [desktopPicker, setDesktopPicker] = useState(null);
  const dragSelection = useRef({ active: false, select: true, lastKey: null });
  const desktopPickerRef = useRef(null);
  const desktopPickerSessionRef = useRef(null);

  const monthDays = useMemo(() => buildMonthDays(currentMonth), [currentMonth]);
  const range = useMemo(() => calendarRange(currentMonth), [currentMonth]);
  const selectedEvent = events.find((item) => item.id === selectedEventId);
  const focusedTodoId = selectedEvent?.todoId ?? null;
  const desktopDatePicking = IS_DESKTOP && Boolean(desktopPicker);
  const pickingRepeat = desktopDatePicking
    ? REPEAT_LABELS[desktopPicker.repeatMode] || "仅一次"
    : form.repeat;
  const pickingDate = desktopDatePicking ? desktopPicker.date : form.date;
  const pickingCustomDates = desktopDatePicking ? desktopPicker.customDates : customDates;
  const customDateSet = useMemo(() => new Set(pickingCustomDates), [pickingCustomDates]);
  const customPicking = desktopDatePicking
    ? pickingRepeat === "自定义"
    : editorOpen && form.repeat === "自定义";
  const startDatePicking = desktopDatePicking
    ? pickingRepeat !== "自定义"
    : editorOpen && START_DATE_REPEATS.has(form.repeat);
  const calendarPicking = customPicking || startDatePicking;
  const calendarLocked = editorOpen && !calendarPicking;
  const outlinedCellIndexes = monthDays.reduce((indexes, day, index) => {
    if (isPastDateKey(day.key)) return indexes;
    if ((customPicking && customDateSet.has(day.key)) || isHoveredStartPattern(day)) indexes.push(index);
    return indexes;
  }, []);
  const selectionOutlinePath = buildGridOutlinePath(outlinedCellIndexes);
  const dayViewEvents = dayViewKey
    ? events
        .filter((item) => item.date === dayViewKey)
        .sort((left, right) => left.time.localeCompare(right.time))
    : [];

  async function refreshCalendar() {
    const result = await getCalendar(range.from, range.to);
    const nextEvents = result.data.map(calendarEventFromOccurrence);
    setEvents(nextEvents);
    return nextEvents;
  }

  useEffect(() => {
    let cancelled = false;
    setCalendarLoading(true);

    getCalendar(range.from, range.to)
      .then((result) => {
        if (!cancelled) setEvents(result.data.map(calendarEventFromOccurrence));
      })
      .catch((error) => {
        if (!cancelled) setNotice(error.message);
      })
      .finally(() => {
        if (!cancelled) setCalendarLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [range.from, range.to]);

  useEffect(() => {
    if (!window.noteDesktop?.onDataChanged) return undefined;

    return window.noteDesktop.onDataChanged(() => {
      refreshCalendar().catch((error) => setNotice(error.message));
    });
  }, [range.from, range.to]);

  useEffect(() => {
    if (!window.noteDesktop?.getDatePickerState || !window.noteDesktop?.onDatePickerStateChanged) {
      return undefined;
    }

    let disposed = false;
    function applyPickerState(state) {
      if (disposed) return;
      desktopPickerRef.current = state;
      setDesktopPicker(state);
      setHoveredStartDate(null);

      if (state && desktopPickerSessionRef.current !== state.sessionId) {
        desktopPickerSessionRef.current = state.sessionId;
        setCurrentMonth(monthFromKey(state.date));
        setExpandedDayKey(null);
        setSelectedEventId(null);
        setDayViewKey(null);
      }
      if (!state) desktopPickerSessionRef.current = null;
    }

    const removeListener = window.noteDesktop.onDatePickerStateChanged(applyPickerState);
    window.noteDesktop.getDatePickerState()
      .then(applyPickerState)
      .catch((error) => setNotice(error.message));

    return () => {
      disposed = true;
      removeListener?.();
    };
  }, []);

  useEffect(() => {
    function endDragSelection() {
      dragSelection.current.active = false;
      dragSelection.current.lastKey = null;
    }

    window.addEventListener("pointerup", endDragSelection);
    window.addEventListener("pointercancel", endDragSelection);
    return () => {
      window.removeEventListener("pointerup", endDragSelection);
      window.removeEventListener("pointercancel", endDragSelection);
    };
  }, []);

  useEffect(() => {
    if (!editorOpen && !selectedEventId && !dayViewKey && !desktopDatePicking) return undefined;

    function closeOnEscape(event) {
      if (event.key !== "Escape") return;
      if (desktopPickerRef.current) {
        finishDesktopDatePicking();
        return;
      }
      setEditorOpen(false);
      setSelectedEventId(null);
      setExpandedDayKey(null);
      setDayViewKey(null);
    }

    window.addEventListener("keydown", closeOnEscape);
    return () => window.removeEventListener("keydown", closeOnEscape);
  }, [dayViewKey, desktopDatePicking, editorOpen, selectedEventId]);

  useEffect(() => {
    if (!notice) return undefined;
    const timer = window.setTimeout(() => setNotice(""), 2400);
    return () => window.clearTimeout(timer);
  }, [notice]);

  useEffect(() => {
    if (!deleteConfirming) return undefined;
    const timer = window.setTimeout(() => setDeleteConfirming(false), 3500);
    return () => window.clearTimeout(timer);
  }, [deleteConfirming]);

  function moveMonth(amount) {
    setCurrentMonth((month) => addMonths(month, amount));
    setExpandedDayKey(null);
    setSelectedEventId(null);
    setDayViewKey(null);
  }

  function openEditor() {
    if (IS_DESKTOP) {
      window.noteDesktop.openCreate({ date: TODAY_KEY }).catch((error) => setNotice(error.message));
      return;
    }

    setSelectedEventId(null);
    setExpandedDayKey(null);
    setDayViewKey(null);
    setForm({ ...EMPTY_FORM, date: TODAY_KEY });
    setCustomDates([]);
    setHoveredStartDate(null);
    setFormError("");
    setPaletteOpen(false);
    setEditorOpen(true);
  }

  function closeEditor() {
    setEditorOpen(false);
    setPaletteOpen(false);
    setCustomDates([]);
    setHoveredStartDate(null);
  }

  function updateField(event) {
    const { name, value } = event.target;
    setForm((current) => ({ ...current, [name]: value }));
    if (name === "repeat") {
      if (value !== "自定义") setCustomDates([]);
      setHoveredStartDate(null);
    }
    setFormError("");
  }

  function selectColor(color) {
    setForm((current) => ({ ...current, color }));
  }

  function updateCustomDate(dateKey, shouldSelect) {
    const activeDesktopPicker = desktopPickerRef.current;
    if (activeDesktopPicker) {
      const nextDates = new Set(activeDesktopPicker.customDates || []);
      if (shouldSelect) nextDates.add(dateKey);
      else nextDates.delete(dateKey);
      const customDates = [...nextDates].sort();
      publishDesktopDateSelection({
        ...activeDesktopPicker,
        date: customDates[0] || activeDesktopPicker.date,
        customDates,
      });
      return;
    }

    setCustomDates((current) => {
      const next = new Set(current);
      if (shouldSelect) next.add(dateKey);
      else next.delete(dateKey);
      return [...next].sort();
    });
  }

  function beginDateSelection(dateKey, event) {
    if (!calendarPicking || isPastDateKey(dateKey) || event.button !== 0) return;
    event.preventDefault();

    if (startDatePicking) {
      const activeDesktopPicker = desktopPickerRef.current;
      if (activeDesktopPicker) {
        publishDesktopDateSelection({ ...activeDesktopPicker, date: dateKey, customDates: [] });
      } else {
        setForm((current) => ({ ...current, date: dateKey }));
      }
      return;
    }

    const shouldSelect = !customDateSet.has(dateKey);
    dragSelection.current = { active: true, select: shouldSelect, lastKey: dateKey };
    updateCustomDate(dateKey, shouldSelect);
  }

  function continueDateSelection(dateKey) {
    const drag = dragSelection.current;
    if (!customPicking || isPastDateKey(dateKey) || !drag.active || drag.lastKey === dateKey) return;
    drag.lastKey = dateKey;
    updateCustomDate(dateKey, drag.select);
  }

  function toggleCustomDateFromKeyboard(dateKey, event) {
    if (!calendarPicking || isPastDateKey(dateKey) || !["Enter", " "].includes(event.key)) return;
    event.preventDefault();
    if (startDatePicking) {
      const activeDesktopPicker = desktopPickerRef.current;
      if (activeDesktopPicker) {
        publishDesktopDateSelection({ ...activeDesktopPicker, date: dateKey, customDates: [] });
      } else {
        setForm((current) => ({ ...current, date: dateKey }));
      }
    } else {
      updateCustomDate(dateKey, !customDateSet.has(dateKey));
    }
  }

  function matchesStartPattern(day, anchorDate) {
    if (!anchorDate || isPastDateKey(anchorDate) || isPastDateKey(day.key)) return false;
    const active = dateFromKey(anchorDate);

    if (pickingRepeat === "每周") return dateFromKey(day.key).getUTCDay() === active.getUTCDay();
    if (pickingRepeat === "每月") return day.day === active.getUTCDate();
    return day.key === anchorDate;
  }

  function isHoveredStartPattern(day) {
    return startDatePicking && Boolean(hoveredStartDate) && matchesStartPattern(day, hoveredStartDate);
  }

  function isSelectedStartPattern(day) {
    return startDatePicking && Boolean(pickingDate) && matchesStartPattern(day, pickingDate);
  }

  function publishDesktopDateSelection(nextState) {
    desktopPickerRef.current = nextState;
    setDesktopPicker(nextState);
    window.noteDesktop?.selectDatePicker?.({
      date: nextState.date,
      customDates: nextState.customDates,
    }).catch((error) => setNotice(error.message));
  }

  function finishDesktopDatePicking() {
    if (!desktopPickerRef.current) return;
    window.noteDesktop?.finishDatePicker?.().catch((error) => setNotice(error.message));
  }

  function closeEventDetails() {
    setSelectedEventId(null);
    setDetailPaletteOpen(false);
    setDeleteConfirming(false);
    setDetailError("");
  }

  function openEventDetails(item) {
    if (IS_DESKTOP) {
      window.noteDesktop
        .openDetail({ todoId: item.todoId, date: item.date })
        .catch((error) => setNotice(error.message));
      return;
    }

    setDeleteConfirming(false);
    if (selectedEvent?.todoId === item.todoId) {
      closeEventDetails();
      return;
    }

    setEditorOpen(false);
    setDayViewKey(item.date);
    setExpandedDayKey(null);
    setDetailPaletteOpen(false);
    setDetailError("");
    setDetailForm({
      content: item.content,
      date: item.startDate,
      time: item.startTime,
      reminder: item.reminder,
      repeat: item.repeat,
      color: item.color,
    });
    setSelectedEventId(item.id);
  }

  function openDayAgenda(dateKey) {
    if (IS_DESKTOP) {
      window.noteDesktop.openDay({ date: dateKey }).catch((error) => setNotice(error.message));
      return;
    }

    setEditorOpen(false);
    setSelectedEventId(null);
    setExpandedDayKey(null);
    setDayViewKey(dateKey);
  }

  async function saveAgendaItem(item, changes) {
    const latest = events.find((eventItem) => eventItem.todoId === item.todoId) || item;
    const payload = { version: latest.version };

    if (changes.content !== undefined) payload.content = changes.content;
    if (changes.time !== undefined) payload.starts_at = dateTimeAt(latest.startDate, changes.time);

    await patchTodo(item.todoId, payload);
    await refreshCalendar();
    setNotice("已保存");
  }

  async function completeAgendaItem(item) {
    if (isEventDone(item)) return;

    const [result] = await Promise.all([
      patchOccurrence(item.todoId, item.date, true),
      new Promise((resolve) => window.setTimeout(resolve, 280)),
    ]);

    setEvents((current) => current.map((eventItem) => {
      const isTarget = eventItem.todoId === result.todo_id && eventItem.date === result.occurs_on;
      return isTarget
        ? { ...eventItem, occurrenceDone: Boolean(result.occurrence_done) }
        : eventItem;
    }));
    setNotice("已完成");
  }

  async function deleteAgendaItem(item) {
    await deleteTodo(item.todoId);
    setEvents((current) => current.filter((eventItem) => eventItem.todoId !== item.todoId));
    setNotice("已删除");
  }

  function updateDetailField(event) {
    const { name, type, checked, value } = event.target;
    setDetailForm((current) => ({
      ...current,
      [name]: type === "checkbox" ? checked : value,
    }));
  }

  async function updateOccurrenceDone(nextDone) {
    if (!selectedEvent || selectedEvent.allDone) return;

    const target = selectedEvent;
    setCompletionSaving("occurrence");
    setDetailError("");

    try {
      const result = await patchOccurrence(target.todoId, target.date, nextDone);

      setEvents((current) =>
        current.map((item) => {
          const isTarget = item.todoId === result.todo_id && item.date === result.occurs_on;
          if (!isTarget) return item;

          return {
            ...item,
            occurrenceDone: Boolean(result.occurrence_done),
          };
        }),
      );

      setNotice(result.occurrence_done ? "已完成" : "已恢复为待完成");
    } catch (error) {
      setDetailError(error.message);
    } finally {
      setCompletionSaving(null);
    }
  }

  async function updateAllDone(nextAllDone) {
    if (!selectedEvent) return;

    const target = selectedEvent;
    setCompletionSaving("all");
    setDetailError("");

    try {
      const updatedTodo = await patchTodo(target.todoId, {
        all_done: nextAllDone,
        version: target.version,
      });

      setEvents((current) =>
        current.map((item) => {
          if (item.todoId !== target.todoId) return item;

          return {
            ...item,
            allDone: Boolean(updatedTodo.all_done),
            version: updatedTodo.version,
          };
        }),
      );

      setNotice(updatedTodo.all_done ? "已全部完成" : "已恢复单日完成状态");
    } catch (error) {
      setDetailError(error.message);
    } finally {
      setCompletionSaving(null);
    }
  }

  async function saveEventDetails(event) {
    event.preventDefault();
    const content = detailForm.content.trim();
    if (!content || !selectedEvent) {
      setDetailError("请输入内容");
      return;
    }

    const payload = {
      content,
      color: detailForm.color,
      notify_mode: REMINDER_VALUES[detailForm.reminder],
      version: selectedEvent.version,
      starts_at: dateTimeAt(detailForm.date, detailForm.time),
    };

    if (selectedEvent.repeat !== "自定义") {
      payload.repeat_mode = REPEAT_VALUES[detailForm.repeat];
    }

    setDetailSaving(true);
    setDetailError("");

    try {
      await patchTodo(selectedEvent.todoId, payload);
      await refreshCalendar();
      closeEventDetails();
      setNotice("已保存");
    } catch (error) {
      setDetailError(error.message);
    } finally {
      setDetailSaving(false);
    }
  }

  async function deleteSelectedTodo() {
    if (!selectedEvent) return;

    if (!deleteConfirming) {
      setDeleteConfirming(true);
      return;
    }

    const todoID = selectedEvent.todoId;
    setDetailDeleting(true);
    setDetailError("");

    try {
      await deleteTodo(todoID);
      setEvents((current) => current.filter((item) => item.todoId !== todoID));
      closeEventDetails();
      setNotice("已删除");
    } catch (error) {
      setDeleteConfirming(false);
      setDetailError(error.message);
    } finally {
      setDetailDeleting(false);
    }
  }

  async function createEvent(event) {
    event.preventDefault();
    const content = form.content.trim();
    const isCustom = form.repeat === "自定义";
    const needsStartDate = START_DATE_REPEATS.has(form.repeat);

    if (!content) {
      setFormError("请输入内容");
      return;
    }
    if (isCustom && customDates.length === 0) {
      setFormError("请在日历中选择日期");
      return;
    }
    if (!form.time) {
      setFormError("请选择时间");
      return;
    }
    if (needsStartDate && !form.date) {
      setFormError("请选择开始日期");
      return;
    }

    const startDate = isCustom ? customDates[0] : needsStartDate ? form.date : TODAY_KEY;
    const startTime = form.time;
    const pickedRandomColor = !form.color;
    const payload = {
      content,
      starts_at: dateTimeAt(startDate, startTime),
      repeat_mode: REPEAT_VALUES[form.repeat],
      notify_mode: REMINDER_VALUES[form.reminder],
      ...(isCustom ? { custom_dates: customDates } : {}),
      ...(form.color ? { color: form.color } : {}),
    };

    setSaving(true);
    setFormError("");

    try {
      await createTodo(payload);
      await refreshCalendar();

      closeEditor();
      setNotice(pickedRandomColor ? "已创建 · 随机配色" : "已创建");
    } catch (error) {
      setFormError(error.message);
    } finally {
      setSaving(false);
    }
  }

  const selectedColor = EVENT_COLORS.find((item) => item.value === form.color);
  const selectedDetailColor = EVENT_COLORS.find((item) => item.value === detailForm.color);
  const formUsesCustomColor = Boolean(form.color) && !selectedColor;
  const detailUsesCustomColor = Boolean(detailForm.color) && !selectedDetailColor;
  const previousMonth = addMonths(currentMonth, -1);
  const nextMonth = addMonths(currentMonth, 1);
  const selectionColor = desktopPicker?.color || form.color || "#F3B51B";

  return (
    <main
      className={[
        "calendar-shell",
        focusedTodoId ? "has-event-focus has-detail-panel" : "",
        editorOpen ? "has-editor-panel" : "",
        calendarPicking ? "is-calendar-picking" : "",
        desktopDatePicking ? "is-desktop-date-picking" : "",
        calendarLocked ? "is-calendar-locked" : "",
        IS_DESKTOP ? "is-desktop-calendar" : "",
      ].filter(Boolean).join(" ")}
      style={{
        "--selection-color": selectionColor,
        "--selection-soft": colorWithAlpha(selectionColor, 0.12),
        "--selection-ink": readableSelectionInk(selectionColor),
        ...(selectedEvent
          ? {
              "--focus-color": selectedEvent.color,
              "--focus-soft": colorWithAlpha(selectedEvent.color, 0.13),
            }
          : {}),
      }}
    >
      <div className="workspace-stack">
        <DayAgendaPanel
          dateKey={dayViewKey}
          items={dayViewEvents}
          onClear={() => setDayViewKey(null)}
          onSave={saveAgendaItem}
          onOpenDetails={openEventDetails}
          onComplete={completeAgendaItem}
          onDelete={deleteAgendaItem}
        />

        <div className="month-stack">
          <MonthPeek month={previousMonth} direction="previous" />

          <section
            className={`calendar-panel ${calendarLoading ? "is-loading" : ""}`}
            aria-label={`${monthLabel(currentMonth)}日历`}
            aria-busy={calendarLoading}
          >
            <header className="calendar-header">
              <span className="title-accent" aria-hidden="true" />
              <h1>{monthLabel(currentMonth)}</h1>
              <div className="month-controls">
                <button type="button" onClick={() => moveMonth(-1)} aria-label="切换到上个月">
                  <CaretUp size={19} weight="bold" />
                </button>
                <button type="button" onClick={() => moveMonth(1)} aria-label="切换到下个月">
                  <CaretDown size={19} weight="bold" />
                </button>
              </div>
              {desktopPicker && (
                <div className="desktop-picker-status" role="status">
                  <span className="desktop-picker-dot" aria-hidden="true" />
                  <div>
                    <strong>{pickingRepeat === "自定义" ? "选择自定义日期" : `选择${pickingRepeat}的开始日期`}</strong>
                    <small>{pickingRepeat === "自定义" ? `已选 ${pickingCustomDates.length} 天` : pickingDate}</small>
                  </div>
                  <button type="button" onClick={finishDesktopDatePicking}>
                    <Check size={15} weight="bold" />
                    返回填写
                  </button>
                </div>
              )}
              <WindowControls />
            </header>

            <div className="weekday-row" role="row">
            {WEEKDAYS.map((weekday) => (
              <div role="columnheader" key={weekday}>
                {weekday}
              </div>
            ))}
          </div>

            <div
              className="month-grid"
              role="grid"
              onPointerLeave={() => setHoveredStartDate(null)}
            >
            {selectionOutlinePath && (
              <svg className="selection-outline" viewBox="0 0 7 6" preserveAspectRatio="none" aria-hidden="true">
                <path d={selectionOutlinePath} vectorEffect="non-scaling-stroke" />
              </svg>
            )}
            {monthDays.map((day) => {
              const dayEvents = events
                .filter((item) => item.date === day.key)
                .sort((left, right) => left.time.localeCompare(right.time));
              const isExpanded = expandedDayKey === day.key;
              const visibleEvents = dayEvents.length > 9 ? dayEvents.slice(0, 8) : dayEvents.slice(0, 9);
              const hasFocusedTodo = focusedTodoId && day.isCurrentMonth
                ? dayEvents.some((item) => item.todoId === focusedTodoId)
                : false;
              const selectedCustomDate = customDateSet.has(day.key);
              const pastDateDisabled = calendarPicking && isPastDateKey(day.key);
              const hoveredStartPattern = isHoveredStartPattern(day);
              const selectedStartPattern = isSelectedStartPattern(day);
              const selectedStartDate = !["每周", "每月"].includes(pickingRepeat) && selectedStartPattern;
              const selectedRepeatPattern = ["每周", "每月"].includes(pickingRepeat) && selectedStartPattern;

              return (
                <div
                  className={[
                    "day-cell",
                    day.isCurrentMonth ? "" : "is-adjacent",
                    day.key === TODAY_KEY ? "is-today" : "",
                    dayViewKey === day.key ? "is-agenda-selected" : "",
                    hasFocusedTodo ? "has-focused-todo" : "",
                    focusedTodoId && day.isCurrentMonth && !hasFocusedTodo ? "is-focus-muted" : "",
                    selectedCustomDate ? "is-custom-selected" : "",
                    hoveredStartPattern ? "is-start-highlighted" : "",
                    selectedStartDate ? "is-start-selected" : "",
                    selectedRepeatPattern ? "is-pattern-selected" : "",
                    calendarPicking && !pastDateDisabled ? "is-pickable" : "",
                    pastDateDisabled ? "is-past-disabled" : "",
                  ]
                    .filter(Boolean)
                    .join(" ")}
                  role="gridcell"
                  aria-selected={dayViewKey === day.key}
                  key={day.key}
                  onPointerDown={(event) => beginDateSelection(day.key, event)}
                  onPointerEnter={() => {
                    if (pastDateDisabled) {
                      setHoveredStartDate(null);
                      return;
                    }
                    if (startDatePicking) setHoveredStartDate(day.key);
                    continueDateSelection(day.key);
                  }}
                  onPointerMove={() => continueDateSelection(day.key)}
                >
                  <button
                    className="date-button"
                    type="button"
                    disabled={calendarLocked || pastDateDisabled}
                    aria-label={
                      pastDateDisabled
                        ? `${day.key}已过期，不可选择`
                        : customPicking
                          ? `${selectedCustomDate ? "取消" : "选择"}${day.key}`
                          : startDatePicking
                            ? `选择开始日期${day.key}`
                            : `查看${day.key}日程`
                    }
                    aria-pressed={calendarPicking ? (customPicking ? selectedCustomDate : selectedStartPattern) : undefined}
                    onKeyDown={(event) => toggleCustomDateFromKeyboard(day.key, event)}
                    onClick={() => {
                      if (!calendarPicking) openDayAgenda(day.key);
                    }}
                  >
                    <span className="date-mark">
                      <strong>{day.day}</strong>
                    </span>
                    <small className="date-note">{calendarNote(day.key)}</small>
                  </button>

                  <div
                    className="day-events"
                    aria-label={`${day.key}日程`}
                    aria-hidden={editorOpen || calendarPicking ? "true" : undefined}
                    inert={editorOpen || calendarPicking ? true : undefined}
                  >
                    {visibleEvents.map((item) => (
                      <EventDot
                        item={item}
                        selected={selectedEventId === item.id}
                        onSelect={openEventDetails}
                        key={item.id}
                      />
                    ))}

                    {dayEvents.length > 9 && (
                      <button
                        className="event-more"
                        type="button"
                        aria-label={`展开${dayEvents.length}条日程`}
                        aria-expanded={isExpanded}
                        onClick={(event) => {
                          event.stopPropagation();
                          setExpandedDayKey(isExpanded ? null : day.key);
                        }}
                      >
                        <DotsThree size={17} weight="bold" />
                      </button>
                    )}

                    {isExpanded && (
                      <div className="event-overflow" onPointerDown={(event) => event.stopPropagation()}>
                        {dayEvents.map((item) => (
                          <EventDot
                            item={item}
                            selected={selectedEventId === item.id}
                            onSelect={openEventDetails}
                            key={`expanded-${item.id}`}
                          />
                        ))}
                      </div>
                    )}
                  </div>
                </div>
              );
            })}
          </div>

          </section>

          <MonthPeek month={nextMonth} direction="next" />
        </div>
      </div>

      {!IS_DESKTOP && (
        <button className="add-fab" type="button" onClick={openEditor} aria-label="添加日程">
          <Plus size={28} aria-hidden="true" />
        </button>
      )}

      {notice && <div className="toast" role="status">{notice}</div>}

      {selectedEvent && (
        <div className="detail-layer">
          <aside
            className="detail-panel"
            role="dialog"
            aria-modal="false"
            aria-labelledby="detail-title"
            style={{
              "--detail-color": detailForm.color,
              "--detail-soft": colorWithAlpha(detailForm.color, 0.14),
            }}
          >
            <header className="side-header">
              <h2 id="detail-title">详情</h2>
              <button type="button" onClick={closeEventDetails} aria-label="关闭详情">
                <X size={21} />
              </button>
            </header>

            <form className="side-form" onSubmit={saveEventDetails}>
              <div className="completion-controls" aria-label="完成状态">
                <label
                  className={`completion-toggle ${selectedEvent.allDone ? "is-overridden" : ""} ${completionSaving === "occurrence" ? "is-saving" : ""}`}
                >
                  <input
                    type="checkbox"
                    checked={selectedEvent.occurrenceDone}
                    disabled={Boolean(completionSaving) || selectedEvent.allDone}
                    aria-label={`${selectedEvent.date} ${selectedEvent.content}的本次完成状态`}
                    onChange={(event) => void updateOccurrenceDone(event.target.checked)}
                  />
                  <span>
                    {completionSaving === "occurrence"
                      ? "更新中"
                      : selectedEvent.allDone
                        ? "本次状态已保留"
                        : selectedEvent.occurrenceDone
                          ? "本次已完成"
                          : "本次未完成"}
                  </span>
                </label>

                <label
                  className={`completion-toggle completion-toggle-all ${selectedEvent.allDone ? "is-active" : ""} ${completionSaving === "all" ? "is-saving" : ""}`}
                >
                  <input
                    type="checkbox"
                    checked={selectedEvent.allDone}
                    disabled={Boolean(completionSaving)}
                    aria-label={`${selectedEvent.content}的全部完成状态`}
                    onChange={(event) => void updateAllDone(event.target.checked)}
                  />
                  <span>
                    {completionSaving === "all"
                      ? "更新中"
                      : selectedEvent.allDone
                        ? "已全部完成"
                        : "全部完成"}
                  </span>
                </label>
              </div>

              <label>
                <span>内容</span>
                <textarea name="content" value={detailForm.content} onChange={updateDetailField} rows={5} maxLength={500} autoFocus />
              </label>

              {selectedEvent.repeat !== "自定义" ? (
                <>
                  <div className="form-row">
                    <label>
                      <span>日期</span>
                      <input name="date" type="date" value={detailForm.date} onChange={updateDetailField} required />
                    </label>
                    <TimeField
                      value={detailForm.time}
                      onChange={(time) => setDetailForm((current) => ({ ...current, time }))}
                    />
                  </div>

                  <SelectField
                    label="重复"
                    value={detailForm.repeat}
                    options={Object.keys(REPEAT_VALUES).filter((label) => label !== "自定义")}
                    onChange={(value) => updateDetailField({ target: { name: "repeat", value } })}
                  />
                </>
              ) : (
                <div className="form-row custom-detail-time">
                  <div className="read-only-value">自定义日期</div>
                  <TimeField
                    value={detailForm.time}
                    onChange={(time) => setDetailForm((current) => ({ ...current, time }))}
                  />
                </div>
              )}

              <SelectField
                label="提醒"
                value={detailForm.reminder}
                options={Object.keys(REMINDER_VALUES)}
                onChange={(value) => updateDetailField({ target: { name: "reminder", value } })}
              />

              <div className="color-field">
                <span className="field-label">颜色</span>
                <button className="color-trigger" type="button" aria-expanded={detailPaletteOpen} onClick={() => setDetailPaletteOpen((current) => !current)}>
                  <span className="color-trigger-swatch" style={{ background: detailForm.color }} />
                  <span>{selectedDetailColor?.label || "自定义"}</span>
                  <Palette size={17} />
                </button>
                {detailPaletteOpen && (
                  <div className="color-palette">
                    <div className="palette-grid">
                      {EVENT_COLORS.map((color) => (
                        <button className={`palette-swatch ${detailForm.color === color.value ? "is-selected" : ""}`} type="button" style={{ "--swatch-color": color.value }} onClick={() => setDetailForm((current) => ({ ...current, color: color.value }))} aria-label={color.label} key={color.value}>
                          {detailForm.color === color.value && <Check size={12} weight="bold" />}
                        </button>
                      ))}
                      <CustomColorSwatch
                        value={detailForm.color}
                        selected={detailUsesCustomColor}
                        onChange={(color) => setDetailForm((current) => ({ ...current, color }))}
                      />
                    </div>
                  </div>
                )}
              </div>

              {detailError && <p className="form-error" role="alert">{detailError}</p>}
              <button
                className={`delete-button ${deleteConfirming ? "is-confirming" : ""}`}
                type="button"
                onClick={deleteSelectedTodo}
                disabled={detailSaving || completionSaving || detailDeleting}
              >
                <Trash size={16} aria-hidden="true" />
                <span aria-live="polite">
                  {detailDeleting ? "删除中" : deleteConfirming ? "再次点击确认删除" : "删除日程"}
                </span>
              </button>
              <footer className="side-footer">
                <button className="cancel-button" type="button" onClick={closeEventDetails}>关闭</button>
                <button className="save-button" type="submit" disabled={detailSaving || completionSaving || detailDeleting}>{detailSaving ? "保存中" : "保存"}</button>
              </footer>
            </form>
          </aside>
        </div>
      )}

      {editorOpen && (
        <div className={`editor-layer ${customPicking ? "is-picking" : ""}`} onMouseDown={(event) => { if (!customPicking && event.target === event.currentTarget) closeEditor(); }}>
          <aside className={`editor-panel ${customPicking ? "is-picking" : ""}`} role="dialog" aria-modal={!customPicking}>
            <header className="side-header">
              <h2>新日程</h2>
              <button type="button" onClick={closeEditor} aria-label="关闭创建面板"><X size={21} /></button>
            </header>

            <form className="side-form" onSubmit={createEvent}>
              <label>
                <span>内容</span>
                <textarea name="content" value={form.content} onChange={updateField} rows={5} maxLength={500} autoFocus />
              </label>

              <SelectField
                label="重复"
                value={form.repeat}
                options={Object.keys(REPEAT_VALUES)}
                onChange={(value) => updateField({ target: { name: "repeat", value } })}
              />

              {startDatePicking && (
                <div className="form-row start-date-row">
                  <div className="start-date-value">
                    <span>开始日期</span>
                    <output>{form.date}</output>
                  </div>
                  <TimeField
                    value={form.time}
                    onChange={(time) => setForm((current) => ({ ...current, time }))}
                  />
                </div>
              )}

              {!startDatePicking && (
                <TimeField
                  value={form.time}
                  onChange={(time) => setForm((current) => ({ ...current, time }))}
                />
              )}

              {customPicking && (
                <div className="custom-pick-status" role="status">
                  <span>{customDates.length} 天</span>
                  <small>在日历中单击或拖动</small>
                </div>
              )}

              <SelectField
                label="提醒"
                value={form.reminder}
                options={Object.keys(REMINDER_VALUES)}
                onChange={(value) => updateField({ target: { name: "reminder", value } })}
              />

              <div className="color-field">
                <span className="field-label">颜色</span>
                <button className="color-trigger" type="button" aria-expanded={paletteOpen} onClick={() => setPaletteOpen((current) => !current)}>
                  <span className={`color-trigger-swatch ${form.color ? "" : "is-random"}`} style={form.color ? { background: form.color } : undefined} />
                  <span>{selectedColor?.label || (form.color ? "自定义" : "随机")}</span>
                  <Palette size={17} />
                </button>
                {paletteOpen && (
                  <div className="color-palette">
                    <div className="palette-grid">
                      <button className={`palette-swatch random-swatch ${form.color ? "" : "is-selected"}`} type="button" onClick={() => selectColor("")} aria-label="随机颜色"><Sparkle size={14} weight="fill" /></button>
                      {EVENT_COLORS.map((color) => (
                        <button className={`palette-swatch ${form.color === color.value ? "is-selected" : ""}`} type="button" style={{ "--swatch-color": color.value }} onClick={() => selectColor(color.value)} aria-label={color.label} key={color.value}>
                          {form.color === color.value && <Check size={12} weight="bold" />}
                        </button>
                      ))}
                      <CustomColorSwatch value={form.color} selected={formUsesCustomColor} onChange={selectColor} />
                    </div>
                  </div>
                )}
              </div>

              {formError && <p className="form-error" role="alert">{formError}</p>}
              <footer className="side-footer">
                <button className="cancel-button" type="button" onClick={closeEditor}>取消</button>
                <button className="save-button" type="submit" disabled={saving}>{saving ? "创建中" : "创建"}</button>
              </footer>
            </form>
          </aside>
        </div>
      )}
    </main>
  );
}

export {
  CustomColorSwatch,
  DayAgendaPanel,
  EVENT_COLORS,
  REMINDER_LABELS,
  REMINDER_VALUES,
  REPEAT_LABELS,
  REPEAT_VALUES,
  SelectField,
  SHANGHAI_TIME_ZONE,
  TODAY_KEY,
  TimeField,
  WindowControls,
  WEEKDAYS,
  addDays,
  addMonths,
  buildGridOutlinePath,
  buildMonthDays,
  calendarEventFromOccurrence,
  calendarNote,
  colorWithAlpha,
  dateFromKey,
  dateKeyFromUTC,
  dateTimeAt,
  monthFromKey,
  monthLabel,
  preciseDateLabel,
  readableSelectionInk,
  shanghaiDateTimeParts,
};

export default App;
