package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/eissar/eagle-go"
)

// thumbnailHandler serves a thumbnail image for a given item ID.
func thumbnailHandler(w http.ResponseWriter, r *http.Request) {
	// Expected path: /img/{itemID}
	// Trim the leading "/img/" to get the item ID.
	itemID := strings.TrimPrefix(r.URL.Path, "/img/")
	if itemID == "" {
		http.Error(w, "missing itemId", http.StatusBadRequest)
		return
	}
	resFlag := r.URL.Query().Get("fq") // full quality flag

	getThumbnail := func() (string, error) {
		if resFlag == "true" {
			return GetEagleThumbnailFullRes(itemID)
		}
		return GetEagleThumbnail(itemID)
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

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(32 * 1024 * 1024) // 32MB
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse multipart form Error:\n %v", err), http.StatusBadRequest)
		return
	}

	// file, header
	file, _, err := r.FormFile("file")
	if err != nil {
		fmt.Printf("Failed to read file: %s", err)
		http.Error(w, "Invalid file upload", http.StatusBadRequest)
		return
	}
	defer file.Close()
	fmt.Println("reading uploaded file key")

	// Read the first 512 bytes to detect content type
	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		http.Error(w, "Failed to read file content", http.StatusInternalServerError)
		return
	}

	// Reset the file pointer to the beginning
	_, err = file.Seek(0, 0)
	if err != nil {
		http.Error(w, "Failed to reset file pointer", http.StatusInternalServerError)
		return
	}

	contentType := http.DetectContentType(buffer)
	fmt.Printf("Detected contentType: %v\n", contentType)
	if !strings.HasPrefix(contentType, "image/png") {
		http.Error(w, "Only png files are allowed (for now)", http.StatusUnsupportedMediaType)
		return
	}

	tempInput, err := os.CreateTemp("", "upload-*.tmp")
	if err != nil {
		http.Error(w, "Failed to create temp file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempInput.Name())
	defer tempInput.Close()

	tempOutput, err := os.CreateTemp("", "upload-*.tmp")
	if err != nil {
		http.Error(w, "Failed to create temp file", http.StatusInternalServerError)
		return
	}
	defer tempOutput.Close()

	// Copy uploaded file to temp input
	_, err = io.Copy(tempInput, file)
	if err != nil {
		http.Error(w, "Failed to save uploaded file", http.StatusInternalServerError)
		return
	}

	// just for basic security, remove all metadata from the image.
	// Use ffmpeg to remove metadata (steganography, etc)
	cmd := exec.Command("ffmpeg", "-i", tempInput.Name(), "-map_metadata", "-1", "-c:v", "copy", "-f", "image2", "-y", tempOutput.Name())
	// bytes, err := cmd.CombinedOutput()
	err = cmd.Run()
	if err != nil {
		fmt.Printf("err: %v\n", err)
		// fmt.Printf("string: %v\n", string(bytes))
		http.Error(w, "Failed to process image", http.StatusInternalServerError)
		return
	}

	// don't bother checking since won't
	// catch any real problems in logic
	// or a race condition with deleting the file
	// + eagle has an error queue anyways
	//
	// check if exists
	// processedFile, err := os.Open(tempOutput.Name())

	// upload the processed file to eagle.
	opts := eagle.ItemAddFromPathOptions{
		Path:       tempOutput.Name(),
		Name:       "",
		Website:    "",
		Annotation: "Uploaded Remotely",
		Tags:       []string{},
		FolderId:   "",
	}
	err = eagle.ItemAddFromPath(BASE_URL, opts)
	if err != nil {
		fmt.Printf("io.Copy err: %v\n", err)
	}
	_, err = w.Write([]byte("success. item uploaded."))
	if err != nil {
		fmt.Printf("error sending response to client: %v\n", err)
	}

	os.Remove(tempOutput.Name())
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
		Folders: r.URL.Query().Get("folders"),
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
	renderErr := galleryTempl.Execute(w, GalleryData{Items: items, Page: 0, AllTags: tagNames, AllFolders: folders, Filter: filter})
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
		Folders: r.URL.Query().Get("folders"),
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
