package main

import (
	// "encoding/json"

	"embed"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"html/template"

	"github.com/eissar/eagle-go"
	"github.com/labstack/echo/v4"

	_ "embed"
)

const BASE_URL = "http://127.0.0.1:41595"

var PageSize = 20 // 20 is default

func thumbnailHandler(c echo.Context) error {
	id := c.Param("itemId")
	resFlag := c.QueryParam("fq") // full quality

	getThumbnail := func() (string, error) {
		if resFlag == "true" {
			return GetEagleThumbnailFullRes(id)
		}
		return GetEagleThumbnail(id)
	}

	thumbnail, err := getThumbnail()
	if err != nil {
		res := fmt.Sprintf("get thumbnail path=%s err=%s", c.Path(), err.Error())
		return c.String(400, res)
	}
	// filepath exists.
	return c.File(thumbnail)
}

func galleryHandler(c echo.Context) error {
	items, fetchErr := eagle.ItemList(BASE_URL, eagle.ItemListOptions{Limit: PageSize})
	if fetchErr != nil {
		return c.String(echo.ErrInternalServerError.Code, fetchErr.Error())
	}

	folders, fetchErr := eagle.FolderList(BASE_URL)
	if fetchErr != nil {
		return c.String(echo.ErrInternalServerError.Code, fetchErr.Error())
	}
	folderNames := make([]string, len(folders))
	for i, f := range folders {
		folderNames[i] = f.Name
	}

	tags, fetchErr := eagle.TagList(BASE_URL)
	if fetchErr != nil {
		return c.String(echo.ErrInternalServerError.Code, fetchErr.Error())
	}
	tagNames := make([]string, len(tags))
	for i, t := range tags {
		tagNames[i] = t.Name
	}

	// first draw
	renderErr := galleryTempl.Execute(c.Response().Writer, PageData{items, 0, tagNames, folderNames})
	// GalleryPage(items, nil, folderNames).Render(c.Request().Context(), c.Response())
	if renderErr != nil {
		fmt.Printf("renderErr: %v\n", renderErr)
		return c.String(echo.ErrInternalServerError.Code, "failed to render template")
	}
	return nil
}

func itemsHandler(c echo.Context) error {

	page, err := strconv.Atoi(c.QueryParam("offset")) // easier to read
	if err != nil {
		page = 0
	}

	// perPage will be statically set at 20.
	// offset does not work like I assumed.

	// perPage, err := strconv.Atoi(c.QueryParam("loadPerPage"))
	// if err != nil {
	// 	perPage = PageSize
	// }

	opts := eagle.ItemListOptions{
		Limit:   PageSize,
		Offset:  page,
		OrderBy: "CREATEDATE",
		Keyword: c.QueryParam("keyword"),
		Ext:     "",
		Tags:    c.QueryParam("tags"),
		Folders: "",
	}

	items, fetchErr := eagle.ItemList(BASE_URL, opts)
	if fetchErr != nil {
		return c.String(echo.ErrInternalServerError.Code, fetchErr.Error())
	}

	folders, fetchErr := eagle.FolderList(BASE_URL)
	if fetchErr != nil {
		return c.String(echo.ErrInternalServerError.Code, fetchErr.Error())
	}
	folderNames := make([]string, len(folders))
	for i, f := range folders {
		folderNames[i] = f.Name
	}

	page += 1

	// just re-use page data
	renderErr := itemsTempl.Execute(c.Response().Writer, PageData{items, page, nil, nil})
	if renderErr != nil {
		fmt.Printf("renderErr: %v\n", renderErr)
		return c.String(echo.ErrInternalServerError.Code, "failed to render template")
	}
	return nil
}

// on my device thumbnails ONLY end with _thumbnail.png or they do not exist.
// this returns the full file path to the highest available resolution of the file.
func GetEagleThumbnailFullRes(itemId string) (string, error) {
	thumbnail, err := GetEagleThumbnail(itemId)
	if err != nil {
		return thumbnail, fmt.Errorf("getEagleThumbnail: err=%w", err)
	}

	thumbnail, err = resolveThumbnailPath(thumbnail)
	if err != nil {
		return thumbnail, fmt.Errorf("getEagleThumbnail: err=%w", err)
	}

	// TODO: we call os.Stat unnecessarily if we match full-res.
	if _, err = os.Stat(thumbnail); err != nil {
		return thumbnail, fmt.Errorf("getEagleThumbnail: err=%w", err)
	}

	//  TODO: fallback list all files other than metadata.json & _thumbnail.png?
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

// tries to find the actual filepath from the response
// of request api/item/thumbnail.
// also calls `url.PathUnescape` on the url.
// then checks if there are
// any files matching `allowed_filetypes`.
func resolveThumbnailPath(thumbnail string) (string, error) {
	thumbnail, err := url.PathUnescape(thumbnail)
	if err != nil {
		return thumbnail, fmt.Errorf("resolvethumb: error cleaning thumbnail path: %s:", err.Error())
	}

	if !strings.HasSuffix(thumbnail, "_thumbnail.png") {
		// should already be the full-resolution file.
		return thumbnail, nil
	}

	// try to find the full-res file.
	thumbnailRoot := strings.TrimSuffix(thumbnail, "_thumbnail.png")

	for _, typ := range allowed_filetypes {
		joinedPath := thumbnailRoot + typ
		if _, err := os.Stat(joinedPath); err == nil {
			// if no error, file exists; return that file.
			return joinedPath, nil
		}
	}

	return thumbnail, nil
	// TODO: create NoFullResolutionErr
	//
	// fmt.Errorf("resolvethumb: no full-res file at path=%s, err=%w", thumbnail)
}

type PageData struct {
	Items      []*eagle.ListItem
	Page       int // offset = Limit * Page
	AllTags    []string
	AllFolders []string
}

var galleryTempl *template.Template
var itemsTempl *template.Template

// Helper functions for the template.
var tmplFuncs = template.FuncMap{
	"join":   strings.Join,
	"lower":  strings.ToLower,
	"printf": fmt.Sprintf,
}

//go:embed gallery.gohtml
var tmplFS embed.FS

func main() {
	// var err error
	// Parse embedded templates instead of reading from disk
	galleryTempl = template.Must(template.New("gallery").Funcs(tmplFuncs).ParseFS(tmplFS, "gallery.gohtml"))
	itemsTempl = template.Must(template.New("items").Funcs(tmplFuncs).ParseFS(tmplFS, "gallery.gohtml"))

	e := echo.New()
	e.GET("/gallery", galleryHandler)
	e.GET("/img/:itemId", thumbnailHandler)
	e.GET("/items", itemsHandler)

	addr := ":8081"
	e.Logger.Fatal(e.Start(addr))
}
