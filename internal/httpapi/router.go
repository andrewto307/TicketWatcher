// Package httpapi wires HTTP routes to domain services.
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"ticket-watcher/internal/auth"
	"ticket-watcher/internal/ratelimit"
	"ticket-watcher/internal/service"
)

// NewRouter builds the application's HTTP handler. Auth endpoints are public but
// rate-limited per IP; everything else under /api requires a valid Bearer token
// (jwtSecret verifies it). If staticFS is non-nil it serves the embedded SPA on
// all other paths (with an index.html fallback for client-side routing); pass nil
// in tests. A nil authLimiter disables inbound throttling (tests only).
func NewRouter(
	authSvc *service.AuthService,
	search *service.SearchService,
	watches *service.WatchService,
	notifications *service.NotificationService,
	accounts *service.AccountService,
	jwtSecret string,
	authLimiter *ratelimit.IPLimiter,
	staticFS fs.FS,
) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", healthHandler)

	r.Route("/api", func(r chi.Router) {
		// Public, and the most abusable surface in the app: unauthenticated,
		// password-guessable, and email-sending. Everything here is throttled.
		r.Group(func(r chi.Router) {
			if authLimiter != nil {
				r.Use(rateLimit(authLimiter))
			}
			r.Post("/auth/register", registerHandler(authSvc))
			r.Post("/auth/login", loginHandler(authSvc))
			r.Post("/auth/forgot", forgotPasswordHandler(authSvc))
			r.Post("/auth/reset", resetPasswordHandler(authSvc))
			// GET so the link works straight from an email client.
			r.Get("/auth/verify", verifyEmailHandler(authSvc))
		})

		// Opt-out. Public and unauthenticated by design: someone acting on an old
		// email must be able to stop the mail without digging up a password. The
		// HMAC token is the authorization. POST implements RFC 8058 one-click, which
		// Gmail and Outlook call directly from their own unsubscribe button.
		r.Get("/unsubscribe", unsubscribeHandler(accounts, false))
		r.Post("/unsubscribe", unsubscribeHandler(accounts, true))

		// Authenticated.
		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware(jwtSecret))
			r.Get("/me", meHandler(authSvc))
			r.Post("/auth/verify/resend", resendVerificationHandler(authSvc))
			r.Get("/search", searchHandler(search))
			r.Post("/watches", createWatchHandler(watches))
			r.Get("/watches", listWatchesHandler(watches))
			r.Get("/watches/{id}/history", watchHistoryHandler(watches))
			r.Patch("/watches/{id}", updateWatchHandler(watches))
			r.Delete("/watches/{id}", deleteWatchHandler(watches))
			r.Get("/notifications", listNotificationsHandler(notifications))
			r.Post("/account/resubscribe", resubscribeHandler(accounts))
			r.Delete("/account", deleteAccountHandler(accounts))
		})
	})

	// Serve the embedded SPA for everything else. Unknown /api/* paths still
	// 404 as JSON via the sub-router above; they never reach here.
	if staticFS != nil {
		r.Handle("/*", spaHandler(staticFS))
	}

	return r
}

// spaHandler serves the embedded single-page app. Real files (JS/CSS/assets)
// are served directly; any other path falls back to index.html so client-side
// routing works on deep links and refreshes.
func spaHandler(dist fs.FS) http.HandlerFunc {
	fileServer := http.FileServer(http.FS(dist))
	index, _ := fs.ReadFile(dist, "index.html")
	return func(w http.ResponseWriter, r *http.Request) {
		name := path.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		if name != "." {
			if f, err := dist.Open(name); err == nil {
				f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		if index == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- auth ---

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func registerHandler(a *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req authRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		token, err := a.Register(r.Context(), req.Email, req.Password)
		if err != nil {
			handleAuthError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"token": token})
	}
}

func loginHandler(a *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req authRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		token, err := a.Login(r.Context(), req.Email, req.Password)
		if err != nil {
			handleAuthError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"token": token})
	}
}

func handleAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrEmailTaken):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, service.ErrWeakPassword), errors.Is(err, service.ErrInvalidEmail),
		errors.Is(err, service.ErrInvalidToken), errors.Is(err, service.ErrAlreadyVerified):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		log.Printf("auth: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

// meHandler reports the caller's own account state (drives the verify banner).
func meHandler(a *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserID(r.Context())
		me, err := a.Me(r.Context(), userID)
		if err != nil {
			// Token outlived its account (deleted) -> 401 so the client clears the
			// stale token and returns to login, rather than reporting a server fault.
			if errors.Is(err, service.ErrNoAccount) {
				writeError(w, http.StatusUnauthorized, err.Error())
				return
			}
			log.Printf("me: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, me)
	}
}

// verifyEmailHandler consumes the emailed link. It redirects into the SPA rather
// than returning JSON, because this URL is opened directly in a browser from an
// inbox — the user should land on the app, not on a wall of text.
func verifyEmailHandler(a *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dest := "/?verified=1"
		if err := a.VerifyEmail(r.Context(), r.URL.Query().Get("token")); err != nil {
			if !errors.Is(err, service.ErrInvalidToken) {
				log.Printf("verify email: %v", err)
			}
			dest = "/?verify_error=1"
		}
		http.Redirect(w, r, dest, http.StatusSeeOther)
	}
}

func resendVerificationHandler(a *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserID(r.Context())
		if err := a.ResendVerification(r.Context(), userID); err != nil {
			handleAuthError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type forgotRequest struct {
	Email string `json:"email"`
}

// forgotPasswordHandler always answers 204, whether or not the address has an
// account: distinguishing the two would let anyone test which emails are registered.
func forgotPasswordHandler(a *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req forgotRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if err := a.RequestPasswordReset(r.Context(), req.Email); err != nil {
			// Log it, but still report success — see above.
			log.Printf("password reset request: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type resetRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func resetPasswordHandler(a *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req resetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if err := a.ResetPassword(r.Context(), req.Token, req.Password); err != nil {
			handleAuthError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// --- account: opt-out and erasure (Tier 3) ---

// unsubscribeHandler opts the token's owner out of all alert email.
//
// oneClick distinguishes the two callers. A human clicking the footer link (GET)
// should land on a page telling them it worked; Gmail's own unsubscribe button
// (POST, RFC 8058) is a machine that wants a bare 200 and would render an HTML
// page nowhere.
func unsubscribeHandler(accounts *service.AccountService, oneClick bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := accounts.Unsubscribe(r.Context(), r.URL.Query().Get("token"))

		if oneClick {
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid unsubscribe link")
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}

		// Self-contained HTML: the reader arrives from an email client and is
		// probably not logged in, so bouncing them into the SPA would land them
		// on a login screen and leave them unsure whether it worked.
		status, body := http.StatusOK, unsubscribedPage
		if err != nil {
			status, body = http.StatusBadRequest, unsubscribeFailedPage
			if !errors.Is(err, auth.ErrInvalidUnsubscribeToken) {
				log.Printf("unsubscribe: %v", err)
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

func resubscribeHandler(accounts *service.AccountService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserID(r.Context())
		if err := accounts.Resubscribe(r.Context(), userID); err != nil {
			log.Printf("resubscribe: %v", err)
			writeError(w, http.StatusInternalServerError, "could not re-enable alerts")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteAccountHandler(accounts *service.AccountService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserID(r.Context())
		if err := accounts.Delete(r.Context(), userID); err != nil {
			if errors.Is(err, service.ErrAccountNotFound) {
				writeError(w, http.StatusNotFound, "account not found")
				return
			}
			log.Printf("delete account: %v", err)
			writeError(w, http.StatusInternalServerError, "could not delete account")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// --- search ---

func searchHandler(search *service.SearchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		results, err := search.Search(r.Context(), r.URL.Query().Get("q"))
		if err != nil {
			if errors.Is(err, service.ErrEmptyQuery) {
				writeError(w, http.StatusBadRequest, "query parameter 'q' is required")
				return
			}
			log.Printf("search failed: %v", err)
			writeError(w, http.StatusBadGateway, "upstream search failed")
			return
		}
		writeJSON(w, http.StatusOK, results)
	}
}

// --- watches ---

type createWatchRequest struct {
	TMEventID     string   `json:"tm_event_id"`
	ConditionType string   `json:"condition_type"`
	Threshold     *float64 `json:"threshold"`
	PollIntervalS int32    `json:"poll_interval_s"`
}

func createWatchHandler(watches *service.WatchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserID(r.Context())
		var req createWatchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		view, err := watches.Create(r.Context(), userID, service.CreateWatchInput{
			TMEventID:     req.TMEventID,
			ConditionType: req.ConditionType,
			Threshold:     req.Threshold,
			PollIntervalS: req.PollIntervalS,
		})
		if err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidCondition),
				errors.Is(err, service.ErrThresholdRequired),
				errors.Is(err, service.ErrMissingEventID):
				writeError(w, http.StatusBadRequest, err.Error())
			case errors.Is(err, service.ErrWatchLimitReached):
				// The request is well-formed and authenticated; the account simply
				// isn't allowed more watches.
				writeError(w, http.StatusForbidden, err.Error())
			default:
				log.Printf("create watch: %v", err)
				writeError(w, http.StatusBadGateway, "could not create watch (event lookup failed?)")
			}
			return
		}
		writeJSON(w, http.StatusCreated, view)
	}
}

func listWatchesHandler(watches *service.WatchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserID(r.Context())
		views, err := watches.List(r.Context(), userID)
		if err != nil {
			log.Printf("list watches: %v", err)
			writeError(w, http.StatusInternalServerError, "could not list watches")
			return
		}
		writeJSON(w, http.StatusOK, views)
	}
}

func watchHistoryHandler(watches *service.WatchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserID(r.Context())
		id, err := watchIDParam(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid watch id")
			return
		}
		hist, err := watches.History(r.Context(), userID, id)
		if err != nil {
			handleWatchError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, hist)
	}
}

type updateWatchRequest struct {
	Threshold *float64 `json:"threshold"`
	Status    *string  `json:"status"`
}

func updateWatchHandler(watches *service.WatchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserID(r.Context())
		id, err := watchIDParam(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid watch id")
			return
		}
		var req updateWatchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		view, err := watches.Update(r.Context(), userID, id, service.UpdateWatchInput{Threshold: req.Threshold, Status: req.Status})
		if err != nil {
			if errors.Is(err, service.ErrInvalidStatus) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			handleWatchError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, view)
	}
}

func deleteWatchHandler(watches *service.WatchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserID(r.Context())
		id, err := watchIDParam(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid watch id")
			return
		}
		if err := watches.Delete(r.Context(), userID, id); err != nil {
			handleWatchError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func listNotificationsHandler(notifications *service.NotificationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserID(r.Context())
		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil {
				limit = n
			}
		}
		views, err := notifications.List(r.Context(), userID, int32(limit))
		if err != nil {
			log.Printf("list notifications: %v", err)
			writeError(w, http.StatusInternalServerError, "could not list notifications")
			return
		}
		writeJSON(w, http.StatusOK, views)
	}
}

func watchIDParam(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

func handleWatchError(w http.ResponseWriter, err error) {
	if errors.Is(err, service.ErrWatchNotFound) {
		writeError(w, http.StatusNotFound, "watch not found")
		return
	}
	log.Printf("watch handler: %v", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
