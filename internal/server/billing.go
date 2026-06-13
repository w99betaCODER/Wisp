package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/w99betaCODER/Wisp/internal/billing"
	"github.com/w99betaCODER/Wisp/internal/model"
	"github.com/w99betaCODER/Wisp/internal/payment"
	"github.com/w99betaCODER/Wisp/internal/store"
	"github.com/w99betaCODER/Wisp/internal/util"
)

// --- plans ---------------------------------------------------------------

func (s *Server) handleListPlans(w http.ResponseWriter, r *http.Request) {
	plans, err := s.store.ListPlans()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list plans")
		return
	}
	writeJSON(w, http.StatusOK, plans)
}

type createPlanRequest struct {
	Name         string `json:"name"`
	PriceCents   int64  `json:"price_cents"`
	Currency     string `json:"currency"`
	DurationDays int    `json:"duration_days"`
	DataLimit    int64  `json:"data_limit"`
}

func (s *Server) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	var req createPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Name == "" || req.DurationDays <= 0 {
		writeError(w, http.StatusBadRequest, "name and a positive duration_days are required")
		return
	}
	if req.Currency == "" {
		req.Currency = "USD"
	}

	plan := model.Plan{
		ID:           util.NewID(),
		Name:         req.Name,
		PriceCents:   req.PriceCents,
		Currency:     req.Currency,
		DurationDays: req.DurationDays,
		DataLimit:    req.DataLimit,
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.store.CreatePlan(plan); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create plan")
		return
	}
	writeJSON(w, http.StatusCreated, plan)
}

func (s *Server) handleDeletePlan(w http.ResponseWriter, r *http.Request) {
	err := s.store.DeletePlan(r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "plan not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete plan")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- orders --------------------------------------------------------------

func (s *Server) handleListOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := s.store.ListOrders()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list orders")
		return
	}
	writeJSON(w, http.StatusOK, orders)
}

type createOrderRequest struct {
	UserID string `json:"user_id"`
	PlanID string `json:"plan_id"`
}

// handleCreateOrder opens a pending order for a user + plan. In production this
// is where you would call a payment provider to get a checkout URL; the order
// is settled later via the pay endpoint (or a provider webhook).
func (s *Server) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	var req createOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if _, err := s.store.GetUser(req.UserID); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusBadRequest, "user not found")
		return
	}
	plan, err := s.store.GetPlan(req.PlanID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusBadRequest, "plan not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load plan")
		return
	}

	order := model.Order{
		ID:          util.NewID(),
		UserID:      req.UserID,
		PlanID:      req.PlanID,
		AmountCents: plan.PriceCents,
		Currency:    plan.Currency,
		Status:      model.OrderPending,
		Provider:    "manual",
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.store.CreateOrder(order); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create order")
		return
	}
	writeJSON(w, http.StatusCreated, order)
}

// handlePayOrder settles an order and applies its plan to the user. This is the
// manual/admin path; a real payment webhook would do the same after verifying
// the gateway's signature.
func (s *Server) handlePayOrder(w http.ResponseWriter, r *http.Request) {
	order, err := s.store.GetOrder(r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load order")
		return
	}
	if order.Status == model.OrderPaid {
		writeJSON(w, http.StatusOK, order) // idempotent: already settled
		return
	}

	now := time.Now().UTC()
	order.Status = model.OrderPaid
	order.PaidAt = &now
	if err := s.store.UpdateOrder(order); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update order")
		return
	}
	if err := billing.Apply(r.Context(), s.store, s.xray, s.cluster, s.cfg, order); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to apply plan")
		return
	}
	writeJSON(w, http.StatusOK, order)
}

// webhookEvent is the minimal payload a gateway POSTs to settle an order.
// Real gateways send richer bodies; an adapter maps theirs onto this shape.
type webhookEvent struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

// handleWebhook settles an order from a payment-gateway callback. The raw body
// is authenticated with HMAC-SHA256 (X-Wisp-Signature header) against
// WISP_WEBHOOK_SECRET, then — if the event reports success — the order is paid
// and its plan applied via the same billing.Apply path as the manual endpoint.
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if s.cfg.WebhookSecret == "" {
		writeError(w, http.StatusNotFound, "webhook not configured")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}
	if !payment.VerifyHMAC(s.cfg.WebhookSecret, body, r.Header.Get("X-Wisp-Signature")) {
		writeError(w, http.StatusUnauthorized, "invalid signature")
		return
	}

	var ev webhookEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	order, err := s.store.GetOrder(ev.OrderID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load order")
		return
	}

	if ev.Status != "paid" && ev.Status != "success" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}
	if order.Status == model.OrderPaid {
		writeJSON(w, http.StatusOK, order)
		return
	}

	now := time.Now().UTC()
	order.Status = model.OrderPaid
	order.PaidAt = &now
	order.Provider = r.PathValue("provider")
	if err := s.store.UpdateOrder(order); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update order")
		return
	}
	if err := billing.Apply(r.Context(), s.store, s.xray, s.cluster, s.cfg, order); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to apply plan")
		return
	}
	writeJSON(w, http.StatusOK, order)
}
