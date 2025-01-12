package main

import (
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const MAX_UPLOAD_SIZE = 512 * 1024 * 1024 // 512MB

// Progress structure to track the progress of a file upload
type Progress struct {
	TotalSize int64
	BytesRead int64
}

// Write is used to satisfy the io.Writer interface
func (pr *Progress) Write(p []byte) (n int, err error) {
	n = len(p)
	pr.BytesRead += int64(n)
	fmt.Printf("File upload in progress: %d\n", pr.BytesRead) // メッセージ表示
	return
}

// Printメソッドを追加
func (pr *Progress) Print() {
	fmt.Printf("File upload in progress: %d Bytes\n", pr.BytesRead)
}

// Global variable to hold the uploads directory path
var uploadDir string

func init() {
	flag.StringVar(&uploadDir, "uploadDir", "./uploads", "Directory for file uploads")
	flag.Parse()
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("index.html"))
	videos, err := getVideos()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = tmpl.Execute(w, videos)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["file"]
	for _, fileHeader := range files {
		if fileHeader.Size > MAX_UPLOAD_SIZE {
			http.Error(w, fmt.Sprintf("The uploaded file is too big: %s. Please use a file less than 512MB in size", fileHeader.Filename), http.StatusBadRequest)
			return
		}

		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		defer file.Close()

		buff := make([]byte, 512)
		_, err = file.Read(buff)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		filetype := http.DetectContentType(buff)
		if filetype != "video/mp4" && filetype != "image/jpeg" && filetype != "image/png" {
			http.Error(w, "The provided file format is not allowed. Please upload an MP4, JPEG, or PNG file", http.StatusBadRequest)
			return
		}

		_, err = file.Seek(0, io.SeekStart)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = os.MkdirAll(uploadDir, os.ModePerm) // アップロード・ディレクトリに変更
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		f, err := os.Create(fmt.Sprintf("%s/%d%s", uploadDir, time.Now().UnixNano(), filepath.Ext(fileHeader.Filename)))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		defer f.Close()

		pr := &Progress{TotalSize: fileHeader.Size}
		_, err = io.Copy(f, io.TeeReader(file, pr)) // Printメソッド対応
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Video structure to hold video information
type Video struct {
	Name string
	Path string
}

// Function to get list of MP4, JPEG, and PNG files
func getVideos() ([]Video, error) {
	var videos []Video
	err := filepath.Walk(uploadDir, func(path string, info os.FileInfo, err error) error { // ここの検索もアップデート
		if err != nil {
			return err
		}
		if !info.IsDir() && (strings.HasSuffix(info.Name(), ".mp4") || strings.HasSuffix(info.Name(), ".jpg") || strings.HasSuffix(info.Name(), ".png")) {
			videos = append(videos, Video{Name: info.Name(), Path: path})
		}
		return nil
	})
	return videos, err
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", IndexHandler)
	mux.HandleFunc("/upload", uploadHandler)

	fs := http.FileServer(http.Dir(uploadDir)) // ここでも変更
	mux.Handle("/videos/", http.StripPrefix("/videos/", fs))

	if err := http.ListenAndServe(":4500", mux); err != nil {
		log.Fatal(err)
	}
}
