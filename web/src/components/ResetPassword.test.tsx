import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ResetPassword } from "./ResetPassword";
import { api } from "../api";

vi.mock("../api", () => ({ api: { resetPassword: vi.fn() } }));

beforeEach(() => {
  vi.mocked(api.resetPassword).mockResolvedValue(undefined);
});

const newPw = () => screen.getByPlaceholderText(/new password \(min 8/i);
const confirmPw = () => screen.getByPlaceholderText(/confirm new password/i);

describe("ResetPassword", () => {
  it("submits the token from the URL with the new password", async () => {
    render(<ResetPassword token="tok-123" />);

    await userEvent.type(newPw(), "brand-new-password");
    await userEvent.type(confirmPw(), "brand-new-password");
    await userEvent.click(screen.getByRole("button", { name: /set new password/i }));

    await waitFor(() => expect(api.resetPassword).toHaveBeenCalledWith("tok-123", "brand-new-password"));
  });

  it("catches a mismatch client-side instead of wasting the single-use token", async () => {
    render(<ResetPassword token="tok-123" />);

    await userEvent.type(newPw(), "password-one");
    await userEvent.type(confirmPw(), "password-two");
    await userEvent.click(screen.getByRole("button", { name: /set new password/i }));

    expect(await screen.findByText(/don't match/i)).toBeInTheDocument();
    // The reset token can only be redeemed once, so a typo must not consume it.
    expect(api.resetPassword).not.toHaveBeenCalled();
  });

  it("confirms success and offers a way to log in", async () => {
    render(<ResetPassword token="tok-123" />);

    await userEvent.type(newPw(), "brand-new-password");
    await userEvent.type(confirmPw(), "brand-new-password");
    await userEvent.click(screen.getByRole("button", { name: /set new password/i }));

    expect(await screen.findByText(/password has been changed/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /go to login/i })).toBeInTheDocument();
    // The form is gone, so the spent token cannot be resubmitted.
    expect(screen.queryByPlaceholderText(/new password/i)).not.toBeInTheDocument();
  });

  it("explains an expired or reused link in the server's own words", async () => {
    vi.mocked(api.resetPassword).mockRejectedValue(new Error("this link is invalid or has expired"));
    render(<ResetPassword token="stale" />);

    await userEvent.type(newPw(), "brand-new-password");
    await userEvent.type(confirmPw(), "brand-new-password");
    await userEvent.click(screen.getByRole("button", { name: /set new password/i }));

    expect(await screen.findByText(/invalid or has expired/i)).toBeInTheDocument();
    // Still usable — the user may have another, valid link.
    expect(screen.getByRole("button", { name: /set new password/i })).toBeEnabled();
  });

  it("enforces a minimum password length in the form", () => {
    render(<ResetPassword token="tok-123" />);

    expect(newPw()).toHaveAttribute("minLength", "8");
    expect(newPw()).toHaveAttribute("type", "password");
    expect(confirmPw()).toHaveAttribute("type", "password");
  });
});
