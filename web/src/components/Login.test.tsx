import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { Login } from "./Login";
import { api } from "../api";
import { getToken } from "../auth";

vi.mock("../api", () => ({
  api: { login: vi.fn(), register: vi.fn(), forgotPassword: vi.fn() },
}));

beforeEach(() => {
  vi.mocked(api.login).mockResolvedValue({ token: "jwt-login" });
  vi.mocked(api.register).mockResolvedValue({ token: "jwt-register" });
  vi.mocked(api.forgotPassword).mockResolvedValue(undefined);
});

const email = () => screen.getByPlaceholderText("email");
const password = () => screen.getByPlaceholderText(/password \(min 8/i);

describe("Login — signing in", () => {
  it("stores the token and tells the app it is authenticated", async () => {
    const onAuthed = vi.fn();
    render(<Login onAuthed={onAuthed} />);

    await userEvent.type(email(), "u@example.test");
    await userEvent.type(password(), "password123");
    await userEvent.click(screen.getByRole("button", { name: "Log in" }));

    expect(api.login).toHaveBeenCalledWith("u@example.test", "password123");
    await waitFor(() => expect(onAuthed).toHaveBeenCalled());
    expect(getToken()).toBe("jwt-login");
  });

  it("shows the server's message and does not authenticate on failure", async () => {
    vi.mocked(api.login).mockRejectedValue(new Error("invalid email or password"));
    const onAuthed = vi.fn();
    render(<Login onAuthed={onAuthed} />);

    await userEvent.type(email(), "u@example.test");
    await userEvent.type(password(), "wrongpassword");
    await userEvent.click(screen.getByRole("button", { name: "Log in" }));

    expect(await screen.findByText(/invalid email or password/i)).toBeInTheDocument();
    expect(onAuthed).not.toHaveBeenCalled();
    expect(getToken()).toBeNull();
  });

  it("surfaces the rate-limit message rather than a generic failure", async () => {
    vi.mocked(api.login).mockRejectedValue(new Error("too many requests — please slow down"));
    render(<Login onAuthed={vi.fn()} />);

    await userEvent.type(email(), "u@example.test");
    await userEvent.type(password(), "password123");
    await userEvent.click(screen.getByRole("button", { name: "Log in" }));

    expect(await screen.findByText(/too many requests/i)).toBeInTheDocument();
  });
});

describe("Login — registering", () => {
  it("switches to register mode and creates the account", async () => {
    const onAuthed = vi.fn();
    render(<Login onAuthed={onAuthed} />);

    await userEvent.click(screen.getByRole("button", { name: /need an account/i }));
    await userEvent.type(email(), "new@example.test");
    await userEvent.type(password(), "password123");
    await userEvent.click(screen.getByRole("button", { name: "Register" }));

    expect(api.register).toHaveBeenCalledWith("new@example.test", "password123");
    await waitFor(() => expect(getToken()).toBe("jwt-register"));
  });

  it("requires a password of at least 8 characters at the form level", async () => {
    render(<Login onAuthed={vi.fn()} />);

    expect(password()).toHaveAttribute("minLength", "8");
    expect(password()).toBeRequired();
    expect(email()).toHaveAttribute("type", "email");
  });

  it("clears a previous error when switching mode", async () => {
    vi.mocked(api.login).mockRejectedValue(new Error("invalid email or password"));
    render(<Login onAuthed={vi.fn()} />);

    await userEvent.type(email(), "u@example.test");
    await userEvent.type(password(), "password123");
    await userEvent.click(screen.getByRole("button", { name: "Log in" }));
    expect(await screen.findByText(/invalid email or password/i)).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /need an account/i }));

    expect(screen.queryByText(/invalid email or password/i)).not.toBeInTheDocument();
  });
});

describe("Login — forgotten password", () => {
  it("is reachable only from the login screen", async () => {
    render(<Login onAuthed={vi.fn()} />);
    expect(screen.getByRole("button", { name: /forgot password/i })).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /need an account/i }));
    expect(screen.queryByRole("button", { name: /forgot password/i })).not.toBeInTheDocument();
  });

  it("asks only for an email", async () => {
    render(<Login onAuthed={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: /forgot password/i }));

    expect(email()).toBeInTheDocument();
    expect(screen.queryByPlaceholderText(/password \(min 8/i)).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /send reset link/i })).toBeInTheDocument();
  });

  it("gives the same confirmation regardless of whether the account exists", async () => {
    render(<Login onAuthed={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: /forgot password/i }));
    await userEvent.type(email(), "maybe@example.test");
    await userEvent.click(screen.getByRole("button", { name: /send reset link/i }));

    // Deliberately non-committal: a different message for known vs unknown
    // addresses would let anyone discover who has an account.
    const notice = await screen.findByText(/if an account exists/i);
    expect(notice.textContent).toMatch(/maybe@example\.test/);
    expect(notice.textContent).toMatch(/expires in an hour/i);
    expect(api.forgotPassword).toHaveBeenCalledWith("maybe@example.test");
  });

  it("offers a way back to login after sending", async () => {
    render(<Login onAuthed={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: /forgot password/i }));
    await userEvent.type(email(), "u@example.test");
    await userEvent.click(screen.getByRole("button", { name: /send reset link/i }));

    await userEvent.click(await screen.findByRole("button", { name: /back to login/i }));

    expect(screen.getByRole("button", { name: "Log in" })).toBeInTheDocument();
  });
});

describe("Login — privacy", () => {
  it("links the privacy policy before signup, which is when it matters", () => {
    render(<Login onAuthed={vi.fn()} />);

    expect(screen.getByRole("link", { name: /privacy policy/i })).toHaveAttribute("href", "/privacy");
  });
});
