package handlers

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"rest-api-disbursement-system/internal/middleware"
	"rest-api-disbursement-system/internal/services"
)

type DisbursementHandler struct {
	disbursements services.DisbursementService
}

type createDisbursementRequest struct {
	RecipientName string       `json:"recipient_name" binding:"required,min=2,max=100"`
	AccountNumber string       `json:"account_number" binding:"required,numeric,min=6,max=30"`
	BankCode      string       `json:"bank_code" binding:"required,min=2,max=100"`
	Amount        amountString `json:"amount" binding:"required,numeric,gt0"`
	Note          string       `json:"note" binding:"omitempty,max=255"`
}

type amountString string

func (a *amountString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*a = ""
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*a = amountString(s)
		return nil
	}

	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		*a = amountString(n.String())
		return nil
	}

	return nil
}

type updateStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=APPROVED REJECTED"`
	Note   string `json:"note"`
}

func NewDisbursementHandler(disbursements services.DisbursementService) *DisbursementHandler {
	return &DisbursementHandler{disbursements: disbursements}
}

func (h *DisbursementHandler) MountRoutes(router *gin.RouterGroup) {
	router.GET("", h.List)
	router.POST("", h.Create)
	router.GET("/:id", h.Detail)
	router.PATCH("/:id/status", h.UpdateStatus)
	router.DELETE("/:id", h.Delete)
	router.GET("/export", h.Export)
}

func (h *DisbursementHandler) List(c *gin.Context) {
	page, pageErr := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, limitErr := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.Query("search")
	status := c.Query("status")

	if pageErr != nil || limitErr != nil {
		Error(c, http.StatusBadRequest, "Request tidak valid", gin.H{
			"page":  "page dan limit harus berupa angka positif",
			"limit": "page dan limit harus berupa angka positif",
		})
		return
	}

	result, err := h.disbursements.List(page, limit, search, status)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidDisbursementStatus):
			Error(c, http.StatusBadRequest, "Request tidak valid", gin.H{"status": "status disbursement tidak valid"})
		case errors.Is(err, services.ErrInvalidPagination):
			Error(c, http.StatusBadRequest, "Request tidak valid", gin.H{"page": "page dan limit harus berupa angka positif", "limit": "page dan limit harus berupa angka positif"})
		default:
			Error(c, http.StatusInternalServerError, "Gagal mengambil data disbursement", nil)
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": result.Data,
		"meta": gin.H{
			"page":        result.Page,
			"limit":       result.Limit,
			"total":       result.Total,
			"total_pages": result.TotalPages,
		},
	})
}

func (h *DisbursementHandler) Create(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		Error(c, http.StatusUnauthorized, "Token tidak valid", nil)
		return
	}

	var request createDisbursementRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ValidationError(c, err, request)
		return
	}

	amount, err := strconv.ParseFloat(string(request.Amount), 64)
	if err != nil {
		Error(c, http.StatusBadRequest, "Request tidak valid", gin.H{"amount": "nilai tidak valid"})
		return
	}

	disbursement, err := h.disbursements.Create(services.CreateDisbursementInput{
		RequesterID:   userID,
		RecipientName: request.RecipientName,
		BankCode:      request.BankCode,
		AccountNumber: request.AccountNumber,
		Amount:        amount,
		Note:          request.Note,
	})
	if err != nil {
		switch {
		case errors.Is(err, services.ErrDisbursementAmountTooLow):
			Error(c, http.StatusBadRequest, "Jumlah disbursement harus lebih dari 10.000", nil)
		default:
			Error(c, http.StatusInternalServerError, "Gagal membuat disbursement", nil)
		}
	} else {
		Success(c, http.StatusCreated, "Disbursement berhasil dibuat", disbursement)
	}
}

func (h *DisbursementHandler) Detail(c *gin.Context) {
	id, ok := disbursementID(c)
	if !ok {
		return
	}

	disbursement, err := h.disbursements.Detail(id)
	if err != nil {
		handleDisbursementError(c, err)
		return
	}

	Success(c, http.StatusOK, "Detail disbursement berhasil diambil", disbursement)
}

