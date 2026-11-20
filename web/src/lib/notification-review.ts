import type { NotificationCard } from "./notification-state";

export type Decision = "accept" | "reject";
export type ReviewState = { busy: boolean; error: string; done?: "accepted" | "rejected" };
export type Reviews = Record<number, ReviewState>;

// 所有通知入口共用此状态，按申请 ID 防止列表和详情重复提交。
export function createReviewer(
  request: (path: string, options: RequestInit) => Promise<unknown>,
  changed: (states: Reviews) => void,
  sync: () => void,
) {
  const states: Reviews = {};
  return async (card: NotificationCard, decision: Decision) => {
    const id = card.join_request_id;
    if (!id || card.type !== "join_requested" || card.request_status !== "pending" || states[id]?.busy || states[id]?.done) return;
    states[id] = { busy: true, error: "" };
    changed({ ...states });
    try {
      await request(`/groups/join-requests/${id}/review`, { method: "POST", body: JSON.stringify({ decision }) });
      states[id] = { busy: false, error: "", done: decision === "accept" ? "accepted" : "rejected" };
      changed({ ...states });
      // 重新取得服务端快照，不在客户端扣计数或递增版本。
      sync();
    } catch (reason) {
      states[id] = { busy: false, error: reason instanceof Error ? reason.message : "审核失败，请重试" };
      changed({ ...states });
      const status = (reason as { status?: number } | null)?.status;
      if (status === 409 || status === 404) sync();
    }
  };
}

export const reviewLabels: Record<string, string> = {
  pending: "待审核", accepted: "已同意", rejected: "已拒绝", cancelled: "已撤销",
};

export function notificationAction(card: NotificationCard) {
  if (card.type === "join_requested") return "申请加入";
  if (card.type === "joined") return "加入了";
  if (card.type === "join_reviewed") return card.request_status === "accepted" ? "已通过你的入群申请：" : "已拒绝你的入群申请：";
  return "更新了";
}
