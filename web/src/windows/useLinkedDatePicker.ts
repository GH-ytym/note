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

export default function useLinkedDatePicker<T extends ScheduleForm | null>({ setForm, setCustomDates, setError, inline = false }: { inline?: boolean; setForm: Dispatch<SetStateAction<T>>; setCustomDates: Dispatch<SetStateAction<string[]>>; setError: (error: string) => void }) {
  const [active, setActive] = useState(false);
  const [field, setField] = useState<"date" | "endDate">("date");

  useEffect(() => {
    if (inline) {
      const receive = (event: Event) => {
        const state = (event as CustomEvent<PickerState>).detail;
        const key = state.field === "endDate" ? "endDate" : "date";
        setForm(current => current ? {...current, [key]:validDate(state.date)} : current);
        if (key === "date") setCustomDates(state.customDates || []);
        setError("");
      };
      const finish = () => setActive(false);
      window.addEventListener("note-inline-selection", receive);
      window.addEventListener("note-inline-finished", finish);
      return () => { window.removeEventListener("note-inline-selection", receive); window.removeEventListener("note-inline-finished", finish); window.dispatchEvent(new CustomEvent("note-inline-picker", {detail:null})); };
    }
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
  }, [setCustomDates, setError, setForm, inline]);

  async function start(form: ScheduleForm, customDates: string[], nextField: "date" | "endDate" = "date") {
    if (inline) { setActive(true); setField(nextField); window.dispatchEvent(new CustomEvent("note-inline-picker", {detail:pickerPayload(form, customDates, nextField)})); return; }
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
    if (inline && active) { window.dispatchEvent(new CustomEvent("note-inline-picker", {detail:pickerPayload(form, customDates, field)})); return; }
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
