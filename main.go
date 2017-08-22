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

maim -s | curl -s -F "f=@-" '{{.}}'
`)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			err := db.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket(bucketName)
				uniqueID, _ := b.NextSequence()
				id := uniqueID % 100

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

				url := fmt.Sprintf("%s/%s.png", rootURL, key)

				if r.URL.Path == "/api/up" {
					fmt.Fprintf(w, "0,%s,%x,%d", url, uniqueID, 0)
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
		err := db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket(bucketName)
			idInPath := strings.Split(r.URL.Path[1:], ".")[0]
			id, err := strconv.ParseUint(idInPath, 16, 64)
			if err != nil {
				return err
			}
			idInDb := fmt.Sprintf("%x", id%100)

			v := b.Get([]byte(idInDb))
			if v != nil {
				w.Header().Set("Content-Type", "image/png")
				w.Write(v)
			} else {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "Not found: %s", idInDb)
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
