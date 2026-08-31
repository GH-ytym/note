import { useEffect, useState } from "react";
import { CheckCircle } from "@phosphor-icons/react";
import { getTodo, patchTodo } from "../api";
import { validDate } from "../lib/calendar";
import WindowFrame, { LoadingWindow } from "./WindowFrame";
import { closeCurrentWindow, notifyDataChanged, windowParams } from "./window-utils";

export default function ContentEditorWindow() {
  const [editorState, setEditorState] = useState(null);
  const [content, setContent] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const params = windowParams();
    const fallbackTodoID = Number(params.get("todo_id"));
    const request = window.noteDesktop?.getContentEditorState?.()
      || (Number.isSafeInteger(fallbackTodoID) && fallbackTodoID > 0
        ? getTodo(fallbackTodoID).then((todo) => ({
            todoId: todo.id,
            date: validDate(params.get("date")),
            content: todo.content,
            version: todo.version,
          }))
        : null);
    if (!request) {
      setError("专注编辑窗口仅在桌面版中可用");
      setLoading(false);
      return;
    }
    request
      .then((state) => {
        if (!state) throw new Error("编辑会话已经结束");
        setEditorState(state);
        setContent(state.content || "");
      })
      .catch((loadError) => setError(loadError.message))
      .finally(() => setLoading(false));
  }, []);

  async function saveAndReturn(event) {
    event.preventDefault();
    const nextContent = content.trim();
    if (!editorState) return;

    setSaving(true);
    setError("");
    try {
      const updated = await patchTodo(editorState.todoId, {
        content: nextContent,
        version: editorState.version,
      });
      await notifyDataChanged({ type: "updated", todoId: editorState.todoId });
      const result = {
        todoId: editorState.todoId,
        content: updated.content,
        version: updated.version,
      };
      if (window.noteDesktop?.finishContentEditor) {
        await window.noteDesktop.finishContentEditor(result);
      } else {
        window.opener?.postMessage({ type: "note:content-editor-saved", payload: result }, window.location.origin);
        closeCurrentWindow();
      }
    } catch (saveError) {
      setError(saveError.message);
      setSaving(false);
    }
  }

  if (loading) return <LoadingWindow title="专注编辑" />;
  if (!editorState) return <LoadingWindow title="专注编辑" message={error || "编辑会话已经结束"} />;

  return (
    <WindowFrame title="专注编辑" className="is-content-editor-window">
      <form className="focused-editor-form" onSubmit={saveAndReturn}>
        <label htmlFor="focused-content">日程内容</label>
        <textarea
          id="focused-content"
          value={content}
          maxLength={500}
          autoFocus
          disabled={saving}
          onChange={(event) => {
            setContent(event.target.value);
            setError("");
          }}
          onKeyDown={(event) => {
            if ((event.ctrlKey || event.metaKey) && event.key === "Enter") {
              event.preventDefault();
              event.currentTarget.form?.requestSubmit();
            }
          }}
        />
        <div className="focused-editor-status">
          <span className={error ? "is-error" : ""} role={error ? "alert" : undefined}>
            {error || `${content.length} / 500`}
          </span>
          <button className="focused-editor-save" type="submit" disabled={saving}>
            <CheckCircle size={21} weight="fill" aria-hidden="true" />
            {saving ? "保存中" : "保存并返回"}
          </button>
        </div>
      </form>
    </WindowFrame>
  );
}
