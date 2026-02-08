package main

import (
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/eissar/eagle-go"
)

var ( // defined in ./assets_dev.go ./assets_prod.go
	galleryTempl *template.Template
	itemsTempl   *template.Template
	detailTempl  *template.Template
)

var BASE_URL = getEnv("EAGLE_URL", "http://127.0.0.1:41595")

func getEnv(key string, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

var VERSION = "v0.0.0"

var PageSize = 20 // 20 is default

type GalleryData struct {
	Items      []*eagle.ListItem
	Page       int // offset = Limit * Page
	AllTags    []string
	AllFolders []eagle.FolderDetailOverview
	Filter     eagle.ItemListOptions
	Version    string
}

// GetEagleThumbnailFullRes returns the highest‑resolution thumbnail for the given item.
func GetEagleThumbnailFullRes(itemID string) (string, error) {
	thumbnail, err := GetEagleThumbnail(itemID)
	if err != nil {
		return thumbnail, fmt.Errorf("getEagleThumbnail: err=%w", err)
	}

	thumbnail, err = resolveThumbnailPath(thumbnail)
	if err != nil {
		return thumbnail, fmt.Errorf("getEagleThumbnail: err=%w", err)
	}

	if _, err = os.Stat(thumbnail); err != nil {
		return thumbnail, fmt.Errorf("getEagleThumbnail: err=%w", err)
	}

	return thumbnail, nil
}

func GetEagleThumbnail(itemID string) (string, error) {
	thumbnail, err := eagle.ItemThumbnail(BASE_URL, itemID)
	if err != nil {
		return "", fmt.Errorf("getEagleThumbnail: err=%w", err)
	}

	thumbnail, err = url.PathUnescape(thumbnail)
	if err != nil {
		return thumbnail, fmt.Errorf("getEagleThumbnail: error cleaning thumbnail path: %s: ", err.Error())
	}

	if _, err = os.Stat(thumbnail); err != nil {
		return thumbnail, fmt.Errorf("getEagleThumbnail: err=%w", err)
	}

	return thumbnail, nil
}

var allowedFiletypes = []string{".jpeg", ".jpg", ".png", ".gif", ".svg", ".webp", ".avif"}

// resolveThumbnailPath attempts to locate the full‑resolution version of a thumbnail.
func resolveThumbnailPath(thumbnail string) (string, error) {
	thumbnail, err := url.PathUnescape(thumbnail)
	if err != nil {
		return thumbnail, fmt.Errorf("resolvethumb: error cleaning thumbnail path: %s: ", err.Error())
	}

	if !strings.HasSuffix(thumbnail, "_thumbnail.png") {
		// Already a full‑resolution file.
		return thumbnail, nil
	}

	// Strip the "_thumbnail.png" suffix and try known extensions.
	thumbnailRoot := strings.TrimSuffix(thumbnail, "_thumbnail.png")
	for _, typ := range allowedFiletypes {
		candidate := thumbnailRoot + typ
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// Fallback to the original thumbnail path.
	return thumbnail, nil
}

// Helper functions for the template.
var tmplFuncs = template.FuncMap{
	"join":   strings.Join,
	"lower":  strings.ToLower,
	"printf": fmt.Sprintf,
	"add":    func(a, b int) int { return a + b },
}

func main() {
	port := flag.String("port", "8081", "port to listen on")
	host := flag.String("host", "127.0.0.1", "host address to bind to")
	flag.Parse()
	// Register routes using the net/http default ServeMux.
	http.HandleFunc("/gallery", galleryHandler)

	http.HandleFunc("/img/", thumbnailHandler) // trailing slash to capture itemId
	http.HandleFunc("/detail/", detailHandler) // trailing slash to capture itemId

	http.HandleFunc("/items", itemsHandler)
	http.HandleFunc("/upload", uploadHandler)

	fmt.Printf("eagle-web version %s\n", VERSION)
	addr := fmt.Sprintf("%s:%s", *host, *port)

	// this is ugly.
	http.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// is_http = false
	// ... if is_http ... (can we just let the client update?)

	fmt.Printf("Starting server at http://%s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Printf("Server failed: %v\n", err)
	}
}
