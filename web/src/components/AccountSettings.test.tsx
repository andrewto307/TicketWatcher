import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AccountSettings } from "./AccountSettings";
import type { Me } from "../types";
import { api } from "../api";

vi.mock("../api", () => ({
  api: { resubscribe: vi.fn(), deleteAccount: vi.fn() },
}));

function me(over: Partial<Me> = {}): Me {
  return { id: 1, email: "user@example.test", email_verified: true, unsubscribed: false, ...over };
}

beforeEach(() => {
  vi.mocked(api.resubscribe).mockResolvedValue(undefined);
  vi.mocked(api.deleteAccount).mockResolvedValue(undefined);
});

describe("AccountSettings — deleting an account", () => {
  // Account deletion is irreversible and sits next to everyday controls, so the
  // guard rail matters more than any other interaction in the app.

  it("does not delete on a single click", async () => {
    render(<AccountSettings me={me()} onChanged={vi.fn()} />);

    await userEvent.click(screen.getByRole("button", { name: /delete account/i }));

    expect(api.deleteAccount).not.toHaveBeenCalled();
    expect(screen.getByText(/permanently deletes your account/i)).toBeInTheDocument();
  });

  it("spells out exactly what is lost", async () => {
    render(<AccountSettings me={me()} onChanged={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: /delete account/i }));

    // getByText lands on the <strong>; the full sentence is on its parent <p>.
    const warning = screen.getByText(/permanently deletes your account/i).closest("p");
    expect(warning?.textContent).toMatch(/every watch/i);
    expect(warning?.textContent).toMatch(/status history/i);
    expect(warning?.textContent).toMatch(/cannot be undone/i);
  });

  it("keeps the confirm button disabled until DELETE is typed exactly", async () => {
    render(<AccountSettings me={me()} onChanged={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: /delete account/i }));

    const confirm = screen.getByRole("button", { name: /permanently delete my account/i });
    const input = screen.getByLabelText(/type delete to confirm/i);

    expect(confirm).toBeDisabled();

    await userEvent.type(input, "delete"); // lowercase must not count
    expect(confirm).toBeDisabled();

    await userEvent.clear(input);
    await userEvent.type(input, "DELETE ME");
    expect(confirm).toBeDisabled();

    await userEvent.clear(input);
    await userEvent.type(input, "DELETE");
    expect(confirm).toBeEnabled();
  });

  it("deletes only after the typed confirmation", async () => {
    render(<AccountSettings me={me()} onChanged={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: /delete account/i }));
    await userEvent.type(screen.getByLabelText(/type delete to confirm/i), "DELETE");
    await userEvent.click(screen.getByRole("button", { name: /permanently delete my account/i }));

    await waitFor(() => expect(api.deleteAccount).toHaveBeenCalledTimes(1));
  });

  it("lets the user back out, and forgets what they typed", async () => {
    render(<AccountSettings me={me()} onChanged={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: /delete account/i }));
    await userEvent.type(screen.getByLabelText(/type delete to confirm/i), "DELETE");
    await userEvent.click(screen.getByRole("button", { name: /cancel/i }));

    expect(api.deleteAccount).not.toHaveBeenCalled();

    // Re-opening must start from a disabled state, not a primed one.
    await userEvent.click(screen.getByRole("button", { name: /delete account/i }));
    expect(screen.getByRole("button", { name: /permanently delete my account/i })).toBeDisabled();
  });

  it("shows the error and stays on screen if deletion fails", async () => {
    vi.mocked(api.deleteAccount).mockRejectedValue(new Error("server exploded"));
    render(<AccountSettings me={me()} onChanged={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: /delete account/i }));
    await userEvent.type(screen.getByLabelText(/type delete to confirm/i), "DELETE");
    await userEvent.click(screen.getByRole("button", { name: /permanently delete my account/i }));

    expect(await screen.findByText(/server exploded/i)).toBeInTheDocument();
    // The user must be able to retry rather than be stuck on a dead button.
    expect(screen.getByRole("button", { name: /permanently delete my account/i })).toBeEnabled();
  });
});

describe("AccountSettings — the unsubscribed state", () => {
  it("says nothing about alerts being off when the user is subscribed", () => {
    render(<AccountSettings me={me()} onChanged={vi.fn()} />);

    expect(screen.queryByText(/alerts are off/i)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /turn alerts back on/i })).not.toBeInTheDocument();
  });

  it("explains that watches keep tracking even with alerts off", () => {
    render(<AccountSettings me={me({ unsubscribed: true })} onChanged={vi.fn()} />);

    const notice = screen.getByText(/alerts are off/i).closest("span");
    expect(notice?.textContent).toMatch(/keep tracking/i);
    expect(notice?.textContent).toMatch(/nothing is emailed/i);
  });

  it("resubscribes and refreshes", async () => {
    const onChanged = vi.fn();
    render(<AccountSettings me={me({ unsubscribed: true })} onChanged={onChanged} />);

    await userEvent.click(screen.getByRole("button", { name: /turn alerts back on/i }));

    expect(api.resubscribe).toHaveBeenCalled();
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });

  it("reports a failed resubscribe rather than appearing to succeed", async () => {
    vi.mocked(api.resubscribe).mockRejectedValue(new Error("nope"));
    const onChanged = vi.fn();
    render(<AccountSettings me={me({ unsubscribed: true })} onChanged={onChanged} />);

    await userEvent.click(screen.getByRole("button", { name: /turn alerts back on/i }));

    expect(await screen.findByText(/nope/i)).toBeInTheDocument();
    expect(onChanged).not.toHaveBeenCalled();
  });

  it("shows which account is signed in", () => {
    render(<AccountSettings me={me({ email: "someone@example.test" })} onChanged={vi.fn()} />);

    expect(screen.getByText("someone@example.test")).toBeInTheDocument();
  });
});
