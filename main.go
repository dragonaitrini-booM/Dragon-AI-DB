package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
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

	"golang.org/x/crypto/pbkdf2"
	"github.com/supabase-community/supabase-go"
	storage_go "github.com/supabase-community/storage-go"
)

const (
	maxFileSize   = 32 << 20 // 32 MB
	keyIterations = 100000   // PBKDF2 iterations
	keyLength     = 32       // AES-256
	saltLength    = 16       // Salt length
)

var (
	masterKey      = "TriniSecure2025!" // Replace with env var in production
	supabaseClient *supabase.Client
	mu             sync.Mutex
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
	ID     int    `json:"id"`
	Source string `json:"source"`
	Env    string `json:"envelope"`
	Size   int64  `json:"size"`
	Tag    string `json:"tag"`
}

// Structs for Kimi API communication
type KimiRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type KimiResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

func init() {
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_KEY")

	if supabaseURL == "" || supabaseKey == "" {
		log.Println("Warning: SUPABASE_URL and SUPABASE_KEY are not set. Database features will be disabled.")
	} else {
		var err error
		supabaseClient, err = supabase.NewClient(supabaseURL, supabaseKey, nil)
		if err != nil {
			log.Fatal("Failed to initialize Supabase client:", err)
		}
		log.Println("Supabase client initialized, mi amor!")
	}

	if v := os.Getenv("MASTER_KEY"); v != "" {
		masterKey = v
	}
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
	if supabaseClient == nil {
		http.Error(w, "Database is not configured", http.StatusInternalServerError)
		return
	}
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

	// Seek back to the beginning of the file to re-read for storage upload
	file.Seek(0, 0)

	// Upload file to Supabase Storage
	bucketName := "datacentral-files"
	contentType := "application/octet-stream" // Default content type
	if len(fileData) > 512 {
		contentType = http.DetectContentType(fileData[:512])
	}
	storageClient := storage_go.NewClient(os.Getenv("SUPABASE_URL")+"/storage/v1", os.Getenv("SUPABASE_KEY"), nil)
	_, err = storageClient.UploadFile(bucketName, header.Filename, file, storage_go.FileOptions{ContentType: &contentType})
	if err != nil {
		// Check if the error is a duplicate file error, which we can ignore for this logic
		if !strings.Contains(err.Error(), "The resource already exists") {
			log.Printf("Supabase storage upload error: %v", err)
			http.Error(w, "File storage failed", http.StatusInternalServerError)
			return
		}
	}


	// Insert into Supabase
	insertData := map[string]interface{}{
		"source":   header.Filename,
		"envelope": toJSON(env),
		"size":     len(fileData),
		"tag":      r.FormValue("tag"),
	}
	var inserted []map[string]interface{}
	_, err = supabaseClient.From("records").Insert(insertData, false, "", "", "").ExecuteTo(&inserted)
	if err != nil {
		log.Printf("Supabase insert error: %v", err)
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
	if supabaseClient == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "results": []Record{}, "total": 0})
		return
	}
	var results []Record
	_, err := supabaseClient.From("records").Select("*", "exact", false).ExecuteTo(&results)
	if err != nil {
		log.Printf("Supabase query error: %v", err)
		http.Error(w, "Database query failed", http.StatusInternalServerError)
		return
	}

	var out []map[string]interface{}
	for _, rec := range results {
		var env Envelope
		if err := fromJSON(rec.Env, &env); err != nil {
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

func handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	if supabaseClient == nil {
		http.Error(w, "Database is not configured", http.StatusInternalServerError)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var fileInfo struct {
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&fileInfo); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Delete from Supabase Storage
	bucketName := "datacentral-files"
	storageClient := storage_go.NewClient(os.Getenv("SUPABASE_URL")+"/storage/v1", os.Getenv("SUPABASE_KEY"), nil)
	_, err := storageClient.RemoveFile(bucketName, []string{fileInfo.Filename})
	if err != nil {
		log.Printf("Supabase storage delete error: %v", err)
		// We can continue even if the file doesn't exist in storage, to ensure metadata is cleaned up
	}

	// Delete from Supabase Database
	var results []map[string]interface{}
	_, err = supabaseClient.From("records").Delete("", "").Eq("source", fileInfo.Filename).ExecuteTo(&results)
	if err != nil {
		log.Printf("Supabase database delete error: %v", err)
		http.Error(w, "Database delete failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "File deleted successfully"})
}

func handleAskKimi(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var userInput struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&userInput); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	apiKey := os.Getenv("MOONSHOT_API_KEY")
	if apiKey == "" {
		log.Println("MOONSHOT_API_KEY is not set.")
		http.Error(w, "AI service is not configured", http.StatusInternalServerError)
		return
	}

	kimiReq := KimiRequest{
		Model: "kimi-k2-0905-preview",
		Messages: []Message{
			{Role: "system", Content: "你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。你会为用户提供安全，有帮助，准确的回答。同时，你会拒绝一切涉及恐怖主义，种族歧视，黄色暴力等问题的回答。Moonshot AI 为专有名词，不可翻译成其他语言。"},
			{Role: "user", Content: userInput.Prompt},
		},
		Temperature: 0.6,
	}

	reqBody, err := json.Marshal(kimiReq)
	if err != nil {
		http.Error(w, "Failed to create AI request", http.StatusInternalServerError)
		return
	}

	req, err := http.NewRequest("POST", "https://api.moonshot.cn/v1/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		http.Error(w, "Failed to create AI request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to communicate with AI service", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var kimiResp KimiResponse
	if err := json.NewDecoder(resp.Body).Decode(&kimiResp); err != nil {
		http.Error(w, "Failed to parse AI response", http.StatusInternalServerError)
		return
	}

	if len(kimiResp.Choices) > 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"reply": kimiResp.Choices[0].Message.Content})
	} else {
		http.Error(w, "No response from AI", http.StatusInternalServerError)
	}
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

            <div class="ai-section" style="margin-top: 40px;">
                <h2>Ask Kimi for Insights</h2>
                <textarea id="ai-prompt" placeholder="Ask a question about yuh data..." style="width: 100%; min-height: 80px; padding: 8px; border-radius: 6px; border: 1px solid #e9e9e7;"></textarea>
                <button onclick="askKimi()" style="margin-top: 10px;">Get Insights</button>
                <div id="ai-response" style="margin-top: 20px; padding: 16px; background: #f7f7f5; border-radius: 6px; min-height: 50px;"></div>
            </div>
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
                    html += "<p><strong>" + file.source + "</strong> - " + file.size + " bytes - " + file.processed_at + " (Tag: " + (file.tag || 'None') + ") <button onclick=\"deleteFile('" + file.source + "')\" style=\"margin-left: 10px; background: #333; color: white; border-radius: 4px; padding: 2px 8px;\">Delete</button></p>";
                });
                document.getElementById('fileList').innerHTML = html;
            } catch (error) {
                document.getElementById('fileList').innerHTML = 'Error loading files, mi amor.';
            }
        }

        async function deleteFile(filename) {
            if (!confirm("Are you sure you want to delete " + filename + "?")) {
                return;
            }

            try {
                const response = await fetch('/delete-file', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({ filename: filename }),
                });

                if (!response.ok) {
                    throw new Error('Failed to delete the file.');
                }

                const result = await response.json();
                alert(result.message);
                loadFiles(); // Refresh the list
            } catch (error) {
                alert('An error occurred while deleting the file.');
                console.error('Error deleting file:', error);
            }
        }

        async function askKimi() {
            const prompt = document.getElementById('ai-prompt').value;
            if (!prompt) {
                alert('Please enter a question for Kimi.');
                return;
            }

            const responseDiv = document.getElementById('ai-response');
            responseDiv.innerText = 'Thinking...';

            try {
                const response = await fetch('/ask-kimi', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({ prompt: prompt }),
                });

                if (!response.ok) {
                    throw new Error('Failed to get a response from the AI.');
                }

                const result = await response.json();
                responseDiv.innerText = result.reply;
            } catch (error) {
                responseDiv.innerText = 'An error occurred, mi amor. Please try again.';
                console.error('Error asking Kimi:', error);
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
	http.HandleFunc("/delete-file", handleDeleteFile)
	http.HandleFunc("/ask-kimi", handleAskKimi)
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
