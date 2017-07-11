package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"time"

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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			err := db.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket(bucketName)
				id, _ := b.NextSequence()

				key := fmt.Sprintf("%s:%x", time.Now().Format("2006-01-02"), id)

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

				url := fmt.Sprintf("%s/%s", rootUrl, key)

				fmt.Fprintf(w, "0,%s,%x,%d", url, id, 0)
				return nil
			})

			if err != nil {
				http.Error(w, err.Error(), 500)
			}
		} else {
			err := db.View(func(tx *bolt.Tx) error {
				b := tx.Bucket(bucketName)
				id := []byte(r.URL.Path[1:])
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
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Print(http.ListenAndServe(":"+port, nil))
}
