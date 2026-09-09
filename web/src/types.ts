// These mirror the Go JSON DTOs (service.WatchView, SnapshotView, etc.).

export type Availability = "onsale" | "offsale" | "cancelled" | "unknown" | string;

export interface EventResult {
  tm_event_id: string;
  name: string;
  venue: string;
  event_date: string | null;
  min_price: number | null;
  max_price: number | null;
  availability: Availability;
}

export interface Watch {
  id: number;
  tm_event_id: string;
  event_name: string;
  venue: string;
  event_date: string | null;
  condition_type: "price_below" | "becomes_available";
  threshold: number | null;
  status: "active" | "paused" | "triggered";
  current_min_price: number | null;
  current_max_price: number | null;
  availability: Availability | null;
  poll_interval_s: number;
  created_at: string;
}

export interface Snapshot {
  min_price: number | null;
  max_price: number | null;
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
