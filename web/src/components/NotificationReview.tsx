import { useContext } from "react";
import { InboxContext } from "./Workspace";
import type { NotificationCard } from "../hooks/useNotifications";
import { reviewLabels } from "../lib/notification-review";

export default function NotificationReview({ card, disabled = false, fresh = false }: { card: NotificationCard; disabled?: boolean; fresh?: boolean }) {
  const inbox = useContext(InboxContext)!;
  const live = fresh ? card : inbox.items.find(item => item.id === card.id) ?? card;
  const state = live.join_request_id ? inbox.reviews[live.join_request_id] : undefined;
  const status = live.request_status === "pending" ? state?.done ?? live.request_status : live.request_status;
  if (live.type !== "join_requested" || !live.join_request_id) return null;
  return <div className="notification-review">
    {status === "pending" ? <div className="notification-actions" aria-busy={state?.busy ?? false}>
      <button className="review-accept" disabled={disabled || state?.busy} onClick={() => void inbox.review(live, "accept")}>{state?.busy ? "处理中…" : "同意"}</button>
      <button disabled={disabled || state?.busy} onClick={() => void inbox.review(live, "reject")}>拒绝</button>
    </div> : <span role="status">{reviewLabels[status ?? ""] ?? status}</span>}
    {state?.error && <p className="notification-error" role="alert">{state.error}</p>}
  </div>;
}
