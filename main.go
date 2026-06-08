package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"pic-shortener/core"
	database "pic-shortener/sqlite"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"gopkg.in/yaml.v3"
)

type MainConfig struct {
	Server   ServerConfig   `yaml:"server"`
	Database DBConfig       `yaml:"database"`
	Postgres PostgresConfig `yaml:"postgres"`
}

type ServerConfig struct {
	Port  string        `yaml:"port"`
	Idle  time.Duration `yaml:"idle"`
	Read  time.Duration `yaml:"read"`
	Write time.Duration `yaml:"write"`
}

type DBConfig struct {
	StoragePath  string `yaml:"originals_path"`
	CachedPath   string `yaml:"cached_path"`
	TempFilePath string `yaml:"temp_file_path"`
	LogFilePath  string `yaml:"app_logs_path"`
}

type PostgresConfig struct {
	Name     string `yaml:"name"`
	Host     string `yaml:"host"`
	Password string `yaml:"password"`
	User     string `yaml:"user"`
}

var (
	Cfg  *MainConfig
	Path string
)

type ctxKey string

const requestIdKey ctxKey = "reqKeyString"

var (
	DB             *pgxpool.Pool
	AlreadyDone    map[string]string // it like map[hash:quality:width]filepath to use already cashed images
	err            error
	mu             sync.RWMutex
	cacheHitsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pic_shortener_cached_hits_total",
		Help: "counter when map AlreadyDone saved server resources",
	})

	requestDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "pic_shortener_request_duration_seconds",
		Help:    "historgam of average time",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5},
	})

	successfulTotals = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pic_shortener_total_successful_optimizations",
		Help: "Total of optimized photoes",
	})

	errorsOfService = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pic_shortener_total_errors",
		Help: "Total errors counter",
	})
)

func main() {

	fmt.Print(`
 ██████╗ ██╗ ██████╗    ███████╗███████╗██████╗ ██╗   ██╗██╗ ██████╗███████╗
 ██╔══██╗██║██╔════╝    ██╔════╝██╔════╝██╔══██╗██║   ██║██║██╔════╝██╔════╝
 ██████╔╝██║██║         ███████╗█████╗  ██████╔╝██║   ██║██║██║     █████╗
 ██╔═══╝ ██║██║         ╚════██║██╔══╝  ██╔══██╗╚██╗ ██╔╝██║██║     ██╔══╝
 ██║     ██║╚██████╗    ███████║███████╗██║  ██║ ╚████╔╝ ██║╚██████╗███████╗
 ╚═╝     ╚═╝ ╚═════╝    ╚══════╝╚══════╝╚═╝  ╚═╝  ╚═══╝  ╚═╝ ╚═════╝╚══════╝

 >> Production server is up and running...
`)

	postgreCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()

	quit := make(chan os.Signal, 1)

	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	Cfg, err = UnparseYAML("config.yml")
	if errors.Is(err, os.ErrNotExist) {
		log.Printf("File does not exist! %v\n", err)
		return
	} else if err != nil {
		log.Println(err)
		return
	}

	logFile, err := os.OpenFile(Cfg.Database.LogFilePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
	if errors.Is(err, os.ErrNotExist) {
		log.Printf("ERR cause Log file doesn't exist %v\n", err)
		return
	} else if err != nil {
		log.Println("ERR at opening log file", err)
		return
	}
	defer logFile.Close()

	mw := io.MultiWriter(os.Stdout, logFile)

	log.SetOutput(mw)

	log.Printf("Log system is running now\n")

	var PrePath string = Cfg.Database.StoragePath
	Path = PrePath

	var dbError *database.DBError

	DB, err = database.InitDBSQLite(postgreCtx, Cfg.Postgres.User, Cfg.Postgres.Password, Cfg.Postgres.Host, Cfg.Postgres.Name)
	if errors.As(err, &dbError) {
		log.Fatalf("Logged for sysadmin [%s]\n", dbError.Error())
	} else {
		log.Printf("Database runs successfully!\n")
	}
	defer DB.Close()

	mu.Lock()
	AlreadyDone, err = database.UploadMap(postgreCtx) // uploader starts
	if err != nil {
		log.Printf("Error while filling map %v\n", err)
		return
	} else {
		log.Printf("Map filled successfully\n")
	}
	mu.Unlock()

	mux := http.NewServeMux()
	server := &http.Server{
		Handler:      LogMiddleware(mux),
		Addr:         Cfg.Server.Port,
		ReadTimeout:  Cfg.Server.Read,
		IdleTimeout:  Cfg.Server.Idle,
		WriteTimeout: Cfg.Server.Write,
	}

	mux.HandleFunc("/", mainHandler)
	mux.HandleFunc("/images", imageHandler)
	mux.Handle("/metrics", promhttp.Handler())
	log.Printf("Service runs: %s", server.Addr)

	go func() {
		err = server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Printf("ERR %v\n", err)
			return
		}
	}()

	<-quit

	log.Printf("Signal has been recieved, we are going to stop the service\n")

	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Service has been stopped: %v", err)
	}
}

