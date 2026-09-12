// These mirror the Go JSON DTOs (service.WatchView, SnapshotView, etc.).

export type Availability = "onsale" | "offsale" | "cancelled" | "unknown" | string;

export interface EventResult {
  tm_event_id: string;
  name: string;
  venue: string;
  event_date: string | null;
  availability: Availability;
}

export interface Watch {
  id: number;
  tm_event_id: string;
  event_name: string;
  venue: string;
  event_date: string | null;
  condition_type: "becomes_available";
  status: "active" | "paused" | "triggered";
  availability: Availability | null;
  poll_interval_s: number;
  created_at: string;
  last_polled_at: string | null;
}

export interface Snapshot {
  availability: string | null;
  checked_at: string;
}

// The signed-in user's own account state (service.UserView).
export interface Me {
  id: number;
  email: string;
  email_verified: boolean;
  unsubscribed: boolean;
}

export interface Notification {
  id: number;
  watch_id: number;
  channel: string;
  sent_at: string;
  payload: Record<string, unknown>;
}
