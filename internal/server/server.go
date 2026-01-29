/*
 * MIT License
 *
 * Copyright (c) 2026 Nguyen Thanh Phuong
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

package server

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/phuonguno98/unostat/pkg/version"
	"github.com/phuonguno98/unostat/web"
)

const (
	// MaxUploadSize limits file upload size (200MB)
	MaxUploadSize = 200 * 1024 * 1024
)

// Server represents the web visualization server.
type Server struct {
	dataService *CSVDataService
	uploadDir   string
	logger      *slog.Logger
	router      *mux.Router
}

// NewServer creates a new web server.
// It initializes the data service, scans for existing files (without loading content), and sets up routes.
func NewServer(uploadDir string, logger *slog.Logger) (*Server, error) {
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	s := &Server{
		dataService: NewCSVDataService(logger),
		uploadDir:   uploadDir,
		logger:      logger,
		router:      mux.NewRouter(),
	}

	s.scanExistingFiles()
	s.setupRoutes()

	return s, nil
}

// scanExistingFiles scans the upload directory for CSV files and registers them.
// It performs a lightweight registration (metadata only) instead of loading full content
// to minimize startup time and memory usage (lazy loading).
func (s *Server) scanExistingFiles() {
	files, err := os.ReadDir(s.uploadDir)
	if err != nil {
		s.logger.Warn("Failed to read upload directory", "dir", s.uploadDir, "error", err)
		return
	}

	count := 0
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(strings.ToLower(file.Name()), ".csv") {
			continue
		}

		fileName := file.Name()
		// ID is the filename without extension (e.g., MyFile_20230101)
		id := strings.TrimSuffix(fileName, filepath.Ext(fileName))
		path := filepath.Join(s.uploadDir, fileName)

		// Try to extract display name by removing the suffix (timestamp/uuid)
		// Assumption: Suffix starts with last underscore
		displayName := id
		if lastIdx := strings.LastIndex(id, "_"); lastIdx != -1 {
			displayName = id[:lastIdx]
		}
		// If displayName became empty or too short, revert to id
		if displayName == "" {
			displayName = id
		}

		// List files only, do not load content
		s.dataService.RegisterFile(id, displayName, path)
		count++
	}

	if count > 0 {
		s.logger.Info("Scanned existing files", "count", count)
	}
}

// sanitizeFilename removes unsafe characters and ensures ASCII compatible name
func sanitizeFilename(name string) string {
	// Sanitize path components first (security)
	name = filepath.Base(name)

	// Remove extension
	ext := filepath.Ext(name)
	name = strings.TrimSuffix(name, ext)

	// Replace unsafe chars with empty string or safe char
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == ' ' {
			return r
		}
		if r == '_' {
			return r
		}
		// Convert common separators to dash
		if r == '.' || r == ':' {
			return '-'
		}
		return -1
	}, name)

	safe = strings.TrimSpace(safe)
	if safe == "" {
		return "unnamed"
	}
	// Limit length
	if len(safe) > 50 {
		safe = safe[:50]
	}
	return safe
}

func (s *Server) setupRoutes() {
	// Add CORS middleware
	s.router.Use(corsMiddleware)
	// Add logging middleware
	s.router.Use(s.loggingMiddleware)

	s.router.HandleFunc("/", s.handleIndex).Methods("GET")
	s.router.HandleFunc("/api/version", s.handleGetVersion).Methods("GET")
	s.router.HandleFunc("/api/files", s.handleGetFiles).Methods("GET")
	s.router.HandleFunc("/api/files", s.handleDeleteAllFiles).Methods("DELETE")
	s.router.HandleFunc("/api/files/upload", s.handleUploadFile).Methods("POST")
	s.router.HandleFunc("/api/files/{id}", s.handleDeleteFile).Methods("DELETE")
	s.router.HandleFunc("/api/files/{id}/load", s.handleLoadFile).Methods("POST")
	s.router.HandleFunc("/api/files/{id}/metrics", s.handleGetMetrics).Methods("GET")
	s.router.HandleFunc("/api/data/{fileId}/{metric}", s.handleGetData).Methods("GET")

	// Static files from embedded FS
	staticFS, err := fs.Sub(web.Assets, "static")
	if err != nil {
		s.logger.Error("Failed to get static assets", "error", err)
	}
	s.router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", s.staticFileHandler(staticFS)))

	imagesFS, err := fs.Sub(web.Assets, "images")
	if err != nil {
		s.logger.Error("Failed to get images assets", "error", err)
	}
	s.router.PathPrefix("/images/").Handler(http.StripPrefix("/images/", s.staticFileHandler(imagesFS)))
}

// corsMiddleware adds CORS headers
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs HTTP requests
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Debug("HTTP request",
			"method", r.Method,
			"path", r.URL.Path,
			"duration", time.Since(start),
		)
	})
}

// staticFileHandler serves static files with caching
func (s *Server) staticFileHandler(root fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Disable caching for development/updates
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		fileServer.ServeHTTP(w, r)
	})
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// handleIndex serves the main dashboard HTML file.
func (s *Server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	// Read index.html from embedded assets
	indexFile, err := web.Assets.Open("index.html")
	if err != nil {
		s.logger.Error("Failed to open index.html", "error", err)
		http.Error(w, "Internal Server Error: index.html not found", http.StatusInternalServerError)
		return
	}
	defer func() {
		if err := indexFile.Close(); err != nil {
			s.logger.Warn("Failed to close index.html", "error", err)
		}
	}()

	if _, err := io.Copy(w, indexFile); err != nil {
		s.logger.Error("Failed to serve index.html", "error", err)
	}
}

// handleGetFiles returns a list of all loaded CSV files.
func (s *Server) handleGetFiles(w http.ResponseWriter, _ *http.Request) {
	files := s.dataService.GetFiles()
	s.writeJSON(w, files)
}

// handleGetVersion returns version information from the version package.
func (s *Server) handleGetVersion(w http.ResponseWriter, _ *http.Request) {
	versionInfo := map[string]string{
		"version": version.Version,
		"commit":  version.Commit,
		"date":    version.Date,
	}
	s.writeJSON(w, versionInfo)
}

// handleUploadFile handles CSV file uploads.
// It validates the file extension, sanitizes the filename, saves it to disk,
// and loads it into the data service.
func (s *Server) handleUploadFile(w http.ResponseWriter, r *http.Request) {
	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadSize)

	if err := r.ParseMultipartForm(MaxUploadSize); err != nil {
		s.writeError(w, "File too large or invalid form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		s.writeError(w, "Failed to get file from form", http.StatusBadRequest)
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			s.logger.Warn("Failed to close uploaded file", "error", err)
		}
	}()

	// Validate file extension
	if filepath.Ext(header.Filename) != ".csv" {
		s.writeError(w, "Only CSV files are allowed", http.StatusBadRequest)
		return
	}

	// Validate filename (prevent path traversal)
	filename := filepath.Base(header.Filename)
	if filename != header.Filename {
		s.writeError(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	// 1. Sanitize the original filename to be safe for disk/URL
	safeName := sanitizeFilename(filename)

	// 2. Add UUID suffix to ensure uniqueness and prevent overwrites
	// Format: Name_UUID.csv
	// Using a separator '_' to splitting later
	// Note: We used to use timestamp, but collisions are possible in high concurrency
	id := uuid.New().String()
	fileID := fmt.Sprintf("%s_%s", safeName, id)

	// 3. Construct paths
	fileNameOnDisk := fileID + ".csv"
	filePath := filepath.Join(s.uploadDir, fileNameOnDisk)

	dst, err := os.Create(filePath)
	if err != nil {
		s.logger.Error("Failed to create file", "error", err)
		s.writeError(w, "Failed to create file", http.StatusInternalServerError)
		return
	}
	defer func() {
		if err := dst.Close(); err != nil {
			s.logger.Warn("Failed to close destination file", "path", filePath, "error", err)
		}
	}()

	if _, err := io.Copy(dst, file); err != nil {
		if rmErr := os.Remove(filePath); rmErr != nil {
			s.logger.Error("Failed to remove incomplete file", "path", filePath, "error", rmErr)
		}
		s.logger.Error("Failed to save file", "error", err)
		s.writeError(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	// Load with derived Display Name (safeName) and unique ID
	if err := s.dataService.LoadFile(fileID, safeName, filePath); err != nil {
		if rmErr := os.Remove(filePath); rmErr != nil {
			s.logger.Error("Failed to remove invalid loaded file", "path", filePath, "error", rmErr)
		}
		s.logger.Warn("Failed to load CSV", "error", err, "filename", header.Filename)
		s.writeError(w, fmt.Sprintf("Failed to load CSV: %v", err), http.StatusBadRequest)
		return
	}

	csvFile, _ := s.dataService.GetFile(fileID)
	s.logger.Info("File uploaded successfully", "name", header.Filename, "saved_as", fileNameOnDisk, "rows", csvFile.RowCount)
	s.writeJSON(w, csvFile)
}

// handleDeleteFile removes a file from both memory and disk.
func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileID := vars["id"]

	// Remove from memory
	if err := s.dataService.DeleteFile(fileID); err != nil {
		// Even if not found in memory, try detailed cleanup if file exists on disk
		s.logger.Warn("File not found in memory during delete", "id", fileID)
	}

	// Remove from disk
	filePath := filepath.Join(s.uploadDir, fileID+".csv")
	if err := os.Remove(filePath); err != nil {
		if !os.IsNotExist(err) {
			s.logger.Error("Failed to delete file from disk", "path", filePath, "error", err)
			// We don't return error to client if memory delete was successful or if file is already gone
		}
	}

	s.logger.Info("File deleted", "id", fileID)
	w.WriteHeader(http.StatusNoContent)
}

// handleDeleteAllFiles removes all files from memory and disk.
// This is a destructive operation used to reset the system state.
func (s *Server) handleDeleteAllFiles(w http.ResponseWriter, _ *http.Request) {
	// 1. Clear memory
	s.dataService.DeleteAll()

	// 2. Clear disk (uploads directory)
	// We read the directory and remove all .csv files to be safe, rather than deleting the folder itself
	// to preserve the directory permissions/structure.
	entries, err := os.ReadDir(s.uploadDir)
	if err != nil {
		s.logger.Error("Failed to read upload dir for cleaning", "error", err)
		s.writeError(w, "Failed to clean storage", http.StatusInternalServerError)
		return
	}

	deletedCount := 0
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".csv" {
			path := filepath.Join(s.uploadDir, entry.Name())
			if err := os.Remove(path); err != nil {
				s.logger.Error("Failed to delete file", "path", path, "error", err)
			} else {
				deletedCount++
			}
		}
	}

	s.logger.Info("Storage cleared", "deleted_files", deletedCount)
	w.WriteHeader(http.StatusNoContent)
}

// handleLoadFile allows clients to explicitly trigger data loading for a file.
// This is part of the lazy loading mechanism.
func (s *Server) handleLoadFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if err := s.dataService.LoadFileContent(id); err != nil {
		s.logger.Error("Failed to load file content", "id", id, "error", err)
		s.writeError(w, fmt.Sprintf("Failed to load file: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, map[string]string{"status": "ok", "message": "File loaded successfully"})
}

// UploadDir returns the absolute path of the upload directory.
func (s *Server) UploadDir() string {
	return s.uploadDir
}

// handleGetMetrics returns the list of available metrics (columns) for a specific file.
func (s *Server) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileID := vars["id"]

	metrics, err := s.dataService.GetMetricColumns(fileID)
	if err != nil {
		s.writeError(w, err.Error(), http.StatusNotFound)
		return
	}

	s.writeJSON(w, map[string]interface{}{
		"metrics": metrics,
	})
}

// handleGetData returns time series data for a specific metric in a file.
// Supports optional 'from' and 'to' query parameters for time range filtering.
func (s *Server) handleGetData(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileID := vars["fileId"]
	metric := vars["metric"]

	var timeFrom, timeTo *time.Time

	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err == nil {
			timeFrom = &t
		}
	}

	if toStr := r.URL.Query().Get("to"); toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err == nil {
			timeTo = &t
		}
	}

	data, err := s.dataService.GetColumnData(fileID, metric, timeFrom, timeTo)
	if err != nil {
		s.writeError(w, err.Error(), http.StatusNotFound)
		return
	}

	s.writeJSON(w, data)
}

func (s *Server) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Error("Failed to write JSON response", "error", err)
	}
}

func (s *Server) writeError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	}); err != nil {
		s.logger.Error("Failed to write error response", "error", err)
	}
}