func imageHandler(w http.ResponseWriter, r *http.Request) {
	postgreCtx := r.Context()
	ip := r.RemoteAddr

	reqID, _ := r.Context().Value(requestIdKey).(string)

	log.Printf("[%s] [%s] We got new request: %v\n", ip, reqID, r.Method)

	if r.Method == http.MethodGet {
		hash := r.URL.Query().Get("hash") // gets all data from get query
		width := r.URL.Query().Get("width")
		quality := r.URL.Query().Get("quality")

		var foundPath string

		queryForMap := fmt.Sprintf("%s:%s:%s", hash, quality, width)

		mu.RLock()
		val, exists := AlreadyDone[queryForMap]
		if exists == true { // checks existing
			cacheHitsTotal.Inc() // adds 1 to global prometheus counter
			foundPath = val
			http.ServeFile(w, r, foundPath)
			mu.RUnlock()
			return
		} else {
			reference := fmt.Sprintf("storage/originals/%v.jpg", hash)
			foundPath = reference
		}
		mu.RUnlock()

		qual, err := strconv.Atoi(quality)
		if err != nil {
			errorsOfService.Inc()
			log.Printf("[%s] [%s] typed data incorrectly\n", reqID, ip)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		wid, err := strconv.Atoi(width)
		if err != nil {
			errorsOfService.Inc()
			log.Printf("[%s] [%s] typed data incorrectly\n", reqID, ip)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var dbError *database.DBError

		original_id, err := database.GetOriginalData(postgreCtx, hash)
		if errors.As(err, &dbError) {
			errorsOfService.Inc()
			log.Printf("ERR [%s] %v\n", reqID, dbError.Error())
			http.Error(w, dbError.Message, dbError.Code)
			return
		}

		id := strconv.Itoa(original_id)

		preDestPath := Cfg.Database.CachedPath

		preDestPathCashed := fmt.Sprintf("%s/%s", preDestPath, hash)

		dstPath := fmt.Sprintf("%s_%vX%v.jpg", preDestPathCashed, width, quality)

		var MediaErr *core.MediaError

		err = core.ResizeJPEG(foundPath, dstPath, wid, qual)
		if err == nil {
			log.Printf("We made new cashed image: %v\n", dstPath)
		} else {
			if errors.As(err, &MediaErr) {
				errorsOfService.Inc()
				log.Printf("Logged for system admin [%s] %s\n", reqID, MediaErr.Error())
				http.Error(w, MediaErr.Message, MediaErr.Code)
				return
			}
		}

		http.ServeFile(w, r, dstPath) // sends file to the client
		log.Printf("We sent new shortcuted image to [%s]\n", ip)

		err = database.SaveCashedAfterGetRequest(postgreCtx, id, width, quality, dstPath, int(time.Now().Unix()))
		if errors.As(err, &dbError) {
			errorsOfService.Inc()
			log.Printf("ERR [%s] %v\n", reqID, dbError.Error())
			return
		}

		key := fmt.Sprintf("%v:%v:%v", hash, quality, width)
		mu.Lock()
		AlreadyDone[key] = dstPath
		mu.Unlock()
	} else if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

		err = r.ParseMultipartForm(10 << 20)
		if err != nil {
			errorsOfService.Inc()
			log.Printf("ERR at parsing Multipart Form [%s] %v\n", reqID, err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		network, _, err := r.FormFile("image")
		if err != nil {
			errorsOfService.Inc()
			log.Printf("ERR at forming network file [%s] %v\n", reqID, err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer network.Close()

		hasher := sha256.New()

		tempfile, err := os.Create(Cfg.Database.TempFilePath)
		if err != nil {
			errorsOfService.Inc()
			log.Printf("ERR at creating temp file [%s] %v\n", reqID, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer tempfile.Close()

		mw := io.MultiWriter(tempfile, hasher)

		var mediaErr *core.MediaError

		stream, err := core.Detector(network)
		if errors.As(err, &mediaErr) {
			errorsOfService.Inc()
			log.Printf("ERR for admin: [%s] %v\n", reqID, mediaErr.Error())
			http.Error(w, mediaErr.Message, mediaErr.Code)
			return
		}

		_, err = io.Copy(mw, stream)
		if err != nil {
			errorsOfService.Inc()
			log.Printf("ERR while copied data [%s] %v\n", reqID, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		hashString := fmt.Sprintf("%x", hasher.Sum(nil))

		fp := fmt.Sprintf(Path+"/%s.jpg", hashString)
		err = os.Rename(Cfg.Database.TempFilePath, fp)
		if err != nil {
			errorsOfService.Inc()
			log.Printf("ERR at renaming temp file [%s] %v\n", reqID, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var dbError *database.DBError

		err = database.SaveOriginalAfterPostRequest(postgreCtx, hashString, fp, int(time.Now().Unix()))
		if err == nil {
			log.Printf("New picture by [%s]: %s\n", reqID, fp)
			mu.Lock()
			AlreadyDone[hashString+":"+"original-quality"+":"+"original-width"] = fp
			mu.Unlock()
		} else if errors.As(err, &dbError) {
			errorsOfService.Inc()
			log.Printf("ERR [%s] %v\n", reqID, dbError.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		successfulTotals.Inc() // prometheus metric

		w.Write([]byte("File has been saved successfully! HASH: " + hashString + "\n"))
	} else {
		errorsOfService.Inc()
		log.Printf("[%s] [%s] 405\n", reqID, ip)
		http.Error(w, "Method is not allowed!", http.StatusMethodNotAllowed)
		return
	}
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, thats my API to optimize your photos")
}

func LogMiddleware(nextFunc http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		start := time.Now().Round(time.Millisecond)

		uniqueIdForCtx := fmt.Sprintf("REQ-%d", 1+rand.IntN(100000))

		ctx := context.WithValue(r.Context(), requestIdKey, uniqueIdForCtx)

		newReq := r.WithContext(ctx)

		w.Header().Set("X-Request-ID", uniqueIdForCtx)

		nextFunc.ServeHTTP(w, newReq)

		result := time.Since(start)

		requestDuration.Observe(result.Seconds())
		log.Printf("Done [%s] [%v] %v ms\n", ip, uniqueIdForCtx, result)
	})
}

func UnparseYAML(filepath string) (*MainConfig, error) {
	bytes, err := os.ReadFile(filepath)
	if err != nil {
		errorsOfService.Inc()
		return nil, fmt.Errorf("ERR at parsing %s %w", filepath, err)
	}

	var Cfg *MainConfig

	err = yaml.Unmarshal(bytes, &Cfg)
	return Cfg, nil
}
