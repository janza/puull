package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/boltdb/bolt"
)

const (
	MAX_IMAGES = 100000
)

func main() {
	db, err := bolt.Open("puull.db", 0600, nil)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	bucketName := []byte("Images")

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketName)
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}

	rootURL := os.Getenv("ROOT_URL")

	t, err := template.New("index").Parse(`#!/usr/bin/env bash
# puull: image uploader
#
# Requires: curl, maim
#
# Installation: curl '{{.}}' | sudo tee /usr/bin/puull && sudo chmod +x /usr/bin/puull
#
# Source: https://github.com/janza/puull
#
# NO DROPBOX, NO PUUSH, ONLY PUULL
set -e

if [[ $(uname) == Linux ]]; then
	maim -s | curl -s -F "f=@-" '{{.}}'
else
	screencapture -i /tmp/screenshot.png
	curl -s -F "f=@/tmp/screenshot.png" '{{.}}'
fi
`)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			err := db.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket(bucketName)
				uniqueID, _ := b.NextSequence()
				id := uniqueID % MAX_IMAGES

				key := fmt.Sprintf("%x", id)

				file, _, err := r.FormFile("f")
				if err != nil {
					return err
				}

				buf := new(bytes.Buffer)
				n, err := buf.ReadFrom(file)
				if err != nil {
					return err
				}
				if n == 0 {
					return fmt.Errorf("empty upload")
				}

				err = b.Put([]byte(key), buf.Bytes())
				if err != nil {
					return err
				}

				url := fmt.Sprintf("%s/%s.png", rootURL, fmt.Sprintf("%x", uniqueID))

				if r.URL.Path == "/api/up" {
					fmt.Fprintf(w, "0,%s,%x,%d", url, fmt.Sprintf("%x", uniqueID), 0)
				} else {
					fmt.Fprint(w, url)
				}
				return nil
			})

			if err != nil {
				http.Error(w, err.Error(), 500)
			}
			return
		}

		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/plain")
			err := t.Execute(w, rootURL)
			if err != nil {
				http.Error(w, err.Error(), 500)
			}
			return
		}
		err := db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket(bucketName)
			splitPath := strings.Split(r.URL.Path[1:], ".")
			extension := splitPath[len(splitPath)-1]
			idInPath := splitPath[0]
			id, err := strconv.ParseUint(idInPath, 16, 64)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, "Invalid request")
				return nil
			}

			uniqueID, _ := b.NextSequence()
			if int(id) < int(uniqueID)-MAX_IMAGES {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "Image expired")
				return nil
			}
			idInDb := fmt.Sprintf("%x", id%MAX_IMAGES)

			v := b.Get([]byte(idInDb))
			if v != nil {
				contentType := "image/png"
				if extension == "mkv" {
					contentType = "video/x-matroska"
				} else if extension == "mp4" {
					contentType = "video/mp4"
				} else if extension == "webm" {
					contentType = "video/webm"
				}
				w.Header().Set("Content-Type", contentType)
				w.Write(v)
			} else {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "Not found")
			}

			return nil
		})
		if err != nil {
			http.Error(w, err.Error(), 500)
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Print(http.ListenAndServe(":"+port, nil))
}
