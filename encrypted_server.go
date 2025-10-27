                                                                                     package main

import (
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"4444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)
const dbFile = "encrypted_records.json"

var (
	masterKey = "local-master-key-please-change"
	mu        sync.Mutex
)

type Envelope struct {
	Algorithm string    `json:"algorithm"`
	KeyID     string    `json:"key_id"`
	Salt      string    `json:"salt"`
	Nonce     string    `json:"nonce"`
	Data      string    `json:"data"`
	Timestamp time.Time `json:"timestamp"`
}

type Record struct {
	Source string `json:"source"`
	Env    string `json:"envelope"`
}

func deriveKey(password string, salt []byte) []byte {
	// Simple KDF for demo only: SHA256(password || salt). For real usage use PBKDF2 or HKDF.
	h := sha256.New()
	h.Write([]byte(password))
	h.Write(salt)
	return h.Sum(nil)
}

func encrypt(plaintext []byte) (Envelope, error) {
	salt := make([]byte, 16)
	if _, err := crand.Read(salt); err != nil {
		return Envelope{}, err
	}
	key := deriveKey(masterKey, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return Envelope{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return Envelope{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := crand.Read(nonce); err != nil {
		return Envelope{}, err
	}
	ct := gcm.Seal(nil, nonce, plaintext, nil)
	env := Envelope{
		Algorithm: "AES-256-GCM",
		KeyID:     "local-001",
		Salt:      hex.EncodeToString(salt),
		Nonce:     hex.EncodeToString(nonce),
		Data:      hex.EncodeToString(ct),
		Timestamp: time.Now().UTC(),
	}
	return env, nil
}

func decrypt(env Envelope) ([]byte, error) {
	salt, err := hex.DecodeString(env.Salt)
	if err != nil {
		return nil, err
	}
	nonce, err := hex.DecodeString(env.Nonce)
	if err != nil {
		return nil, err
	}
	ct, err := hex.DecodeString(env.Data)
	if err != nil {
		return nil, err
	}
	key := deriveKey(masterKey, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, err
	}
	return pt, nil
}

func loadRecords() ([]Record, error) {
	mu.Lock()
	defer mu.Unlock()
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		return []Record{}, nil
	}
	b, err := os.ReadFile(dbFile)
	if err != nil {
		return nil, err
	}
	var recs []Record
	if err := json.Unmarshal(b, &recs); err != nil {
		return nil, err
	}
	return recs, nil
}

func saveRecords(recs []Record) error {
	mu.Lock()
	defer mu.Unlock()
	b, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dbFile, b, 0600)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "ok",
		"timestamp":  time.Now().UTC(),
		"encryption": "AES-256-GCM",
		"key_id":     "local-001",
	})
}

func handleAddSample(w http.ResponseWriter, r *http.Request) {
	// Create a small sample metadata payload and encrypt it
	meta := map[string]interface{}{
		"source":       "financial_sample.csv",
		"size":         115,
		"tag":          "finance",
		"processed_at": time.Now().UTC().Format(time.RFC3339),
	}
	buf, _ := json.Marshal(meta)
	env, err := encrypt(buf)
	if err != nil {
		http.Error(w, "encryption failed", 500)
		return
	}
	recs, err := loadRecords()
	if err != nil {
		http.Error(w, "db load failed", 500)
		return
	}
	recs = append([]Record{{Source: "financial_sample.csv", Env: toJSON(env)}}, recs...)
	if err := saveRecords(recs); err != nil {
		http.Error(w, "db save failed", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "sample added"})
}

func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func fromJSON(s string, v interface{}) error {
	return json.Unmarshal([]byte(s), v)
}

func handleQuery(w http.ResponseWriter, r *http.Request) {
	recs, err := loadRecords()
	if err != nil {
		http.Error(w, "db load failed", 500)
		return
	}
	out := []map[string]interface{}{}
	for _, rec := range recs {
		var env Envelope
		if err := fromJSON(rec.Env, &env); err != nil {
			continue
		}
		pt, err := decrypt(env)
		if err != nil {
			continue
		}
		var meta map[string]interface{}
		if err := json.Unmarshal(pt, &meta); err != nil {
			continue
		}
		row := map[string]interface{}{
			"source":       rec.Source,
			"processed_at": meta["processed_at"],
			"size":         meta["size"],
			"tag":          meta["tag"],
		}
		out = append(out, row)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"results": out})
}

func main() {
	// read MASTER_KEY from env if provided
	if v := os.Getenv("MASTER_KEY"); v != "" {
		masterKey = v
	}
	log.Printf("Starting minimal encrypted server on :9090 (master key set=%t)", masterKey != "")
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/add_sample", handleAddSample)
	http.HandleFunc("/query", handleQuery)
	// simple static page
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, "Minimal Encrypted Server. Endpoints: /health /add_sample /query\n")
	})
	// Start server
	srv := &http.Server{Addr: ":9090"}
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("server exited: %v", err)
		}
	}()
	// Wait until terminated
	// Wait for interrupt signal to gracefully shut down
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}
