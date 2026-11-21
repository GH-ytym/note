// 一个当前范围只保留一个请求；查询期间的新触发合并为下一次查询。
export function createCalendarSync<T>(options: {
  load: (signal: AbortSignal) => Promise<T>;
  apply: (value: T) => void;
  fail: (reason: unknown) => void;
  settled?: () => void;
  interval?: number;
}) {
  const controller = new AbortController();
  let active = false, stopped = false, dirty = false;
  let running: Promise<void> | undefined;
  let timer: ReturnType<typeof setTimeout> | undefined;

  function request(): Promise<void> {
    if (stopped) return Promise.resolve();
    dirty = true;
    clearTimeout(timer);
    if (!active) return running ?? Promise.resolve();
    if (running) return running;
    // 合并同一轮事件循环的触发，赋值 running 后才开始执行。
    running = Promise.resolve().then(async () => {
      while (dirty && active && !stopped) {
        dirty = false;
        try {
          const value = await options.load(controller.signal);
          if (!stopped) options.apply(value);
        } catch (reason) {
          if (!stopped) options.fail(reason);
        } finally {
          if (!stopped) options.settled?.();
        }
      }
    }).finally(() => {
      running = undefined;
      if (active && !stopped) {
        if (dirty) return request();
        timer = setTimeout(() => void request(), options.interval ?? 60_000);
      }
    });
    return running;
  }

  return {
    request,
    setActive(value: boolean) {
      if (active === value || stopped) return;
      active = value;
      clearTimeout(timer);
      if (active) void request();
    },
    stop() {
      stopped = true;
      clearTimeout(timer);
      controller.abort();
    },
  };
}
