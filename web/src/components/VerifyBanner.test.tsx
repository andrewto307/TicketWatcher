import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { VerifyBanner } from "./VerifyBanner";
import { api } from "../api";

vi.mock("../api", () => ({ api: { resendVerification: vi.fn() } }));

beforeEach(() => {
  vi.mocked(api.resendVerification).mockResolvedValue(undefined);
});

describe("VerifyBanner", () => {
  it("says where the link went and what unverified actually costs", () => {
    render(<VerifyBanner email="user@example.test" />);

    expect(screen.getByText("user@example.test")).toBeInTheDocument();
    // The consequence is specific: watches keep working, only email is withheld.
    const text = screen.getByText(/confirm your email/i).closest("span")?.textContent ?? "";
    expect(text).toMatch(/keep tracking/i);
    expect(text).toMatch(/won't be emailed/i);
  });

  it("resends and confirms, without leaving the button clickable again", async () => {
    render(<VerifyBanner email="user@example.test" />);

    await userEvent.click(screen.getByRole("button", { name: /resend email/i }));

    expect(api.resendVerification).toHaveBeenCalledTimes(1);
    expect(await screen.findByText(/check your inbox/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /resend email/i })).not.toBeInTheDocument();
  });

  it("shows the server's reason when resending fails", async () => {
    vi.mocked(api.resendVerification).mockRejectedValue(new Error("email is already verified"));
    render(<VerifyBanner email="user@example.test" />);

    await userEvent.click(screen.getByRole("button", { name: /resend email/i }));

    expect(await screen.findByText(/already verified/i)).toBeInTheDocument();
    // Failure must not claim success.
    expect(screen.queryByText(/check your inbox/i)).not.toBeInTheDocument();
  });

  it("disables the button while the request is in flight", async () => {
    let release!: () => void;
    vi.mocked(api.resendVerification).mockReturnValue(
      new Promise<void>((res) => {
        release = res;
      }),
    );
    render(<VerifyBanner email="user@example.test" />);

    await userEvent.click(screen.getByRole("button", { name: /resend email/i }));

    expect(screen.getByRole("button")).toBeDisabled();
    release();
    await waitFor(() => expect(screen.getByText(/check your inbox/i)).toBeInTheDocument());
  });
});
