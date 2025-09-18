package main

import (
	// "encoding/json"

	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/eissar/eagle-go"
	"github.com/labstack/echo/v4"
)

const BASE_URL = "http://127.0.0.1:41595"

func ServeThumbnailHandler() echo.HandlerFunc {
	return func(c echo.Context) error {
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
}

func itemsHandler(c echo.Context) error {
	items, fetchErr := eagle.ItemList(BASE_URL, eagle.ItemListOptions{Limit: 200})
	if fetchErr != nil {
		return c.String(http.StatusInternalServerError, fetchErr.Error())
	}

	folders, fetchErr := eagle.FolderList(BASE_URL)
	if fetchErr != nil {
		return c.String(http.StatusInternalServerError, fetchErr.Error())
	}
	folderNames := make([]string, len(folders))
	for i, f := range folders {
		folderNames[i] = f.Name
	}

	// renderErr := itemsTemplate(items, BASE_URL).Render(c.Request().Context(), c.Response())
	renderErr := GalleryPage(items, nil, folderNames).Render(c.Request().Context(), c.Response())
	if renderErr != nil {
		return c.String(http.StatusInternalServerError, "failed to render template")
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
		// should already the full-resolution file.
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

func main() {
	e := echo.New()
	e.GET("/items", itemsHandler)
	// e.GET("/eagle://item/:itemId", ServeThumbnailHandler())
	e.GET("/img/:itemId", ServeThumbnailHandler())

	addr := ":8080"
	e.Logger.Fatal(e.Start(addr))
}
