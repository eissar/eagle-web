//go:build dev

// Development build with live template reloading
package main

import (
	"html/template"
	"log"
	"time"

	"github.com/radovskyb/watcher"
)

func reloadTemplates() {
	gallery, err := template.New("gallery").Funcs(tmplFuncs).ParseFiles("gallery.gohtml")
	if err != nil {
		log.Printf("Failed to reload gallery template: %v", err)
		return
	}
	items, err := template.New("items").Funcs(tmplFuncs).ParseFiles("gallery.gohtml")
	if err != nil {
		log.Printf("Failed to reload items template: %v", err)
		return
	}
	galleryTempl = gallery
	itemsTempl = items
	log.Println("Templates reloaded successfully")
}

func init() {
	reloadTemplates()

	w := watcher.New()
	
	// Set max events to process at a time
	w.SetMaxEvents(100)
	
	// Only notify write and create events
	w.FilterOps(watcher.Write, watcher.Create)

	go func() {
		for {
			select {
			case event := <-w.Event:
				log.Println(event.String())
				reloadTemplates()
			case err := <-w.Error:
				log.Println("Error:", err)
			case <-w.Closed:
				return
			}
		}
	}()

	// Watch the template file with a 500ms polling interval
	if err := w.Add("gallery.gohtml"); err != nil {
		log.Fatal(err)
	}
	
	// Start the watcher with a polling interval
	go func() {
		if err := w.Start(time.Millisecond * 500); err != nil {
			log.Fatal(err)
		}
	}()
}
