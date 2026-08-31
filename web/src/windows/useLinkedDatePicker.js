import { useEffect, useState } from "react";
import { REPEAT_VALUES, validDate } from "../lib/calendar";

function pickerPayload(form, customDates) {
  return {
    repeatMode: REPEAT_VALUES[form.repeat],
    date: form.date,
    customDates: form.repeat === "自定义" ? customDates : [],
    color: form.color,
  };
}

export default function useLinkedDatePicker({ setForm, setCustomDates, setError }) {
  const [active, setActive] = useState(false);

  useEffect(() => {
    const removeSelectionListener = window.noteDesktop?.onDatePickerSelection?.((selection) => {
      if (!selection) return;
      setForm((current) => current ? { ...current, date: validDate(selection.date) } : current);
      setCustomDates(Array.isArray(selection.customDates) ? selection.customDates : []);
      setError("");
    });
    const removeFinishedListener = window.noteDesktop?.onDatePickerFinished?.(() => setActive(false));

    return () => {
      removeSelectionListener?.();
      removeFinishedListener?.();
    };
  }, [setCustomDates, setError, setForm]);

  async function start(form, customDates) {
    if (!window.noteDesktop?.startDatePicker) return;
    try {
      setActive(true);
      await window.noteDesktop.startDatePicker(pickerPayload(form, customDates));
    } catch (pickerError) {
      setActive(false);
      setError(pickerError.message);
    }
  }

  async function update(form, customDates) {
    if (!active || !window.noteDesktop?.updateDatePicker) return;
    try {
      await window.noteDesktop.updateDatePicker(pickerPayload(form, customDates));
    } catch (pickerError) {
      setActive(false);
      setError(pickerError.message);
    }
  }

  return { active, start, update };
}
