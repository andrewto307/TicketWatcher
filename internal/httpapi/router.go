// Package httpapi wires HTTP routes to domain services.
package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"ticket-watcher/internal/service"
)

// NewRouter builds the application's HTTP handler.
func NewRouter(search *service.SearchService, watches *service.WatchService, notifications *service.NotificationService) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", healthHandler)

	r.Route("/api", func(r chi.Router) {
		r.Get("/search", searchHandler(search))
		r.Post("/watches", createWatchHandler(watches))
		r.Get("/watches", listWatchesHandler(watches))
		r.Get("/watches/{id}/history", watchHistoryHandler(watches))
		r.Patch("/watches/{id}", updateWatchHandler(watches))
		r.Delete("/watches/{id}", deleteWatchHandler(watches))
		r.Get("/notifications", listNotificationsHandler(notifications))
	})

	return r
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

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

type createWatchRequest struct {
	TMEventID     string   `json:"tm_event_id"`
	ConditionType string   `json:"condition_type"`
	Threshold     *float64 `json:"threshold"`
	PollIntervalS int32    `json:"poll_interval_s"`
}

func createWatchHandler(watches *service.WatchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createWatchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		view, err := watches.Create(r.Context(), service.CreateWatchInput{
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
		views, err := watches.List(r.Context())
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
		id, err := watchIDParam(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid watch id")
			return
		}
		hist, err := watches.History(r.Context(), id)
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
		view, err := watches.Update(r.Context(), id, service.UpdateWatchInput{Threshold: req.Threshold, Status: req.Status})
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
		id, err := watchIDParam(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid watch id")
			return
		}
		if err := watches.Delete(r.Context(), id); err != nil {
			handleWatchError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func listNotificationsHandler(notifications *service.NotificationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil {
				limit = n
			}
		}
		views, err := notifications.List(r.Context(), int32(limit))
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
