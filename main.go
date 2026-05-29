package main

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"pic-shortener/core"
	database "pic-shortener/sqlite"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	DB          *sql.DB
	AlreadyDone map[string]string // it like map[hash:quality:width]filepath to use already cashed images
	err         error
	mu          sync.RWMutex
)

func main() {
	DB, err = database.InitDBSQLite()
	if err != nil {
		log.Printf("Initialazing error %v\n", err)
		os.Exit(0)
	} else {
		log.Printf("Database runs successfully!\n")
	}
	defer DB.Close()

	mu.Lock()
	AlreadyDone, err = database.UploadMap()
	if err != nil {
		log.Printf("Error while filling map %v\n", err)
		return
	} else {
		log.Printf("Map filled successfully\n")
	}
	mu.Unlock()

	mux := http.NewServeMux()
	server := &http.Server{
		Handler:     mux,
		Addr:        ":10000",
		ReadTimeout: 10 * time.Second,
	}
	mux.HandleFunc("/", mainHandler)
	mux.HandleFunc("/images", imageHandler)
	log.Printf("Service runs: %s", server.Addr)
	err = server.ListenAndServe()
	if err != nil {
		log.Printf("ERR %v\n", err)
		return
	}
}

func imageHandler(w http.ResponseWriter, r *http.Request) {
	ip := r.RemoteAddr

	log.Printf("[%s] We got new request: %v\n", ip, r.Body)

	if r.Method == http.MethodGet {
		hash := r.URL.Query().Get("hash") // gets all data from get query
		width := r.URL.Query().Get("width")
		quality := r.URL.Query().Get("quality")

		var foundPath string

		queryForMap := fmt.Sprintf("%s:%s:%s", hash, quality, width)

		val, exists := AlreadyDone[queryForMap]
		if exists == true { // checks existing
			foundPath = val
			http.ServeFile(w, r, foundPath)
			return
		} else {
			reference := fmt.Sprintf("storage/originals/%v.jpg", hash)
			foundPath = reference
		}

		qual, err := strconv.Atoi(quality)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		wid, err := strconv.Atoi(width)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		original_id, err := database.GetOriginalData(hash)
		if err != nil {
			log.Printf("ERR at getting original id %v\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		id := strconv.Itoa(original_id)

		newFoundPath := strings.SplitN(foundPath, ".", 2)

		preDestPath := strings.SplitN(newFoundPath[0], "/", 2)
		preDestPathCashed := fmt.Sprintf("%s/cashe/%s", preDestPath[0], hash)

		dstPath := fmt.Sprintf("%s_%vX%v.jpg", preDestPathCashed, width, quality)

		err = core.ResizeJPEG(foundPath, dstPath, wid, qual)
		if err == nil {
			log.Printf("We made new cashed image: %v\n", dstPath)
		} else {
			log.Printf("ERR at resizing image %v\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.ServeFile(w, r, dstPath) // sends file to the client
		log.Printf("We sent new shortcuted image to [%s]\n", ip)

		err = database.SaveCashedAfterGetRequest(id, width, quality, dstPath, int(time.Now().Unix()))
		if err != nil {
			log.Printf("ERR at saving cashed data %v\n", err)
			http.Error(w, "Image wasn't saved successfully\n", http.StatusInternalServerError)
			return
		}

		key := fmt.Sprintf("%v:%v:%v", hash, quality, width)
		mu.Lock()
		AlreadyDone[key] = dstPath
		mu.Unlock()
	} else if r.Method == http.MethodPost {
		//
		r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

		err = r.ParseMultipartForm(10 << 20)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		network, _, err := r.FormFile("image")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer network.Close()

		hasher := sha256.New()

		tempfile, err := os.Create("storage/originals/temp_upload.jpg")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer tempfile.Close()

		mw := io.MultiWriter(tempfile, hasher)

		_, err = io.Copy(mw, network)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		hashString := fmt.Sprintf("%x", hasher.Sum(nil))

		fp := fmt.Sprintf("storage/originals/%s.jpg", hashString)
		err = os.Rename("storage/originals/temp_upload.jpg", fp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = database.SaveOriginalAfterPostRequest(hashString, fp, int(time.Now().Unix()))
		if err == nil {
			log.Printf("New picture: %s\n", fp)
			mu.Lock()
			AlreadyDone[hashString+":"+"original-quality"+":"+"original-width"] = fp
			mu.Unlock()
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Write([]byte("File has been saved successfully! HASH: " + hashString + "\n"))
	}
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, thats my API to optimize your photos")
}