func (h *DisbursementHandler) UpdateStatus(c *gin.Context) {
	role, ok := currentUserRole(c)
	if !ok {
		Error(c, http.StatusForbidden, "Forbidden", nil)
		return
	}

	userID, ok := currentUserID(c)
	if !ok {
		Error(c, http.StatusUnauthorized, "Token tidak valid", nil)
		return
	}

	id, ok := disbursementID(c)
	if !ok {
		return
	}

	var request updateStatusRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ValidationError(c, err, request)
		return
	}

	disbursement, err := h.disbursements.UpdateStatus(
		id,
		userID,
		role,
		request.Status,
		request.Note,
	)

	if err != nil {
		handleDisbursementError(c, err)
		return
	}

	Success(c, http.StatusOK, "Status disbursement berhasil diperbarui", disbursement)
}

func (h *DisbursementHandler) Delete(c *gin.Context) {
	id, ok := disbursementID(c)
	if !ok {
		return
	}

	err := h.disbursements.Delete(id)
	if err != nil {
		handleDisbursementError(c, err)
		return
	}

	Success(c, http.StatusOK, "Disbursement berhasil dihapus", nil)
}

func (h *DisbursementHandler) Export(c *gin.Context) {
	status := c.Query("status")

	disbursements, err := h.disbursements.Export(status)
	if err != nil {
		handleDisbursementError(c, err)
		return
	}

	filename := fmt.Sprintf(
		"disbursements_%s.csv",
		time.Now().Format("20060102_150405"),
	)

	c.Header("Content-Type", "text/csv")
	c.Header(
		"Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"`, filename),
	)

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	writer.Write([]string{
		"ID",
		"Recipient Name",
		"Account Number",
		"Amount",
		"Status",
		"Created At",
	})

	for _, d := range disbursements {
		writer.Write([]string{
			fmt.Sprintf("%d", d.ID),
			d.RecipientName,
			d.AccountNumber,
			fmt.Sprintf("%.2f", d.Amount),
			string(d.Status),
			d.CreatedAt.Format(time.RFC3339),
		})
	}
}

func currentUserID(c *gin.Context) (uint, bool) {
	value, exists := c.Get(middleware.UserIDKey)
	if !exists {
		return 0, false
	}

	userID, ok := value.(uint)
	return userID, ok
}

func disbursementID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		Error(c, http.StatusBadRequest, "ID disbursement tidak valid", gin.H{"id": "id harus berupa angka positif"})
		return 0, false
	}

	return uint(id), true
}

func currentUserRole(c *gin.Context) (string, bool) {
	value, exists := c.Get(middleware.UserRoleKey)
	if !exists {
		return "", false
	}

	role, ok := value.(string)
	return role, ok
}

func handleDisbursementError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrForbidden):
		Error(c, http.StatusForbidden, "Anda tidak memiliki akses", nil)
	case errors.Is(err, services.ErrDisbursementNotFound):
		Error(c, http.StatusNotFound, "Disbursement tidak ditemukan", gin.H{"id": "disbursement tidak ditemukan"})
	case errors.Is(err, services.ErrDisbursementAlreadyProcessed):
		Error(c, http.StatusConflict, "Disbursement sudah diproses", gin.H{"status": "hanya disbursement pending yang dapat diproses"})
	case errors.Is(err, services.ErrInvalidDisbursementStatus):
		Error(c, http.StatusBadRequest, "Status disbursement tidak valid", gin.H{"status": "status harus APPROVED atau REJECTED"})
	case errors.Is(err, services.ErrDisbursementCannotBeDeleted):
		Error(c, http.StatusBadRequest, "Disbursement tidak dapat dihapus", gin.H{"status": "hanya disbursement dengan status PENDING yang dapat dihapus"})
	default:
		Error(c, http.StatusInternalServerError, "Gagal memproses disbursement", nil)
	}
}
