import type { Notification } from "../types";

export function NotificationLog({ notifications }: { notifications: Notification[] }) {
  if (notifications.length === 0) {
    return <p className="muted">No alerts sent yet.</p>;
  }
  return (
    <ul className="notif-log">
      {notifications.map((n) => {
        const subject = (n.payload?.subject as string | undefined) ?? `${n.channel} alert`;
        return (
          <li key={n.id}>
            <span className="notif-channel">{n.channel}</span>
            <span className="notif-subject">{subject}</span>
            <span className="muted notif-time">{new Date(n.sent_at).toLocaleString()}</span>
          </li>
        );
      })}
    </ul>
  );
}
