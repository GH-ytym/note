import { useEffect, useState } from "react";
import { REPEAT_VALUES, validDate } from "../lib/calendar";

function pickerPayload(form, customDates, field = "date") {
  return {
    field,
    repeatMode: field === "endDate" ? "once" : REPEAT_VALUES[form.repeat],
    date: form[field],
    customDates: field === "date" && form.repeat === "自定义" ? customDates : [],
    color: form.color,
  };
}

export default function useLinkedDatePicker({ setForm, setCustomDates, setError }) {
  const [active, setActive] = useState(false);
  const [field, setField] = useState("date");

  useEffect(() => {
    const removeSelectionListener = window.noteDesktop?.onDatePickerSelection?.((selection) => {
      if (!selection) return;
      const selectedField = selection.field === "endDate" ? "endDate" : "date";
      setForm((current) => current ? { ...current, [selectedField]: validDate(selection.date) } : current);
      if (selectedField === "date") setCustomDates(Array.isArray(selection.customDates) ? selection.customDates : []);
      setError("");
    });
    const removeFinishedListener = window.noteDesktop?.onDatePickerFinished?.(() => setActive(false));

    return () => {
      removeSelectionListener?.();
      removeFinishedListener?.();
    };
  }, [setCustomDates, setError, setForm]);

  async function start(form, customDates, nextField = "date") {
    if (!window.noteDesktop?.startDatePicker) return;
    try {
      setActive(true);
      setField(nextField);
      await window.noteDesktop.startDatePicker(pickerPayload(form, customDates, nextField));
    } catch (pickerError) {
      setActive(false);
      setError(pickerError.message);
    }
  }

  async function update(form, customDates) {
    if (!active || !window.noteDesktop?.updateDatePicker) return;
    try {
      await window.noteDesktop.updateDatePicker(pickerPayload(form, customDates, field));
    } catch (pickerError) {
      setActive(false);
      setError(pickerError.message);
    }
  }

  return { active, field, start, update };
}
