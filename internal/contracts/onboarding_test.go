package contracts

import (
	"context"
	"errors"
	"testing"
)

type recordingPipeline struct {
	generated bool
	markedPDF string
}

func (p *recordingPipeline) GenerateContract(_ context.Context, _ string, _ string) (string, error) {
	p.generated = true
	return "generated-pdf-ref", nil
}

func (p *recordingPipeline) MarkApprovedContract(_ context.Context, pdf, _, _ string) (string, error) {
	p.markedPDF = pdf
	return "approved-pdf-ref", nil
}

func TestTenantApprovalDecisionAndHandoff(t *testing.T) {
	tests := []struct {
		name      string
		state     TenantState
		wantErr   error
		wantCalls bool
	}{
		{name: "active tenant is approved", state: TenantActive, wantCalls: true},
		{name: "pending tenant waits", state: TenantPending, wantErr: ErrTenantCannotFinalize},
		{name: "suspended tenant is blocked", state: TenantSuspended, wantErr: ErrTenantCannotFinalize},
		{name: "closed tenant is blocked", state: TenantClosed, wantErr: ErrTenantCannotFinalize},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewTenantStore()
			if err := store.Set("tenant-42", tt.state); err != nil {
				t.Fatal(err)
			}
			pipe := &recordingPipeline{}
			workflow := Onboarding{Tenants: store, PDF: pipe}
			result, err := workflow.Finalize(context.Background(), FinalizeRequest{TenantID: "tenant-42", CompanyName: "Acme Data", Plan: "annual"})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if pipe.generated != tt.wantCalls {
				t.Fatalf("generate called = %v, want %v", pipe.generated, tt.wantCalls)
			}
			if tt.wantCalls && (pipe.markedPDF != "generated-pdf-ref" || result.PDF != "approved-pdf-ref") {
				t.Fatalf("handoff = %q, result = %q", pipe.markedPDF, result.PDF)
			}
		})
	}
}
