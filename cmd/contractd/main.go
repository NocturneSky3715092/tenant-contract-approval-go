package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"example.com/tenant-contract-signer/internal/contracts"
)

type server struct {
	onboarding *contracts.Onboarding
}

func main() {
	apiKey := requiredEnv("INFRAI_API_KEY")
	s := &server{
		onboarding: &contracts.Onboarding{Tenants: contracts.NewTenantStore(), PDF: contracts.NewClient(apiKey)},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/tenants/{tenant_id}/state", s.setTenantState)
	mux.HandleFunc("POST /contracts/finalize", s.finalizeContract)
	log.Printf("contract service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func requiredEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		log.Fatalf("%s is required", name)
	}
	return value
}

func (s *server) setTenantState(w http.ResponseWriter, r *http.Request) {
	var in struct {
		State contracts.TenantState `json:"state"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	valid := in.State == contracts.TenantPending || in.State == contracts.TenantActive || in.State == contracts.TenantSuspended || in.State == contracts.TenantClosed
	if !valid {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid tenant state"})
		return
	}
	if err := s.onboarding.Tenants.Set(r.PathValue("tenant_id"), in.State); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"tenant_id": r.PathValue("tenant_id"), "state": string(in.State)})
}

func (s *server) finalizeContract(w http.ResponseWriter, r *http.Request) {
	var in contracts.FinalizeRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.TenantID) == "" || strings.TrimSpace(in.CompanyName) == "" || strings.TrimSpace(in.Plan) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tenant_id, company_name, and plan are required"})
		return
	}
	result, err := s.onboarding.Finalize(r.Context(), in)
	if errors.Is(err, contracts.ErrTenantCannotFinalize) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	var apiErr *contracts.APIError
	if errors.As(err, &apiErr) {
		status := apiErr.Status
		if status < 400 || status >= 500 {
			status = http.StatusBadGateway
		}
		writeJSON(w, status, map[string]string{"error": apiErr.Message, "code": apiErr.Code})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "contract pipeline failed"})
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
