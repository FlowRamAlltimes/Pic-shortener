package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
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
	"github.com/minio/minio-go/v7"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"gopkg.in/yaml.v3"
)

type MainConfig struct {
	Server   ServerConfig   `yaml:"server"`
	Database DBConfig       `yaml:"database"`
	Postgres PostgresConfig `yaml:"postgres"`
	MinIO    MinIOEnv       `yaml:"minio"`
}

type ServerConfig struct {
	Port  string        `yaml:"port"`
	Idle  time.Duration `yaml:"idle"`
	Read  time.Duration `yaml:"read"`
	Write time.Duration `yaml:"write"`
}

type DBConfig struct {
	LogFilePath string `yaml:"app_logs_path"`
}

type PostgresConfig struct {
	Name     string `yaml:"name"`
	Host     string `yaml:"host"`
	Password string `yaml:"password"`
	User     string `yaml:"user"`
}

type MinIOEnv struct {
	minioClient     *minio.Client
	CachedBucket    string `yaml:"cached_bucket"`
	OriginalBucket  string `yaml:"originals_bucket"`
	Endpoint        string `yaml:"endpoint"`
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
}

type ctxKey string

const requestIdKey ctxKey = "reqKeyString"

var (
	DB             *pgxpool.Pool
	AlreadyDone    map[string]string // it like map[hash:quality:width]filepath to use already cashed images
	err            error
	mu             sync.RWMutex
	Cfg            *MainConfig
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
	postgreCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()

	fmt.Print(`
 ██████╗ ██╗ ██████╗    ███████╗███████╗██████╗ ██╗   ██╗██╗ ██████╗███████╗
 ██╔══██╗██║██╔════╝    ██╔════╝██╔════╝██╔══██╗██║   ██║██║██╔════╝██╔════╝
 ██████╔╝██║██║         ███████╗█████╗  ██████╔╝██║   ██║██║██║     █████╗
 ██╔═══╝ ██║██║         ╚════██║██╔══╝  ██╔══██╗╚██╗ ██╔╝██║██║     ██╔══╝
 ██║     ██║╚██████╗    ███████║███████╗██║  ██║ ╚████╔╝ ██║╚██████╗███████╗
 ╚═╝     ╚═╝ ╚═════╝    ╚══════╝╚══════╝╚═╝  ╚═╝  ╚═══╝  ╚═╝ ╚═════╝╚══════╝

 >> Production server is up and running...
`)

	Cfg, err = UnparseYAML("config.yml")
	if errors.Is(err, os.ErrNotExist) {
		log.Printf("File does not exist! %v\n", err)
		return
	} else if err != nil {
		log.Println(err)
		return
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	slog.SetDefault(logger)

	var customErr *core.MediaError

	client, err := core.InitMinIO(postgreCtx, Cfg.MinIO.CachedBucket, Cfg.MinIO.OriginalBucket, Cfg.MinIO.Endpoint, Cfg.MinIO.AccessKeyID, Cfg.MinIO.SecretAccessKey, false)
	if errors.As(err, &customErr) {
		slog.Error(customErr.Op, "message", customErr.Message, "err", customErr.Err)
		return
	}

	env := &MinIOEnv{
		minioClient:     client,
		CachedBucket:    Cfg.MinIO.CachedBucket,
		OriginalBucket:  Cfg.MinIO.OriginalBucket,
		Endpoint:        Cfg.MinIO.Endpoint,
		AccessKeyID:     Cfg.MinIO.AccessKeyID,
		SecretAccessKey: Cfg.MinIO.SecretAccessKey,
	}

	quit := make(chan os.Signal, 1)

	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

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
		Handler:      env.LogMiddleware(mux),
		Addr:         Cfg.Server.Port,
		ReadTimeout:  Cfg.Server.Read,
		IdleTimeout:  Cfg.Server.Idle,
		WriteTimeout: Cfg.Server.Write,
	}

	mux.HandleFunc("/", mainHandler)
	mux.HandleFunc("/images", env.imageHandler)
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
		log.Fatalf("ERR %v", err)
	}

	log.Printf("Service has been stopped successfully!")
}

