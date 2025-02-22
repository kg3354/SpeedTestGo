#  SpeedTestGo: High-Performance Network Speed Measurement in Go
This project is a **lightweight speed test server** written in **Go**, designed to measure **download and upload speeds** efficiently. It includes **rate limiting**, **SHA-256 verification**, **automatic file cleanup**, and a **Python wrapper** for automation.

---

##  Project Structure
```
speedtest/
│── cmd/
│   └── server/                  # Main server binary
│       └── main.go               # Entry point for the Go server
│── internal/
│   └── handlers/                 # API handlers
│       ├── download.go           # Handles download speed test logic
│── scripts/
│   ├── speedtest_wrapper.py      # Python wrapper (optional automation)
│── tmpdata/                      # Temporary storage for test files
│── go.mod                         # Go dependencies
│── go.sum                         # Go module checksums
│── speedtest-server               # Compiled Go server binary
```

---

## How It Works
1. The server generates a **test file** upon request.
2. The client **downloads the file** and the server measures the **download speed**.
3. The client **verifies the file hash**, ensuring data integrity.
4. Once verified, the file is **deleted**, but the **download speed is cached** for later retrieval.
5. The client can fetch the **speed result** after verification.

---

##  Installation & Setup
### **1️ Install Go Dependencies**
Ensure **Go** is installed (`go 1.19+` recommended).
```bash
git clone https://github.com/your-repo/speedtest.git
cd speedtest
go mod tidy
```

### **2️ Build & Run Server**
```bash
go build -o speedtest-server ./cmd/server
./speedtest-server
```
The server will start at `http://localhost:8080`.

---

##  API Endpoints
### **1️ Initialize a Download Session**
**Creates a test file and returns a session ID.**
```bash
curl -X POST -d '{"size_mb":20}' -H "Content-Type: application/json" http://localhost:8080/download/init
```
#### **Response**
```json
{
  "session_id": "abc12345-6789",
  "size": 20971520,
  "hash_algorithm": "sha256",
  "expected_hash": "607d9b51cb30a184a5b672611592974a..."
}
```

---

### **2️ Download the Test File**
**Downloads the generated test file using the session ID.**
```bash
curl -X GET "http://localhost:8080/download/data?session_id=abc12345-6789" --output downloaded.bin
```

---

### **3️ Verify the File's Integrity**
**Computes the SHA-256 hash locally and verifies it with the server.**
```bash
SHA256=$(shasum -a 256 downloaded.bin | awk '{print $1}')
curl -X POST -d '{"session_id":"abc12345-6789", "computed_hash":"'$SHA256'"}' \
     -H "Content-Type: application/json" http://localhost:8080/download/verify
```
#### **Response**
```json
{
  "status": "success"
}
```
 **File is deleted from the server after verification, but speed is cached.**

---

### **4️ Retrieve Cached Download Speed**
**Fetches the speed result even after file deletion.**
```bash
curl -X GET "http://localhost:8080/download/speed?session_id=abc12345-6789"
```
#### **Response**
```json
{
  "session_id": "abc12345-6789",
  "download_speed_mbps": 5869.59
}
```

---

##  Python Automation (Optional)
A Python wrapper is available in `scripts/speedtest_wrapper.py` to **automate**:
- Session initialization
- File download
- Hash verification
- Speed retrieval

Run it with:
```bash
pip install requests
python scripts/speedtest_wrapper.py
```

---

##  Features & Optimizations
✅ **Rate Limiting** - Requests per IP are limited to **every 10 seconds**.  
✅ **Efficient Storage Cleanup** - Files are **hard deleted** post-verification.  
✅ **SHA-256 Integrity Check** - Ensures **accurate** speed tests.  
✅ **Cached Speed Results** - Speeds remain available after file deletion.  
✅ **Cross-Platform** - Works on **Linux, Mac, Windows**.  

---

##  Roadmap
- [ ] **Upload Speed Testing** 🆙  
- [ ] **Web Dashboard for Visualization** 📊  
- [ ] **Multi-threaded Download Support** 🚀  

---

## License
This project is licensed under **MIT License**.
