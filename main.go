package main

import (
	"embed"
	"fmt"
	"github.com/eissar/eagle-go"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const BASE_URL = "http://127.0.0.1:41595"

var PageSize = 20 // 20 is default

type GalleryData struct {
	Items      []*eagle.ListItem
	Page       int // offset = Limit * Page
	AllTags    []string
	AllFolders []string
	Filter     eagle.ItemListOptions
}

// thumbnailHandler serves a thumbnail image for a given item ID.
// The route is registered as "/img/" and the item ID is extracted from the URL path.
func thumbnailHandler(w http.ResponseWriter, r *http.Request) {
	// Expected path: /img/{itemId}
	// Trim the leading "/img/" to get the item ID.
	itemId := strings.TrimPrefix(r.URL.Path, "/img/")
	if itemId == "" {
		http.Error(w, "missing itemId", http.StatusBadRequest)
		return
	}
	resFlag := r.URL.Query().Get("fq") // full quality flag

	getThumbnail := func() (string, error) {
		if resFlag == "true" {
			return GetEagleThumbnailFullRes(itemId)
		}
		return GetEagleThumbnail(itemId)
	}

	thumbnail, err := getThumbnail()
	if err != nil {
		res := fmt.Sprintf("get thumbnail path=%s err=%s", r.URL.Path, err.Error())
		http.Error(w, res, http.StatusBadRequest)
		return
	}
	// Serve the file directly.
	http.ServeFile(w, r, thumbnail)
}

// galleryHandler renders the gallery page with a list of items.
func galleryHandler(w http.ResponseWriter, r *http.Request) {
	filter := eagle.ItemListOptions{
		Limit:   PageSize,
		Offset:  0,
		OrderBy: "CREATEDATE",
		Keyword: r.URL.Query().Get("keyword"),
		Ext:     "",
		Tags:    r.URL.Query().Get("tags"),
		Folders: "",
	}
	items, fetchErr := eagle.ItemList(BASE_URL, filter)
	if fetchErr != nil {
		http.Error(w, fetchErr.Error(), http.StatusInternalServerError)
		return
	}

	folders, fetchErr := eagle.FolderList(BASE_URL)
	if fetchErr != nil {
		http.Error(w, fetchErr.Error(), http.StatusInternalServerError)
		return
	}
	folderNames := make([]string, len(folders))
	for i, f := range folders {
		folderNames[i] = f.Name
	}

	tags, fetchErr := eagle.TagList(BASE_URL)
	if fetchErr != nil {
		http.Error(w, fetchErr.Error(), http.StatusInternalServerError)
		return
	}
	tagNames := make([]string, len(tags))
	for i, t := range tags {
		tagNames[i] = t.Name
	}

	// first draw
	renderErr := galleryTempl.Execute(w, GalleryData{Items: items, Page: 0, AllTags: tagNames, AllFolders: folderNames, Filter: filter})
	if renderErr != nil {
		fmt.Printf("renderErr: %v\n", renderErr)
		http.Error(w, "failed to render template", http.StatusInternalServerError)
		return
	}
}

// itemsHandler returns a paginated list of items.
func itemsHandler(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("offset")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 0 {
		page = 0
	}

	filter := eagle.ItemListOptions{
		Limit:   PageSize,
		Offset:  page,
		OrderBy: "CREATEDATE",
		Keyword: r.URL.Query().Get("keyword"),
		Ext:     "",
		Tags:    r.URL.Query().Get("tags"),
		Folders: "",
	}

	items, fetchErr := eagle.ItemList(BASE_URL, filter)
	if fetchErr != nil {
		http.Error(w, fetchErr.Error(), http.StatusInternalServerError)
		return
	}

	// folders are needed for navigation UI.
	folders, fetchErr := eagle.FolderList(BASE_URL)
	if fetchErr != nil {
		http.Error(w, fetchErr.Error(), http.StatusInternalServerError)
		return
	}
	folderNames := make([]string, len(folders))
	for i, f := range folders {
		folderNames[i] = f.Name
	}

	// page += 1 // increment for next page indicator // instead, increment in the template.

	renderErr := itemsTempl.Execute(w, GalleryData{Items: items, Page: page, AllTags: nil, AllFolders: nil, Filter: filter})
	if renderErr != nil {
		fmt.Printf("renderErr: %v\n", renderErr)
		http.Error(w, "failed to render template", http.StatusInternalServerError)
		return
	}
}

// GetEagleThumbnailFullRes returns the highest‑resolution thumbnail for the given item.
func GetEagleThumbnailFullRes(itemId string) (string, error) {
	thumbnail, err := GetEagleThumbnail(itemId)
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

func GetEagleThumbnail(itemId string) (string, error) {
	thumbnail, err := eagle.ItemThumbnail(BASE_URL, itemId)
	if err != nil {
		return "", fmt.Errorf("getEagleThumbnail: err=%w", err)
	}

	thumbnail, err = url.PathUnescape(thumbnail)
	if err != nil {
		return thumbnail, fmt.Errorf("getEagleThumbnail: error cleaning thumbnail path: %s:", err.Error())
	}

	if _, err = os.Stat(thumbnail); err != nil {
		return thumbnail, fmt.Errorf("getEagleThumbnail: err=%w", err)
	}

	return thumbnail, nil
}

var allowed_filetypes = []string{".jpeg", ".jpg", ".png", ".gif", ".svg", ".webp", ".avif"}

// resolveThumbnailPath attempts to locate the full‑resolution version of a thumbnail.
func resolveThumbnailPath(thumbnail string) (string, error) {
	thumbnail, err := url.PathUnescape(thumbnail)
	if err != nil {
		return thumbnail, fmt.Errorf("resolvethumb: error cleaning thumbnail path: %s:", err.Error())
	}

	if !strings.HasSuffix(thumbnail, "_thumbnail.png") {
		// Already a full‑resolution file.
		return thumbnail, nil
	}

	// Strip the "_thumbnail.png" suffix and try known extensions.
	thumbnailRoot := strings.TrimSuffix(thumbnail, "_thumbnail.png")
	for _, typ := range allowed_filetypes {
		candidate := thumbnailRoot + typ
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// Fallback to the original thumbnail path.
	return thumbnail, nil
}

var galleryTempl *template.Template
var itemsTempl *template.Template

// Helper functions for the template.
var tmplFuncs = template.FuncMap{
	"join":   strings.Join,
	"lower":  strings.ToLower,
	"printf": fmt.Sprintf,
	"add":    func(a, b int) int { return a + b },
}

//go:embed gallery.gohtml
var tmplFS embed.FS

func main() {
	// Parse embedded templates.
	galleryTempl = template.Must(template.New("gallery").Funcs(tmplFuncs).ParseFS(tmplFS, "gallery.gohtml"))
	itemsTempl = template.Must(template.New("items").Funcs(tmplFuncs).ParseFS(tmplFS, "gallery.gohtml"))

	// Register routes using the net/http default ServeMux.
	http.HandleFunc("/gallery", galleryHandler)
	http.HandleFunc("/img/", thumbnailHandler) // trailing slash to capture itemId
	http.HandleFunc("/items", itemsHandler)

	addr := ":8081"
	fmt.Printf("Starting server at %s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Printf("Server failed: %v\n", err)
	}
}
