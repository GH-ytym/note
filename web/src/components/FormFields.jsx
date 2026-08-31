import { useEffect, useRef, useState } from "react";
import {
  CaretDown,
  CaretUp,
  Check,
  Clock,
  Palette,
  Sparkle,
} from "@phosphor-icons/react";
import { EVENT_COLORS, joinTime, pad, splitTime } from "../lib/calendar";

export function CustomColorSwatch({ value, selected, onChange }) {
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

export function SelectField({ label, value, options, onChange }) {
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

export function TimeField({ label = "时间", value, onChange, compact = false }) {
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState(value || "00:00");
  const popoverRef = useRef(null);

  useEffect(() => {
    if (!open) setDraft(value || "00:00");
  }, [open, value]);

  useEffect(() => {
    if (!open || !popoverRef.current) return undefined;

    const popover = popoverRef.current;
    function handleNativeWheel(event) {
      event.preventDefault();
      event.stopPropagation();

      const unitElement = event.target instanceof Element
        ? event.target.closest("[data-time-unit]")
        : null;
      if (!unitElement || !popover.contains(unitElement)) return;

      const unit = unitElement.dataset.timeUnit;
      const amount = event.deltaY > 0 ? 1 : -1;
      setDraft((current) => {
        const { hour, minute } = splitTime(current);
        return unit === "hour"
          ? joinTime(hour + amount, minute)
          : joinTime(hour, minute + amount);
      });
    }

    popover.addEventListener("wheel", handleNativeWheel, { passive: false });
    return () => popover.removeEventListener("wheel", handleNativeWheel);
  }, [open]);

  function adjust(unit, amount) {
    setDraft((current) => {
      const { hour, minute } = splitTime(current);
      return unit === "hour"
        ? joinTime(hour + amount, minute)
        : joinTime(hour, minute + amount);
    });
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
        <div ref={popoverRef} className="time-popover" role="dialog" aria-label={`${label}选择器`}>
          <div className="time-dials">
            <div className="time-unit" data-time-unit="hour">
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

            <div className="time-unit" data-time-unit="minute">
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

export function ColorField({ value, onChange, allowRandom = false }) {
  const [open, setOpen] = useState(false);
  const selected = EVENT_COLORS.find((item) => item.value === value);
  const custom = Boolean(value) && !selected;

  return (
    <div className="color-field">
      <span className="field-label">颜色</span>
      <button
        className="color-trigger"
        type="button"
        aria-expanded={open}
        onClick={() => setOpen((current) => !current)}
      >
        <span
          className={`color-trigger-swatch ${value ? "" : "is-random"}`}
          style={value ? { background: value } : undefined}
        />
        <span>{selected?.label || (value ? "自定义" : "随机")}</span>
        <Palette size={17} />
      </button>

      {open && (
        <div className="color-palette">
          <div className="palette-grid">
            {allowRandom && (
              <button
                className={`palette-swatch random-swatch ${value ? "" : "is-selected"}`}
                type="button"
                onClick={() => onChange("")}
                aria-label="随机颜色"
              >
                <Sparkle size={14} weight="fill" />
              </button>
            )}
            {EVENT_COLORS.map((color) => (
              <button
                className={`palette-swatch ${value === color.value ? "is-selected" : ""}`}
                type="button"
                style={{ "--swatch-color": color.value }}
                onClick={() => onChange(color.value)}
                aria-label={color.label}
                key={color.value}
              >
                {value === color.value && <Check size={12} weight="bold" />}
              </button>
            ))}
            <CustomColorSwatch value={value} selected={custom} onChange={onChange} />
          </div>
        </div>
      )}
    </div>
  );
}
