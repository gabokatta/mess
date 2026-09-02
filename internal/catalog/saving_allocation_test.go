package catalog

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/domain"
)

func TestCreateAndListSavingAllocations(t *testing.T) {
	db := openTestStore(t).DB()
	sept := domain.NewPeriod(2026, time.September)

	if _, err := CreateSavingAllocation(db, SavingAllocation{
		Period: sept, Destination: Cash, Amount: decimal.NewFromInt(50000), Currency: domain.ARS,
	}); err != nil {
		t.Fatalf("CreateSavingAllocation() unexpected error: %v", err)
	}
	if _, err := CreateSavingAllocation(db, SavingAllocation{
		Period: sept, Destination: Invested, Amount: decimal.NewFromInt(100), Currency: domain.USD,
	}); err != nil {
		t.Fatalf("CreateSavingAllocation() unexpected error: %v", err)
	}

	got, err := SavingAllocations(db, sept)
	if err != nil {
		t.Fatalf("SavingAllocations() unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("SavingAllocations() returned %d rows, want 2", len(got))
	}
	if got[0].Destination != Cash || got[0].Currency != domain.ARS {
		t.Errorf("SavingAllocations()[0] = %+v, want the Cash/ARS row", got[0])
	}
	if got[1].Destination != Invested || got[1].Currency != domain.USD {
		t.Errorf("SavingAllocations()[1] = %+v, want the Invested/USD row", got[1])
	}
	if got[0].ID == 0 {
		t.Error("CreateSavingAllocation() should assign a non-zero ID")
	}
}

func TestDeleteSavingAllocationRemovesOnlyThatRow(t *testing.T) {
	db := openTestStore(t).DB()
	sept := domain.NewPeriod(2026, time.September)

	cash, err := CreateSavingAllocation(db, SavingAllocation{
		Period: sept, Destination: Cash, Amount: decimal.NewFromInt(50000), Currency: domain.ARS,
	})
	if err != nil {
		t.Fatalf("CreateSavingAllocation() unexpected error: %v", err)
	}
	if _, err := CreateSavingAllocation(db, SavingAllocation{
		Period: sept, Destination: Invested, Amount: decimal.NewFromInt(100), Currency: domain.USD,
	}); err != nil {
		t.Fatalf("CreateSavingAllocation() unexpected error: %v", err)
	}

	if err := DeleteSavingAllocation(db, cash.ID); err != nil {
		t.Fatalf("DeleteSavingAllocation() unexpected error: %v", err)
	}

	got, err := SavingAllocations(db, sept)
	if err != nil {
		t.Fatalf("SavingAllocations() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Destination != Invested {
		t.Fatalf("SavingAllocations() = %+v, want only the Invested row left", got)
	}
}

func TestSavingAllocationsFiltersByPeriod(t *testing.T) {
	db := openTestStore(t).DB()
	sept := domain.NewPeriod(2026, time.September)
	oct := domain.NewPeriod(2026, time.October)

	if _, err := CreateSavingAllocation(db, SavingAllocation{
		Period: sept, Destination: Cash, Amount: decimal.NewFromInt(1000), Currency: domain.ARS,
	}); err != nil {
		t.Fatalf("CreateSavingAllocation() unexpected error: %v", err)
	}

	got, err := SavingAllocations(db, oct)
	if err != nil {
		t.Fatalf("SavingAllocations() unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("SavingAllocations(oct) = %+v, want none (allocation is in sept)", got)
	}
}

func TestAllSavingAllocationsOrdersByPeriod(t *testing.T) {
	db := openTestStore(t).DB()
	sept := domain.NewPeriod(2026, time.September)
	aug := domain.NewPeriod(2026, time.August)

	if _, err := CreateSavingAllocation(db, SavingAllocation{
		Period: sept, Destination: Cash, Amount: decimal.NewFromInt(1000), Currency: domain.ARS,
	}); err != nil {
		t.Fatalf("CreateSavingAllocation() unexpected error: %v", err)
	}
	if _, err := CreateSavingAllocation(db, SavingAllocation{
		Period: aug, Destination: Cash, Amount: decimal.NewFromInt(2000), Currency: domain.ARS,
	}); err != nil {
		t.Fatalf("CreateSavingAllocation() unexpected error: %v", err)
	}

	got, err := AllSavingAllocations(db)
	if err != nil {
		t.Fatalf("AllSavingAllocations() unexpected error: %v", err)
	}
	if len(got) != 2 || !got[0].Period.Equal(aug) || !got[1].Period.Equal(sept) {
		t.Errorf("AllSavingAllocations() = %+v, want August before September", got)
	}
}
