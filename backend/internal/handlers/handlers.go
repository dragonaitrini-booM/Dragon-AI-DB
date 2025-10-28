package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"ai-database-app/internal/ai"
	"ai-database-app/internal/auth"
	"ai-database-app/internal/database"
	"github.com/gorilla/mux"
)

type Handlers struct {
	db     *database.Service
	ai     *ai.Service
	auth   *auth.Service
	logger *slog.Logger
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

func New(db *sql.DB, aiService *ai.Service, authService *auth.Service, logger *slog.Logger) *Handlers {
	dbService := database.NewService(&database.Connection{DB: db}, logger)

	return &Handlers{
		db:     dbService,
		ai:     aiService,
		auth:   authService,
		logger: logger,
	}
}

// Health check endpoint
func (h *Handlers) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "1.0.0",
	}

	h.writeJSON(w, http.StatusOK, response)
}

// Authentication endpoints
func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	var req auth.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	response, err := h.auth.Register(req)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "registration failed", err)
		return
	}

	h.writeJSON(w, http.StatusCreated, response)
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req auth.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	response, err := h.auth.Login(req)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "login failed", err)
		return
	}

	h.writeJSON(w, http.StatusOK, response)
}

// AI endpoints
func (h *Handlers) ExecuteAICommand(w http.ResponseWriter, r *http.Request) {
	var req ai.ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// Get user from context (set by auth middleware)
	userID := r.Context().Value("user_id").(string)
	req.UserID = userID

	response, err := h.ai.ExecuteCommand(r.Context(), req)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "AI command execution failed", err)
		return
	}

	h.writeJSON(w, http.StatusOK, response)
}

func (h *Handlers) ExecuteBatchCommands(w http.ResponseWriter, r *http.Request) {
	var req ai.BatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// Set user ID for all requests
	userID := r.Context().Value("user_id").(string)
	for i := range req.Requests {
		req.Requests[i].UserID = userID
	}

	response, err := h.ai.ExecuteBatch(r.Context(), req)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "batch execution failed", err)
		return
	}

	h.writeJSON(w, http.StatusOK, response)
}

// Database endpoints
func (h *Handlers) ListDatabases(w http.ResponseWriter, r *http.Request) {
	databases, err := h.db.ListDatabases()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to list databases", err)
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"databases": databases,
	})
}

func (h *Handlers) ListTables(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database := vars["database"]

	tables, err := h.db.ListTables(database)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to list tables", err)
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"tables": tables,
	})
}

func (h *Handlers) GetTableSchema(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	table := vars["table"]

	schema, err := h.db.GetTableSchema(table)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to get table schema", err)
		return
	}

	h.writeJSON(w, http.StatusOK, schema)
}

func (h *Handlers) ExecuteQuery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// Apply default limit
	if req.Limit == 0 {
		req.Limit = 1000
	}

	// Add LIMIT to query if not present
	query := req.Query
	if req.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", req.Limit)
	}

	result, err := h.db.ExecuteQuery(query)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "query execution failed", err)
		return
	}

	h.writeJSON(w, http.StatusOK, result)
}

// Export endpoints
func (h *Handlers) ExportPDF(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title     string           `json:"title"`
		ChartSpec map[string]any   `json:"chart_spec,omitempty"`
		TableData []map[string]any `json:"table_data,omitempty"`
		Insights  []string         `json:"insights,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// Generate PDF (implementation depends on your PDF library)
	pdfData, err := h.generatePDF(req.Title, req.ChartSpec, req.TableData, req.Insights)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "PDF generation failed", err)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=report.pdf")
	w.Write(pdfData)
}

func (h *Handlers) ExportExcel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TableData []map[string]any `json:"table_data"`
		SheetName string           `json:"sheet_name,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if req.SheetName == "" {
		req.SheetName = "Data"
	}

	// Generate Excel file (implementation depends on your Excel library)
	excelData, err := h.generateExcel(req.TableData, req.SheetName)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Excel generation failed", err)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=data.xlsx")
	w.Write(excelData)
}

func (h *Handlers) ExportCSV(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TableData []map[string]any `json:"table_data"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// Generate CSV
	csvData, err := h.generateCSV(req.TableData)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "CSV generation failed", err)
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=data.csv")
	w.Write(csvData)
}

// Dashboard endpoints (placeholder implementations)
func (h *Handlers) ListDashboards(w http.ResponseWriter, r *http.Request) {
	// Implementation for listing user dashboards
	h.writeJSON(w, http.StatusOK, map[string]any{
		"dashboards": []any{},
	})
}

func (h *Handlers) CreateDashboard(w http.ResponseWriter, r *http.Request) {
	// Implementation for creating dashboards
	h.writeJSON(w, http.StatusCreated, map[string]any{
		"message": "Dashboard created successfully",
	})
}

func (h *Handlers) GetDashboard(w http.ResponseWriter, r *http.Request) {
	// Implementation for getting specific dashboard
	vars := mux.Vars(r)
	id := vars["id"]

	h.writeJSON(w, http.StatusOK, map[string]any{
		"id":        id,
		"dashboard": map[string]any{},
	})
}

func (h *Handlers) UpdateDashboard(w http.ResponseWriter, r *http.Request) {
	// Implementation for updating dashboards
	h.writeJSON(w, http.StatusOK, map[string]any{
		"message": "Dashboard updated successfully",
	})
}

func (h *Handlers) DeleteDashboard(w http.ResponseWriter, r *http.Request) {
	// Implementation for deleting dashboards
	h.writeJSON(w, http.StatusOK, map[string]any{
		"message": "Dashboard deleted successfully",
	})
}

// Helper methods
func (h *Handlers) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode JSON response", slog.String("error", err.Error()))
	}
}

func (h *Handlers) writeError(w http.ResponseWriter, status int, message string, err error) {
	h.logger.Error(message, slog.String("error", err.Error()))

	errorResp := ErrorResponse{
		Error:   message,
		Details: err.Error(),
	}

	h.writeJSON(w, status, errorResp)
}

// Placeholder implementations for export functions
func (h *Handlers) generatePDF(title string, chartSpec map[string]any, tableData []map[string]any, insights []string) ([]byte, error) {
	// Implement PDF generation using a library like gofpdf or wkhtmltopdf
	return nil, fmt.Errorf("PDF generation not implemented")
}

func (h *Handlers) generateExcel(tableData []map[string]any, sheetName string) ([]byte, error) {
	// Implement Excel generation using a library like excelize
	return nil, fmt.Errorf("Excel generation not implemented")
}

func (h *Handlers) generateCSV(tableData []map[string]any) ([]byte, error) {
	// Implement CSV generation
	return nil, fmt.Errorf("CSV generation not implemented")
}
