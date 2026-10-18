package contracts

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type TenantState string

const (
	TenantPending   TenantState = "pending"
	TenantActive    TenantState = "active"
	TenantSuspended TenantState = "suspended"
	TenantClosed    TenantState = "closed"
)

var ErrTenantCannotFinalize = errors.New("tenant is not active")

type PDFPipeline interface {
	GenerateContract(context.Context, string, string) (string, error)
	MarkApprovedContract(context.Context, string, string, string) (string, error)
}

type TenantStore struct {
	mu     sync.RWMutex
	states map[string]TenantState
}

func NewTenantStore() *TenantStore { return &TenantStore{states: make(map[string]TenantState)} }

func (s *TenantStore) Set(tenantID string, state TenantState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current := s.states[tenantID]
	if current == TenantClosed && state != TenantClosed {
		return errors.New("closed tenant cannot transition")
	}
	s.states[tenantID] = state
	return nil
}

func (s *TenantStore) Get(tenantID string) TenantState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.states[tenantID]
}

type Onboarding struct {
	Tenants *TenantStore
	PDF     PDFPipeline
}

type FinalizeRequest struct {
	TenantID    string `json:"tenant_id"`
	CompanyName string `json:"company_name"`
	Plan        string `json:"plan"`
}

type FinalizeResult struct {
	TenantID string `json:"tenant_id"`
	State    string `json:"state"`
	PDF      string `json:"pdf"`
}

func (o *Onboarding) Finalize(ctx context.Context, in FinalizeRequest) (FinalizeResult, error) {
	if o.Tenants.Get(in.TenantID) != TenantActive {
		return FinalizeResult{}, ErrTenantCannotFinalize
	}
	markdown := fmt.Sprintf("# SaaS Agreement\n\nTenant: %s\n\nPlan: %s\n", in.CompanyName, in.Plan)
	generated, err := o.PDF.GenerateContract(ctx, markdown, in.TenantID+":generate")
	if err != nil {
		return FinalizeResult{}, err
	}
	approved, err := o.PDF.MarkApprovedContract(ctx, generated, in.TenantID, in.TenantID+":watermark")
	if err != nil {
		return FinalizeResult{}, err
	}
	return FinalizeResult{TenantID: in.TenantID, State: "approved", PDF: approved}, nil
}
