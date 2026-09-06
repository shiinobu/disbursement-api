package repository

import (
	"gorm.io/gorm"

	"rest-api-disbursement-system/internal/models"
)

type DisbursementRepository interface {
	Create(disbursement *models.Disbursement) error
	FindAll(page, limit int, search, status string) ([]models.Disbursement, int64, error)
	FindByID(id uint) (*models.Disbursement, error)
	Update(disbursement *models.Disbursement) error
	UpdatePendingStatus(id, userID uint, status string, rejectionReason *string) (bool, error)
	Delete(id uint) error
	Export(status string) ([]models.Disbursement, error)
}

type disbursementRepository struct {
	db *gorm.DB
}

func NewDisbursementRepository(db *gorm.DB) DisbursementRepository {
	return &disbursementRepository{db: db}
}

func (r *disbursementRepository) Create(disbursement *models.Disbursement) error {
	return r.db.Create(disbursement).Error
}

func (r *disbursementRepository) FindAll(page, limit int, search, status string) ([]models.Disbursement, int64, error) {
	var disbursements []models.Disbursement
	var total int64

	query := r.db.Model(&models.Disbursement{})

	if search != "" {
		searchTerm := "%" + search + "%"
		query = query.Where("recipient_name LIKE ?", searchTerm)
	}

	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit

	err := query.
		Preload("Requester").
		Preload("ProcessedBy").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&disbursements).
		Error

	if err != nil {
		return nil, 0, err
	}

	return disbursements, total, nil
}

func (r *disbursementRepository) FindByID(id uint) (*models.Disbursement, error) {
	var disbursement models.Disbursement
	err := r.db.
		Preload("Requester").
		Preload("ProcessedBy").
		First(&disbursement, id).
		Error
	if err != nil {
		return nil, err
	}
	return &disbursement, nil
}

func (r *disbursementRepository) Update(disbursement *models.Disbursement) error {
	return r.db.Save(disbursement).Error
}

func (r *disbursementRepository) UpdatePendingStatus(id, userID uint, status string, rejectionReason *string) (bool, error) {
	updates := map[string]interface{}{
		"status":          status,
		"processed_by_id": userID,
		"rejection_reason": rejectionReason,
	}

	result := r.db.Model(&models.Disbursement{}).
		Where("id = ? AND status = ?", id, models.StatusPending).
		Updates(updates)

	if result.Error != nil {
		return false, result.Error
	}

	return result.RowsAffected == 1, nil
}

func (r *disbursementRepository) Delete(id uint) error {
	return r.db.Delete(&models.Disbursement{}, id).Error
}

func (r *disbursementRepository) Export(status string) ([]models.Disbursement, error) {
	var disbursements []models.Disbursement
	err := r.db.
		Preload("Requester").
		Preload("ProcessedBy").
		Where("status = ?", status).
		Find(&disbursements).
		Error
	if err != nil {
		return nil, err
	}
	return disbursements, nil
}
