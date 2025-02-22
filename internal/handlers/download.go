package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Possible file sizes in bytes
var allowedSizes = map[int]int64{
	5:    5 * 1024 * 1024,
	10:   10 * 1024 * 1024,
	20:   20 * 1024 * 1024,
	50:   50 * 1024 * 1024,
	100:  100 * 1024 * 1024,
	200:  200 * 1024 * 1024,
	500:  500 * 1024 * 1024,
	1000: 1000 * 1024 * 1024,
}

// Session stores information about a particular test session
type Session struct {
	FilePath          string
	ExpectedHash      string
	HashAlgorithm     string
	FileSize          int64
	CreatedAt         time.Time
	DownloadSpeedMbps float64
}

type DownloadHandler struct {
	sessions      map[string]*Session
	mu            sync.Mutex
	lastAccessMap map[string]time.Time // Map to track last access time per device
}

func NewDownloadHandler() *DownloadHandler {
	handler := &DownloadHandler{
		sessions:      make(map[string]*Session),
		lastAccessMap: make(map[string]time.Time),
	}
	handler.StartCleanup()
	return handler
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0]) // Return the first IP in the list
	}

	// Fallback to RemoteAddr
	remoteAddr := r.RemoteAddr
	if colonIndex := strings.LastIndex(remoteAddr, ":"); colonIndex != -1 {
		return remoteAddr[:colonIndex] // Strip port if present
	}

	return remoteAddr // Return as-is if no port
}

func (h *DownloadHandler) CheckRateLimit(r *http.Request) bool {
	clientIP := getClientIP(r)
	h.mu.Lock()
	defer h.mu.Unlock()

	lastAccess, exists := h.lastAccessMap[clientIP]
	if exists && time.Since(lastAccess) < 10*time.Second {
		log.Printf("Rate limit exceeded for IP: %s", clientIP)
		return false // Deny access
	}

	// Update access time
	h.lastAccessMap[clientIP] = time.Now()
	log.Printf("Access granted for IP: %s. Updated lastAccessMap: %+v", clientIP, h.lastAccessMap)
	return true // Allow access
}

type DownloadInitRequest struct {
	SizeMB int `json:"size_mb"`
}

type DownloadInitResponse struct {
	SessionID     string `json:"session_id"`
	Size          int64  `json:"size"`
	HashAlgorithm string `json:"hash_algorithm"`
	ExpectedHash  string `json:"expected_hash"`
}

