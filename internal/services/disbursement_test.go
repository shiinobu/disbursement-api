package services

import (
	"errors"
	"testing"

	"rest-api-disbursement-system/internal/models"
)

type stubDisbursementRepo struct {
	findAllPage   int
	findAllLimit  int
	findAllSearch string
	findAllStatus string

	findAllResult []models.Disbursement
	findAllTotal  int64

	findAllErr error
	exportErr  error

	updatePendingStatusCalled bool
	updatePendingStatusID     uint
	updatePendingStatusUserID uint
	updatePendingStatus       string
	updatePendingStatusReason *string
	updatePendingStatusResult bool
	updatePendingStatusErr    error

	detailResult *models.Disbursement
	detailErr    error
}

func (s *stubDisbursementRepo) Create(disbursement *models.Disbursement) error {
	return nil
}

func (s *stubDisbursementRepo) FindAll(page, limit int, search, status string) ([]models.Disbursement, int64, error) {
	s.findAllPage = page
	s.findAllLimit = limit
	s.findAllSearch = search
	s.findAllStatus = status

	return s.findAllResult, s.findAllTotal, s.findAllErr
}

func (s *stubDisbursementRepo) FindByID(id uint) (*models.Disbursement, error) {
	if s.detailErr != nil {
		return nil, s.detailErr
	}
	return s.detailResult, nil
}

func (s *stubDisbursementRepo) Update(disbursement *models.Disbursement) error {
	return errors.New("not implemented")
}

func (s *stubDisbursementRepo) UpdatePendingStatus(id, userID uint, status string, rejectionReason *string) (bool, error) {
	s.updatePendingStatusCalled = true
	s.updatePendingStatusID = id
	s.updatePendingStatusUserID = userID
	s.updatePendingStatus = status
	s.updatePendingStatusReason = rejectionReason

	return s.updatePendingStatusResult, s.updatePendingStatusErr
}

func (s *stubDisbursementRepo) Delete(id uint) error {
	return errors.New("not implemented")
}

func (s *stubDisbursementRepo) Export(status string) ([]models.Disbursement, error) {
	return nil, s.exportErr
}

func TestDisbursementServiceListFiltersByStatus(t *testing.T) {
	repo := &stubDisbursementRepo{
		findAllResult: []models.Disbursement{{ID: 1, Status: models.StatusPending}},
		findAllTotal:  1,
	}

	service := NewDisbursementService(repo)

	result, err := service.List(2, 20, "budi", string(models.StatusPending))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.findAllPage != 2 || repo.findAllLimit != 20 || repo.findAllSearch != "budi" || repo.findAllStatus != string(models.StatusPending) {
		t.Fatalf("unexpected repository args: %+v", repo)
	}

	if result.Total != 1 || len(result.Data) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestDisbursementServiceListRejectsInvalidPagination(t *testing.T) {
	repo := &stubDisbursementRepo{}
	service := NewDisbursementService(repo)

	for _, test := range []struct {
		name  string
		page  int
		limit int
	}{
		{name: "zero page", page: 0, limit: 10},
		{name: "negative page", page: -1, limit: 10},
		{name: "zero limit", page: 1, limit: 0},
		{name: "negative limit", page: 1, limit: -10},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.List(test.page, test.limit, "", "")
			if !errors.Is(err, ErrInvalidPagination) {
				t.Fatalf("expected ErrInvalidPagination, got %v", err)
			}
		})
	}
}

func TestDisbursementServiceListRejectsInvalidStatus(t *testing.T) {
	repo := &stubDisbursementRepo{}
	service := NewDisbursementService(repo)

	_, err := service.List(1, 10, "", "INVALID")
	if !errors.Is(err, ErrInvalidDisbursementStatus) {
		t.Fatalf("expected ErrInvalidDisbursementStatus, got %v", err)
	}
}

func TestDisbursementServiceExportRejectsInvalidStatus(t *testing.T) {
	repo := &stubDisbursementRepo{}
	service := NewDisbursementService(repo)

	_, err := service.Export("INVALID")
	if !errors.Is(err, ErrInvalidDisbursementStatus) {
		t.Fatalf("expected ErrInvalidDisbursementStatus, got %v", err)
	}
}

func TestDisbursementServiceUpdateStatusUsesAtomicPendingTransition(t *testing.T) {
	repo := &stubDisbursementRepo{
		updatePendingStatusResult: true,
		detailResult: &models.Disbursement{
			ID:     10,
			Status: models.StatusApproved,
		},
	}

	service := NewDisbursementService(repo)

	result, err := service.UpdateStatus(10, 7, "ADMIN", string(models.StatusApproved), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !repo.updatePendingStatusCalled {
		t.Fatal("expected UpdatePendingStatus to be called")
	}
	if repo.updatePendingStatusID != 10 || repo.updatePendingStatusUserID != 7 {
		t.Fatalf("unexpected update identifiers: id=%d userID=%d", repo.updatePendingStatusID, repo.updatePendingStatusUserID)
	}
	if repo.updatePendingStatus != string(models.StatusApproved) {
		t.Fatalf("status = %q, want APPROVED", repo.updatePendingStatus)
	}
	if repo.updatePendingStatusReason != nil {
		t.Fatal("expected nil rejection reason for approval")
	}
	if result.Status != models.StatusApproved {
		t.Fatalf("result status = %q, want APPROVED", result.Status)
	}
}
