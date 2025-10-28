package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/pbkdf2"
)

const (
	dbFile        = "data.db"
	maxFileSize   = 32 << 20 // 32 MB
	keyIterations = 100000   // PBKDF2 iterations
	keyLength     = 32       // AES-256
	saltLength    = 16       // Salt length
)

var (
	masterKey = "TriniSecure2025!" // Replace with env var in production
	db        *sql.DB
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
	ID        int    `json:"id"`
	Source    string `json:"source"`
	Env       string `json:"envelope"`
	Size      int64  `json:"size"`
	Tag       string `json:"tag"`
}

func init() {
	var err error
	db, err = sql.Open("sqlite3", dbFile)
	if err != nil {
		log.Fatal("Database connection failed:", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source TEXT NOT NULL,
			envelope TEXT NOT NULL,
			size INTEGER NOT NULL,
			tag TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_source ON records(source);
	`)
	if err != nil {
		log.Fatal("Database schema creation failed:", err)
	}

	if v := os.Getenv("MASTER_KEY"); v != "" {
		masterKey = v
	}
	log.Println("Database and key initialized, mi amor!")
}

func deriveKey(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, keyIterations, keyLength, sha256.New)
}

func encrypt(plaintext []byte) (Envelope, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return Envelope{}, fmt.Errorf("salt generation failed: %v", err)
	}

	key := deriveKey(masterKey, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return Envelope{}, fmt.Errorf("cipher creation failed: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return Envelope{}, fmt.Errorf("GCM creation failed: %v", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return Envelope{}, fmt.Errorf("nonce generation failed: %v", err)
	}

	ct := gcm.Seal(nil, nonce, plaintext, nil)
	return Envelope{
		Algorithm: "AES-256-GCM",
		KeyID:     "tt-local-001",
		Salt:      hex.EncodeToString(salt),
		Nonce:     hex.EncodeToString(nonce),
		Data:      hex.EncodeToString(ct),
		Timestamp: time.Now().UTC(),
	}, nil
}

func decrypt(env Envelope) ([]byte, error) {
	salt, err := hex.DecodeString(env.Salt)
	if err != nil {
		return nil, fmt.Errorf("salt decode failed: %v", err)
	}

	nonce, err := hex.DecodeString(env.Nonce)
	if err != nil {
		return nil, fmt.Errorf("nonce decode failed: %v", err)
	}

	ct, err := hex.DecodeString(env.Data)
	if err != nil {
		return nil, fmt.Errorf("data decode failed: %v", err)
	}

	key := deriveKey(masterKey, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cipher creation failed: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("GCM creation failed: %v", err)
	}

	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %v", err)
	}
	return pt, nil
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "ok",
		"timestamp":  time.Now().UTC(),
		"encryption": "AES-256-GCM",
		"key_id":     "tt-local-001",
	})
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(maxFileSize); err != nil {
		log.Printf("Form parse error: %v", err)
		http.Error(w, "File too large or invalid form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("File access error: %v", err)
		http.Error(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".csv" && ext != ".xlsx" && ext != ".xls" && ext != ".pdf" && ext != ".txt" {
		http.Error(w, "Unsupported file type", http.StatusBadRequest)
		return
	}

	fileData, err := io.ReadAll(file)
	if err != nil {
		log.Printf("File read error: %v", err)
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	meta := map[string]interface{}{
		"source":       header.Filename,
		"size":         len(fileData),
		"tag":          r.FormValue("tag"),
		"processed_at": time.Now().UTC().Format(time.RFC3339),
	}
	buf, err := json.Marshal(meta)
	if err != nil {
		log.Printf("Metadata marshal error: %v", err)
		http.Error(w, "Metadata processing failed", http.StatusInternalServerError)
		return
	}

	env, err := encrypt(buf)
	if err != nil {
		log.Printf("Encryption error: %v", err)
		http.Error(w, "Encryption failed", http.StatusInternalServerError)
		return
	}

	mu.Lock()
	_, err = db.Exec(
		"INSERT INTO records (source, envelope, size, tag) VALUES (?, ?, ?, ?)",
		header.Filename, toJSON(env), len(fileData), r.FormValue("tag"),
	)
	mu.Unlock()
	if err != nil {
		log.Printf("Database insert error: %v", err)
		http.Error(w, "Database storage failed", http.StatusInternalServerError)
		return
	}

	log.Printf("File metadata encrypted and stored: %s (%d bytes)", header.Filename, len(fileData))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "success",
		"message":   "Yuh file metadata encrypted and stored, mi amor!",
		"filename":  header.Filename,
		"size":      len(fileData),
		"tag":       r.FormValue("tag"),
	})
}

func handleQuery(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`
		SELECT id, source, envelope, size, tag
		FROM records
		ORDER BY created_at DESC
	`)
	if err != nil {
		log.Printf("Query error: %v", err)
		http.Error(w, "Database query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var out []map[string]interface{}
	for rows.Next() {
		var rec Record
		var envStr string
		if err := rows.Scan(&rec.ID, &rec.Source, &envStr, &rec.Size, &rec.Tag); err != nil {
			log.Printf("Row scan error: %v", err)
			continue
		}

		var env Envelope
		if err := fromJSON(envStr, &env); err != nil {
			log.Printf("Envelope parse error: %v", err)
			continue
		}

		pt, err := decrypt(env)
		if err != nil {
			log.Printf("Decryption error: %v", err)
			continue
		}

		var meta map[string]interface{}
		if err := json.Unmarshal(pt, &meta); err != nil {
			log.Printf("Metadata parse error: %v", err)
			continue
		}

		row := map[string]interface{}{
			"id":           rec.ID,
			"source":       rec.Source,
			"size":         rec.Size,
			"tag":          rec.Tag,
			"processed_at": meta["processed_at"],
		}
		out = append(out, row)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"results": out,
		"total":   len(out),
	})
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>DataCentral T&T - Yuh Business Data Hub</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: #fafafa;
            color: #2e3338;
        }
        .container {
            display: flex;
            min-height: 100vh;
        }
        .sidebar {
            width: 224px;
            background: #fbfbfa;
            border-right: 1px solid #e9e9e7;
            padding: 8px;
            position: fixed;
            height: 100vh;
            overflow-y: auto;
        }
        .logo {
            padding: 16px 8px;
            font-weight: 600;
            font-size: 14px;
            color: #c41e3a;
            border-bottom: 1px solid #e9e9e7;
            margin-bottom: 8px;
        }
        .search-bar {
            height: 30px;
            margin-bottom: 16px;
        }
        .search-bar input {
            width: 100%;
            height: 100%;
            border: 1px solid #e9e9e7;
            border-radius: 6px;
            padding: 0 8px;
            font-size: 14px;
            background: #f7f7f5;
        }
        .nav-item {
            display: flex;
            align-items: center;
            padding: 6px 8px;
            border-radius: 4px;
            text-decoration: none;
            color: #2e3338;
            font-size: 14px;
            margin-bottom: 2px;
        }
        .nav-item:hover {
            background: #f1f1ef;
        }
        .main-content {
            margin-left: 224px;
            padding: 24px;
            flex-grow: 1;
        }
        .header {
            background: linear-gradient(135deg, #c41e3a 0%, #000000 100%);
            color: white;
            padding: 24px;
            border-radius: 8px;
            margin-bottom: 24px;
        }
        .upload-area {
            border: 2px dashed #c41e3a;
            padding: 40px;
            text-align: center;
            margin: 20px 0;
            background: rgba(196, 30, 58, 0.1);
        }
        button {
            background: #c41e3a;
            color: white;
            padding: 10px 20px;
            border: none;
            border-radius: 6px;
            cursor: pointer;
        }
        button:hover {
            background: #a1122f;
        }
        .file-list {
            margin-top: 30px;
        }
        @media (max-width: 768px) {
            .sidebar {
                width: 100%;
                height: auto;
                position: static;
            }
            .main-content {
                margin-left: 0;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="sidebar">
            <div class="logo">DataCentral T&T</div>
            <div class="search-bar">
                <input placeholder="Search yuh data...">
            </div>
            <a href="/" class="nav-item">Dashboard</a>
            <a href="#" class="nav-item">Check Yuh Stats</a>
            <a href="#" class="nav-item">Lime with Yuh Team</a>
            <a href="#" class="nav-item">Settings</a>
        </div>
        <div class="main-content">
            <div class="header">
                <h1>Welcome to Yuh Data Hub, mi amor!</h1>
                <p>Securely manage yuh business data with a Trini touch. Upload in TTD!</p>
            </div>
            <div class="upload-area">
                <form id="uploadForm" enctype="multipart/form-data">
                    <input type="file" id="fileInput" name="file" accept=".csv,.xlsx,.xls,.pdf,.txt" required>
                    <br><br>
                    <input type="text" name="tag" placeholder="Tag (e.g., finance, inventory)" style="padding: 8px; margin-bottom: 10px;">
                    <br>
                    <button type="submit">Encrypt & Upload Yuh File</button>
                </form>
            </div>
            <button onclick="loadFiles()">Check Yuh Encrypted Files</button>
            <div id="fileList" class="file-list"></div>
        </div>
    </div>
    <script>
        document.getElementById('uploadForm').onsubmit = async (e) => {
            e.preventDefault();
            const formData = new FormData(e.target);
            try {
                const response = await fetch('/upload', {
                    method: 'POST',
                    body: formData
                });
                const result = await response.json();
                alert(result.message);
                loadFiles();
            } catch (error) {
                alert('Upload failed, mi amor. Try again!');
            }
        };
        async function loadFiles() {
            try {
                const response = await fetch('/query');
                const data = await response.json();
                let html = '<h3>Yuh Encrypted Files (' + data.total + ')</h3>';
                data.results.forEach(file => {
                    html += '<p><strong>' + file.source + '</strong> - ' + file.size + ' bytes - ' + file.processed_at + ' (Tag: ' + (file.tag || 'None') + ')</p>';
                });
                document.getElementById('fileList').innerHTML = html;
            } catch (error) {
                document.getElementById('fileList').innerHTML = 'Error loading files, mi amor.';
            }
        }
    </script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func fromJSON(s string, v interface{}) error {
	return json.Unmarshal([]byte(s), v)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/upload", handleUpload)
	http.HandleFunc("/query", handleQuery)
	http.HandleFunc("/", handleDashboard)

	srv := &http.Server{Addr: ":" + port}
	go func() {
		log.Printf("Starting DataCentral T&T on port %s, mi amor!", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server failed:", err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Println("Shutting down gracefully, mi amor...")
	srv.Close()
}
