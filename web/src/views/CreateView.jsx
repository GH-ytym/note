import { useEffect, useState } from "react";
import { createEvent, createTodo } from "../api";
import { ColorField, SelectField, TimeField } from "../components/FormFields";
import {
  REMINDER_VALUES,
  REPEAT_VALUES,
  TODAY_KEY,
  dateTimeAt,
  validDate,
} from "../lib/calendar";
import LinkedDateField from "../windows/LinkedDateField";
import WindowFrame from "../windows/WindowFrame";
import useLinkedDatePicker from "../windows/useLinkedDatePicker";
import { closeCurrentWindow, notifyDataChanged } from "../windows/window-utils";

export default function CreateView({ initialDate = TODAY_KEY, onDone }) {
	const [kind, setKind] = useState("todo");
	const [form, setForm] = useState({
    title: "",
    content: "",
    repeat: "仅一次",
    date: validDate(initialDate),
		time: "09:00",
		endDate: validDate(initialDate),
		endTime: "10:00",
    reminder: "弹窗提醒",
    color: "",
  });
  const [customDates, setCustomDates] = useState([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const datePicker = useLinkedDatePicker({ setForm, setCustomDates, setError });
	const finish = onDone || closeCurrentWindow;

	useEffect(() => {
		if (form.endDate >= form.date) return;
		setForm((current) => ({ ...current, endDate: current.date }));
	}, [form.date, form.endDate]);

  function setField(name, value) {
    const nextForm = { ...form, [name]: value };
    setForm(nextForm);
    if (name === "color") void datePicker.update(nextForm, customDates);
    setError("");
  }

  function setRepeat(value) {
    const nextForm = { ...form, repeat: value };
    const nextCustomDates = value === "自定义" ? [] : customDates;
    setForm(nextForm);
    setCustomDates(nextCustomDates);
    setError("");
    void datePicker.start(nextForm, nextCustomDates);
  }

  function setNativeDate(value) {
    setField("date", value);
    if (form.repeat === "自定义") setCustomDates(value ? [value] : []);
  }

	async function submit(event) {
    event.preventDefault();
    const title = form.title.trim();
    const content = form.content.trim();
    if (!title) {
      setError("请输入标题");
      return;
    }
    if (!form.date || !form.time) {
      setError("请选择日期和时间");
      return;
    }
    if (form.repeat === "自定义" && customDates.length === 0) {
      setError("请至少添加一个自定义日期");
      return;
    }

		const dates = [...customDates].sort();
		const startsOn = form.repeat === "自定义" ? dates[0] : form.date;
		const common = {
			title,
			content,
			starts_at: dateTimeAt(startsOn, form.time),
			repeat_mode: REPEAT_VALUES[form.repeat],
			...(form.color ? { color: form.color } : {}),
			...(form.repeat === "自定义" ? { custom_dates: dates } : {}),
		};

		let payload;
		if (kind === "event") {
			const endsAt = dateTimeAt(form.endDate, form.endTime);
			if (Date.parse(endsAt) <= Date.parse(common.starts_at)) {
				setError("结束时间必须晚于开始时间");
				return;
			}
			payload = { ...common, ends_at: endsAt };
		} else {
			payload = {
				...common,
				notify_mode: REMINDER_VALUES[form.reminder],
			};
		}

    setSaving(true);
    setError("");
    try {
			const created = kind === "event"
				? await createEvent(payload)
				: await createTodo(payload);
			await notifyDataChanged({
				type: "created",
				...(kind === "event" ? { eventId: created.id } : { todoId: created.id }),
			});
      finish();
    } catch (createError) {
      setError(createError.message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <WindowFrame title="新日程" className="is-form-window" onBack={onDone}>
		<form className="side-form utility-form" onSubmit={submit}>
			<div className="item-kind-switch" role="group" aria-label="类型">
				<button className={kind === "todo" ? "is-active" : ""} type="button" onClick={() => setKind("todo")}>待办</button>
				<button className={kind === "event" ? "is-active" : ""} type="button" onClick={() => setKind("event")}>日程</button>
			</div>
        <label>
          <span>标题</span>
          <input
            value={form.title}
            maxLength={50}
            required
            autoFocus
            onChange={(event) => setField("title", event.target.value)}
          />
        </label>

        <label>
          <span>内容</span>
          <textarea
            value={form.content}
            rows={5}
            maxLength={500}
            onChange={(event) => setField("content", event.target.value)}
          />
        </label>

        <SelectField
          label="重复"
          value={form.repeat}
          options={Object.keys(REPEAT_VALUES)}
          onChange={setRepeat}
        />

		<div className="form-row">
          <LinkedDateField
            repeat={form.repeat}
            date={form.date}
            customDates={customDates}
            color={form.color}
            active={datePicker.active}
            onOpen={() => void datePicker.start(form, customDates)}
            onNativeChange={setNativeDate}
          />
          <TimeField value={form.time} onChange={(value) => setField("time", value)} />
		</div>

		{kind === "event" && (
			<div className="form-row">
				<label>
					<span>结束日期</span>
					<input
						type="date"
						min={form.date || TODAY_KEY}
						value={form.endDate}
						onChange={(event) => setField("endDate", event.target.value)}
					/>
				</label>
				<TimeField label="结束时间" value={form.endTime} onChange={(value) => setField("endTime", value)} />
			</div>
		)}

		{kind === "todo" && <SelectField
			label="提醒"
          value={form.reminder}
          options={Object.keys(REMINDER_VALUES)}
          onChange={(value) => setField("reminder", value)}
		/>}

        <ColorField value={form.color} onChange={(value) => setField("color", value)} allowRandom />

        {error && <p className="form-error" role="alert">{error}</p>}
        <footer className="side-footer utility-footer">
          <button className="cancel-button" type="button" onClick={finish}>取消</button>
          <button className="save-button" type="submit" disabled={saving}>{saving ? "创建中" : "创建"}</button>
        </footer>
      </form>
    </WindowFrame>
  );
}
