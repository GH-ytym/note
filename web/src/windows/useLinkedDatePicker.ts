import { useEffect, useState } from "react";
import type { Dispatch, SetStateAction } from "react";
import type { ScheduleForm } from "../types";
import type { PickerState } from "../desktop";
import { REPEAT_VALUES, validDate } from "../lib/calendar";

function pickerPayload(form: ScheduleForm, customDates: string[], field: "date" | "endDate" = "date"): PickerState {
  return {
    field,
    repeatMode: field === "endDate" ? "once" : REPEAT_VALUES[form.repeat],
    date: form[field] || form.date,
    customDates: field === "date" && form.repeat === "自定义" ? customDates : [],
    color: form.color,
  };
}

export default function useLinkedDatePicker<T extends ScheduleForm | null>({ setForm, setCustomDates, setError }: { setForm: Dispatch<SetStateAction<T>>; setCustomDates: Dispatch<SetStateAction<string[]>>; setError: (error: string) => void }) {
  const [active, setActive] = useState(false);
  const [field, setField] = useState<"date" | "endDate">("date");

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

  async function start(form: ScheduleForm, customDates: string[], nextField: "date" | "endDate" = "date") {
    if (!window.noteDesktop?.startDatePicker) return;
    try {
      setActive(true);
      setField(nextField);
      await window.noteDesktop.startDatePicker(pickerPayload(form, customDates, nextField));
    } catch (pickerError) {
      setActive(false);
      setError(pickerError instanceof Error ? pickerError.message : "日期选择失败");
    }
  }

  async function update(form: ScheduleForm, customDates: string[]) {
    if (!active || !window.noteDesktop?.updateDatePicker) return;
    try {
      await window.noteDesktop.updateDatePicker(pickerPayload(form, customDates, field));
    } catch (pickerError) {
      setActive(false);
      setError(pickerError instanceof Error ? pickerError.message : "日期选择失败");
    }
  }

  return { active, field, start, update };
}
