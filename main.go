package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"os"
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

	rootUrl := os.Getenv("ROOT_URL")

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

maim -s | curl -s -F "f=@-" '{{.}}' | cut -f 2 -d,
`)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			err := db.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket(bucketName)
				id, _ := b.NextSequence()
				id = id % 100

				key := fmt.Sprintf("%x", id)

				file, _, err := r.FormFile("f")
				if err != nil {
					return err
				}

				buf := new(bytes.Buffer)
				buf.ReadFrom(file)

				err = b.Put([]byte(key), buf.Bytes())
				if err != nil {
					return err
				}

				url := fmt.Sprintf("%s/%s.png", rootUrl, key)

				if r.URL.Path == "/api/up" {
					fmt.Fprintf(w, "0,%s,%x,%d", url, id, 0)
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
			err := t.Execute(w, rootUrl)
			if err != nil {
				http.Error(w, err.Error(), 500)
			}
			return
		}
		err := db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket(bucketName)
			id := []byte(strings.Split(r.URL.Path[1:], ".")[0])
			v := b.Get(id)
			if v != nil {
				w.Header().Set("Content-Type", "image/png")
				w.Write(v)
			} else {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "Not found: %s", id)
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
