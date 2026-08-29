const SHANGHAI_TIME_ZONE = "Asia/Shanghai";
const SHANGHAI_OFFSET = "+08:00";
const RETRY_DELAY_MS = 60_000;

function dateKeyAt(timestamp, timeZone = SHANGHAI_TIME_ZONE) {
  const date = new Date(timestamp);
  if (Number.isNaN(date.getTime())) return "";

  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(date);
  const value = (type) => parts.find((part) => part.type === type)?.value;
  return `${value("year")}-${value("month")}-${value("day")}`;
}

function nextDateKey(dateKey) {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(String(dateKey || ""));
  if (!match) throw new Error("invalid date key");

  const next = new Date(Date.UTC(
    Number(match[1]),
    Number(match[2]) - 1,
    Number(match[3]) + 1,
  ));
  return next.toISOString().slice(0, 10);
}

function occurrenceKey(item) {
  const todoID = Number(item?.todo_id);
  const occursAt = Date.parse(item?.occurs_at);
  if (!Number.isSafeInteger(todoID) || todoID < 1 || !Number.isFinite(occursAt)) return "";
  return `${todoID}:${new Date(occursAt).toISOString()}`;
}

function isReminderCandidate(item, now) {
  const occursAt = Date.parse(item?.occurs_at);
  return Number.isFinite(occursAt)
    && occursAt > now
    && ["silent", "popup"].includes(item?.notify_mode)
    && !item?.all_done
    && !item?.occurrence_done;
}

function delayUntilNextShanghaiDay(now, dateKey) {
  const nextMidnight = Date.parse(`${nextDateKey(dateKey)}T00:00:01${SHANGHAI_OFFSET}`);
  return Math.max(1_000, nextMidnight - now);
}

class ReminderScheduler {
  constructor({
    loadOccurrences,
    onReminder,
    onError = () => {},
    now = () => Date.now(),
    setTimer = setTimeout,
    clearTimer = clearTimeout,
  }) {
    if (typeof loadOccurrences !== "function") throw new TypeError("loadOccurrences is required");
    if (typeof onReminder !== "function") throw new TypeError("onReminder is required");

    this.loadOccurrences = loadOccurrences;
    this.onReminder = onReminder;
    this.onError = onError;
    this.now = now;
    this.setTimer = setTimer;
    this.clearTimer = clearTimer;
    this.occurrenceTimers = new Map();
    this.fired = new Set();
    this.refreshTimer = null;
    this.revision = 0;
    this.activeDate = "";
    this.stopped = true;
  }

  start() {
    this.stopped = false;
    return this.refresh();
  }

  async refresh() {
    if (this.stopped) return;

    const revision = ++this.revision;
    this.clearOccurrenceTimers();
    this.clearRefreshTimer();

    const startedAt = this.now();
    const from = dateKeyAt(startedAt);
    const to = nextDateKey(from);
    if (from !== this.activeDate) {
      this.activeDate = from;
      this.fired.clear();
    }

    let occurrences;
    try {
      occurrences = await this.loadOccurrences(from, to);
      if (!Array.isArray(occurrences)) throw new Error("calendar response data must be an array");
    } catch (error) {
      if (revision !== this.revision || this.stopped) return;
      this.reportError(error);
      this.scheduleRefresh(RETRY_DELAY_MS);
      return;
    }

    if (revision !== this.revision || this.stopped) return;

    const refreshedAt = this.now();
    for (const item of occurrences) {
      if (!isReminderCandidate(item, refreshedAt)) continue;

      const key = occurrenceKey(item);
      if (!key || this.fired.has(key)) continue;

      const delay = Date.parse(item.occurs_at) - refreshedAt;
      const timer = this.setTimer(() => {
        this.occurrenceTimers.delete(key);
        if (this.stopped || this.fired.has(key)) return;
        this.fired.add(key);

        try {
          const result = this.onReminder(item);
          if (result && typeof result.catch === "function") result.catch((error) => this.reportError(error));
        } catch (error) {
          this.reportError(error);
        }
      }, delay);
      this.occurrenceTimers.set(key, timer);
    }

    this.scheduleRefresh(delayUntilNextShanghaiDay(refreshedAt, from));
  }

  stop() {
    this.stopped = true;
    this.revision += 1;
    this.clearOccurrenceTimers();
    this.clearRefreshTimer();
    this.fired.clear();
  }

  get scheduledCount() {
    return this.occurrenceTimers.size;
  }

  clearOccurrenceTimers() {
    for (const timer of this.occurrenceTimers.values()) this.clearTimer(timer);
    this.occurrenceTimers.clear();
  }

  clearRefreshTimer() {
    if (this.refreshTimer === null) return;
    this.clearTimer(this.refreshTimer);
    this.refreshTimer = null;
  }

  scheduleRefresh(delay) {
    this.clearRefreshTimer();
    this.refreshTimer = this.setTimer(() => {
      this.refreshTimer = null;
      void this.refresh();
    }, delay);
  }

  reportError(error) {
    try {
      this.onError(error instanceof Error ? error : new Error(String(error)));
    } catch {
      // Error reporting must not stop the remaining reminder timers.
    }
  }
}

module.exports = {
  ReminderScheduler,
  dateKeyAt,
  delayUntilNextShanghaiDay,
  isReminderCandidate,
  nextDateKey,
  occurrenceKey,
};