// InitDownload creates a temp file of requested size, computes its hash, and returns session info
func (h *DownloadHandler) InitDownload(w http.ResponseWriter, r *http.Request) {
	if !h.CheckRateLimit(r) {
		http.Error(w, "Rate limit exceeded. Try again later.", http.StatusTooManyRequests)
		return
	}
	var req DownloadInitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	size, ok := allowedSizes[req.SizeMB]
	if !ok {
		http.Error(w, "Invalid size requested. Allowed values: 5,10,20,50,100", http.StatusBadRequest)
		return
	}

	sessionID := uuid.New().String()

	// Generate a temporary file
	filePath := filepath.Join("tmpdata", sessionID+".bin")
	if err := h.generateRandomFile(filePath, size); err != nil {
		log.Printf("Error generating file: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Compute SHA-256 hash of the file
	expectedHash, err := computeFileHash(filePath)
	if err != nil {
		log.Printf("Error hashing file: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.mu.Lock()
	h.sessions[sessionID] = &Session{
		FilePath:      filePath,
		ExpectedHash:  expectedHash,
		HashAlgorithm: "sha256",
		FileSize:      size,
		CreatedAt:     time.Now(),
	}
	h.mu.Unlock()

	resp := DownloadInitResponse{
		SessionID:     sessionID,
		Size:          size,
		HashAlgorithm: "sha256",
		ExpectedHash:  expectedHash,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Error encoding init response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func (h *DownloadHandler) DownloadData(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "session_id is required", http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	sess, exists := h.sessions[sessionID]
	h.mu.Unlock()

	if !exists {
		http.Error(w, "Invalid session_id", http.StatusNotFound)
		return
	}

	f, err := os.Open(sess.FilePath)
	if err != nil {
		log.Printf("Error opening file: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	// Start tracking time
	startTime := time.Now()

	// Serve the file content
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	http.ServeContent(w, r, filepath.Base(sess.FilePath), time.Now(), f)

	// End tracking time
	endTime := time.Now()

	// Calculate download speed
	duration := endTime.Sub(startTime).Seconds()                         // Time in seconds
	speedMbps := (float64(sess.FileSize) * 8) / (duration * 1024 * 1024) // Convert bytes to Mbps

	h.mu.Lock()
	sess.DownloadSpeedMbps = speedMbps // Store speed in session
	h.mu.Unlock()

	log.Printf("Download speed for session %s: %.2f Mbps", sessionID, speedMbps)
}

type DownloadVerifyRequest struct {
	SessionID    string `json:"session_id"`
	ComputedHash string `json:"computed_hash"`
}

type DownloadVerifyResponse struct {
	Status string `json:"status"`
}
type SpeedResponse struct {
	SessionID         string  `json:"session_id"`
	DownloadSpeedMbps float64 `json:"download_speed_mbps"`
}

func (h *DownloadHandler) GetSpeed(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "session_id is required", http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	sess, exists := h.sessions[sessionID]
	h.mu.Unlock()

	if !exists {
		http.Error(w, "Invalid session_id", http.StatusNotFound)
		return
	}

	resp := SpeedResponse{
		SessionID:         sessionID,
		DownloadSpeedMbps: sess.DownloadSpeedMbps, // Use stored speed
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// VerifyDownload checks if the computed hash matches the expected hash. If it does, remove the file.
func (h *DownloadHandler) VerifyDownload(w http.ResponseWriter, r *http.Request) {
	var req DownloadVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	sess, exists := h.sessions[req.SessionID]
	if !exists {
		h.mu.Unlock()
		http.Error(w, "Invalid session_id", http.StatusNotFound)
		return
	}

	expectedHash := sess.ExpectedHash
	filePath := sess.FilePath

	if req.ComputedHash == expectedHash {
		// Attempt to delete the file
		if err := os.Remove(filePath); err != nil {
			log.Printf("Error removing file: %v", err)
			http.Error(w, "File removal failed", http.StatusInternalServerError)
			h.mu.Unlock()
			return
		}

		// Remove session after successful deletion
		delete(h.sessions, req.SessionID)
		h.mu.Unlock()

		resp := DownloadVerifyResponse{Status: "success"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	} else {
		h.mu.Unlock()
		http.Error(w, "Hash mismatch", http.StatusBadRequest)
	}
}

// generateRandomFile creates a file of the given size filled with random bytes
func (h *DownloadHandler) generateRandomFile(path string, size int64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// For simplicity, just write random bytes
	buf := make([]byte, 1024*1024) // 1MB buffer
	totalWritten := int64(0)

	rand.Seed(time.Now().UnixNano())
	for totalWritten < size {
		// If we need less than 1MB to finish, adjust
		remain := size - totalWritten
		toWrite := len(buf)
		if int64(toWrite) > remain {
			toWrite = int(remain)
		}

		_, err := rand.Read(buf[:toWrite])
		if err != nil {
			return err
		}

		n, err := f.Write(buf[:toWrite])
		if err != nil {
			return err
		}

		totalWritten += int64(n)
	}

	return nil
}

// computeFileHash computes the SHA-256 hash of a file
func computeFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (h *DownloadHandler) StartCleanup() {
	go func() {
		ticker := time.NewTicker(1 * time.Minute) // Check every minute
		defer ticker.Stop()

		for range ticker.C {
			h.mu.Lock()
			now := time.Now()
			for sessionID, sess := range h.sessions {
				if now.Sub(sess.CreatedAt) > time.Hour {
					log.Printf("Cleaning up session: %s", sessionID)

					// Delete file
					if err := os.Remove(sess.FilePath); err != nil {
						log.Printf("Failed to delete file %s: %v", sess.FilePath, err)
					}

					// Remove session
					delete(h.sessions, sessionID)
				}
			}
			h.mu.Unlock()
		}
	}()
}