func (env *MinIOEnv) imageHandler(w http.ResponseWriter, r *http.Request) {
	postgreCtx, stop := context.WithTimeout(r.Context(), 3*time.Second)
	defer stop()

	ip := r.RemoteAddr

	reqID, _ := r.Context().Value(requestIdKey).(string)

	log.Printf("[%s] [%s] We got new request: %v\n", ip, reqID, r.Method)

	if r.Method == http.MethodGet {
		hash := r.URL.Query().Get("hash") // gets all data from get query
		width := r.URL.Query().Get("width")
		quality := r.URL.Query().Get("quality")
		format := r.URL.Query().Get("format")

		slog.Debug("Format of query", "req_id", reqID, "format", format)

		var foundPath string

		queryForMap := fmt.Sprintf("%s_%s_%s_%s", hash, width, quality, format)

		mu.RLock()
		_, exists := AlreadyDone[queryForMap] // val = bucket
		mu.RUnlock()
		if exists == true {
			objectName := fmt.Sprintf("%s_%s_%s_%s", hash, width, quality, format)
			obj, err := env.minioClient.GetObject(postgreCtx, env.CachedBucket, objectName, minio.GetObjectOptions{})
			if err != nil {
				slog.Error("ERR at getting object", "req_id", reqID, "err", err)
				http.Error(w, "Unexpected error, tell your REQ-ID to our support", 500)
				return
			}
			defer obj.Close()

			info, err := obj.Stat()
			if err != nil {
				slog.Error("ERR at getting stat from object", "req_id", reqID, "err", err)
				http.Error(w, "Unexpected error, tell your REQ-ID to our support", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "image/"+format)
			w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
			w.WriteHeader(http.StatusOK)

			if _, err = io.Copy(w, obj); err != nil {
				slog.Error("ERR at copying data", "req_id", reqID, "err", err)
				http.Error(w, "Unexpected error, tell your REQ-ID to our support", 500)
			}

			cacheHitsTotal.Inc() // adds 1 to global prometheus counter
			successfulTotals.Inc()

			slog.Debug("We sent new cached image", "req_id", reqID, "err", err)
			return
		} else {
			foundPath = hash
		}

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

		targetObject := fmt.Sprintf("%s_%v_%v_%v", hash, width, quality, format)

		var MediaErr *core.MediaError

		buf, formatNew, err := core.ResizeImage(postgreCtx, foundPath, hash, env.OriginalBucket, wid, qual, env.minioClient, format) // formatNew = format, its just for understanding
		if err == nil {
			log.Printf("We made new cashed image: %v\n", targetObject)
		} else {
			if errors.As(err, &MediaErr) {
				errorsOfService.Inc()
				log.Printf("Logged for system admin [%s] %s\n", reqID, MediaErr.Error())
				http.Error(w, MediaErr.Message, MediaErr.Code)
				return
			}
		}
		opts := minio.PutObjectOptions{
			ContentType: "image/" + format,
		}

		if _, err = env.minioClient.PutObject(postgreCtx, env.CachedBucket, targetObject, bytes.NewReader(buf), int64(len(buf)), opts); err != nil {
			slog.Error("ERR at saving data into storage", "req_id", reqID, "err", err)
			http.Error(w, "Unexpected error, tell your REQ-ID to our support", 500)
			return
		}

		slog.Debug("We sent newly optimized image", "req_id", reqID)

		err = database.SaveCashedAfterGetRequest(postgreCtx, id, width, quality, targetObject, formatNew, int(time.Now().Unix()))
		if errors.As(err, &dbError) {
			errorsOfService.Inc()
			log.Printf("ERR [%s] %v\n", reqID, dbError.Error())
			return
		}

		key := fmt.Sprintf("%v_%v_%v_%v", hash, width, quality, format)
		mu.Lock()
		AlreadyDone[key] = env.CachedBucket
		mu.Unlock()

		w.Header().Set("Content-Type", "image/"+format)
		w.Header().Set("Content-Length", strconv.Itoa(len(buf)))
		w.WriteHeader(http.StatusOK)
		w.Write(buf)

		successfulTotals.Inc()

	} else if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

		err = r.ParseMultipartForm(10 << 20)
		if err != nil {
			errorsOfService.Inc()
			log.Printf("ERR at parsing Multipart Form [%s] %v\n", reqID, err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		network, _, err := r.FormFile("image") // gets all formed data from 'image' block
		if err != nil {
			errorsOfService.Inc()
			slog.Error("ERR at forming file by keyword", "req_iq", reqID, "err", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer network.Close()

		hasher := sha256.New()

		var buf bytes.Buffer

		mw := io.MultiWriter(&buf, hasher)

		var mediaErr *core.MediaError

		stream, format, err := core.Detector(network)
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

		opts := minio.PutObjectOptions{
			ContentType: "image/" + format,
		}

		if _, err = env.minioClient.PutObject(postgreCtx, env.OriginalBucket, hashString, &buf, int64(buf.Len()), opts); err != nil {
			slog.Error("ERR at putting object in the minIO storage after post request", "req_id", reqID, "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var dbError *database.DBError

		err = database.SaveOriginalAfterPostRequest(postgreCtx, hashString, format, int(time.Now().Unix()))
		if err == nil {
			log.Printf("New picture by [%s]: %s\n", reqID, hashString)
			mu.Lock()
			keyword := fmt.Sprint(hashString + ":" + "original-quality" + ":" + "original-width" + ":" + format)
			AlreadyDone[keyword] = env.OriginalBucket
			mu.Unlock()
		} else if errors.As(err, &dbError) {
			errorsOfService.Inc()
			log.Printf("ERR [%s] %v\n", reqID, dbError.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		successfulTotals.Inc() // prometheus metric

		w.Header().Set("Content-Type", "image/"+format)
		w.WriteHeader(http.StatusOK)
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

func (env *MinIOEnv) LogMiddleware(nextFunc http.Handler) http.Handler {
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
