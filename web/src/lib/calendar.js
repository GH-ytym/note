export const WEEKDAYS = ["一", "二", "三", "四", "五", "六", "日"];
export const SHANGHAI_TIME_ZONE = "Asia/Shanghai";

export const REPEAT_VALUES = {
  仅一次: "once",
  每天: "daily",
  工作日: "weekdays",
  周末: "weekends",
  每周: "weekly",
  每月: "monthly",
  自定义: "custom",
};

export const REPEAT_LABELS = Object.fromEntries(
  Object.entries(REPEAT_VALUES).map(([label, value]) => [value, label]),
);

export const REMINDER_VALUES = {
  静默提醒: "silent",
  弹窗提醒: "popup",
};

export const REMINDER_LABELS = {
  silent: "静默提醒",
  popup: "弹窗提醒",
};

export const EVENT_COLORS = [
  { value: "#F3B51B", label: "琥珀" },
  { value: "#F47C48", label: "珊瑚" },
  { value: "#E95B78", label: "莓红" },
  { value: "#A879F2", label: "鸢尾" },
  { value: "#5B8DEF", label: "钴蓝" },
  { value: "#35B7A0", label: "薄荷" },
  { value: "#82B94B", label: "青柠" },
];

export const START_DATE_REPEATS = new Set(["仅一次", "每周", "每月"]);

export const EMPTY_FORM = {
  title: "",
  content: "",
  reminder: "弹窗提醒",
  repeat: "仅一次",
  color: "",
  date: "",
  time: "09:00",
};

export const IS_DESKTOP = Boolean(window.noteDesktop?.isDesktop);

export function pad(value) {
  return String(value).padStart(2, "0");
}

export function dateKeyFromUTC(date) {
  return `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(date.getUTCDate())}`;
}

export function dateFromKey(key) {
  const [year, month, day] = key.split("-").map(Number);
  return new Date(Date.UTC(year, month - 1, day));
}

export function monthFromKey(key) {
  const date = dateFromKey(key);
  return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), 1));
}

export function addDays(date, amount) {
  return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate() + amount));
}

export function addMonths(date, amount) {
  return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth() + amount, 1));
}

export function getShanghaiTodayKey() {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: SHANGHAI_TIME_ZONE,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(new Date());
  const value = (type) => parts.find((part) => part.type === type)?.value;
  return `${value("year")}-${value("month")}-${value("day")}`;
}

export const TODAY_KEY = getShanghaiTodayKey();

export function isPastDateKey(dateKey) {
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

export function monthLabel(month) {
  return `${month.getUTCFullYear()}年${month.getUTCMonth() + 1}月`;
}

export function preciseDateLabel(dateKey) {
  const date = dateFromKey(dateKey);
  return `${date.getUTCFullYear()}年${date.getUTCMonth() + 1}月${date.getUTCDate()}日`;
}

export function calendarNote(dateKey) {
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

export function buildGridOutlinePath(indices) {
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

export function splitTime(value) {
  const [hour = "00", minute = "00"] = (value || "00:00").split(":");
  return { hour: Number(hour), minute: Number(minute) };
}

export function joinTime(hour, minute) {
  return `${pad((hour + 24) % 24)}:${pad((minute + 60) % 60)}`;
}

export function shortMonthLabel(month) {
  return `${month.getUTCFullYear()} / ${pad(month.getUTCMonth() + 1)}`;
}

export function calendarRange(month) {
  return {
    from: dateKeyFromUTC(addMonths(month, -1)),
    to: dateKeyFromUTC(addMonths(month, 2)),
  };
}

export function buildMonthDays(month) {
  const firstDay = new Date(Date.UTC(month.getUTCFullYear(), month.getUTCMonth(), 1));
  const mondayOffset = (firstDay.getUTCDay() + 6) % 7;
  const firstCell = addDays(firstDay, -mondayOffset);

  return Array.from({ length: 42 }, (_, index) => {
    const date = addDays(firstCell, index);
    return {
      key: dateKeyFromUTC(date),
      day: date.getUTCDate(),
      isCurrentMonth:
        date.getUTCFullYear() === month.getUTCFullYear()
        && date.getUTCMonth() === month.getUTCMonth(),
    };
  });
}

export function buildPeekDays(month, edge) {
  if (edge === "end") {
    return Array.from({ length: 7 }, (_, index) => addDays(month, index).getUTCDate());
  }

  const currentMonth = addMonths(month, 1);
  return Array.from({ length: 7 }, (_, index) => addDays(currentMonth, index - 7).getUTCDate());
}

export function dateTimeAt(dateKey, time = "00:00") {
  return new Date(`${dateKey}T${time}:00+08:00`).toISOString();
}

export function shanghaiDateTimeParts(value) {
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

export function calendarEventFromOccurrence(item) {
  const occurrence = shanghaiDateTimeParts(item.occurs_at);
  const start = shanghaiDateTimeParts(item.starts_at || item.occurs_at);

  return {
    id: `${item.todo_id}-${item.occurs_at}`,
    todoId: item.todo_id,
    title: item.title || item.content || "",
    content: item.content ?? item.title ?? "",
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

export function isEventDone(item) {
  return Boolean(item?.allDone || item?.occurrenceDone);
}

export function colorWithAlpha(color, alpha) {
  const hex = (color || "#F3B51B").replace("#", "");
  const value = Number.parseInt(hex, 16);
  return `rgb(${(value >> 16) & 255} ${(value >> 8) & 255} ${value & 255} / ${alpha})`;
}

export function readableSelectionInk(color) {
  const hex = (color || "#F3B51B").replace("#", "");
  const value = Number.parseInt(hex, 16);
  const red = (value >> 16) & 255;
  const green = (value >> 8) & 255;
  const blue = value & 255;
  const brightness = (red * 299 + green * 587 + blue * 114) / 255000;

  return brightness > 0.58 ? "#171100" : "#F7F9F5";
}

export function validDate(value) {
  return /^\d{4}-\d{2}-\d{2}$/.test(value || "") ? value : TODAY_KEY;
}

export function nextDateKey(value) {
  return dateKeyFromUTC(addDays(dateFromKey(value), 1));
}

export function todoDateKeys(todo) {
  return (todo.custom_dates || [])
    .map((item) => String(item.date || "").slice(0, 10))
    .filter((date) => /^\d{4}-\d{2}-\d{2}$/.test(date))
    .sort();
}
