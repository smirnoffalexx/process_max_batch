package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Channel is used for stopping goroutine with ProcessQueue
var Channel chan struct{}

// Queue is a queue for batch items, which is not processed yet
var Queue SyncQueue

// TotalProcessed is a number of processed items
var TotalProcessed int

// SyncQueue is a struct for sync queue
type SyncQueue struct {
	Batch Batch
	Mutex sync.Mutex
}

// Batch is a batch of items.
type Batch []Item

// Item is some abstract item.
type Item struct{}

// ProcessRequest is a request data for process request
type ProcessRequest struct {
	Data Batch `json:"data"`
}

// ProcessResponse is a response data for process request
type ProcessResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// ProcessResponse is a response data for limits request
type LimitsResponse struct {
	Number   uint64        `json:"number"`
	Duration time.Duration `json:"duration"`
}

// TotalProcessedResponse is a response data for total request
type TotalProcessedResponse struct {
	TotalProcessed int
}

func main() {
	log.Logger = zerolog.New(os.Stdout).With().Stack().Timestamp().Logger()
	log.Info().Msg("Client started")

	Channel = make(chan struct{})

	RunProcess()

	SetupRoutes()
}

// SetupRoutes runs REST API for client
func SetupRoutes() {
	r := gin.New()
	public := r.Group("client")
	public.POST("/process", HandleProcess)
	public.GET("/total", HandleTotalProcessed)
	public.POST("/stop", HandleStop)

	s := &http.Server{
		Addr:         "0.0.0.0:8081",
		ReadTimeout:  100 * time.Second,
		WriteTimeout: 100 * time.Second,
		Handler:      r,
	}

	if err := s.ListenAndServe(); err != nil {
		log.Error().Err(err).Msg("")
	}
}

// HandleStop is used for stopping RunProcess goroutine via channel
func HandleStop(c *gin.Context) {
	Channel <- struct{}{}

	c.JSON(
		http.StatusOK,
		gin.H{"success": true},
	)
}

// HandleTotalProcessed is used to see amount of total processed items
func HandleTotalProcessed(c *gin.Context) {
	c.JSON(
		http.StatusOK,
		TotalProcessedResponse{
			TotalProcessed: TotalProcessed,
		},
	)
}

// HandleProcess is used for adding new items to the processing queue
func HandleProcess(c *gin.Context) {
	var data ProcessRequest
	if err := c.ShouldBindJSON(&data); err != nil {
		log.Error().Err(err).Msg("")
		c.JSON(http.StatusBadRequest,
			ProcessResponse{
				Success: false,
				Error:   err.Error(),
			},
		)
		return
	}

	if len(data.Data) == 0 {
		c.JSON(http.StatusBadRequest,
			ProcessResponse{
				Success: false,
				Error:   "No items provided",
			},
		)
		return
	}

	Queue.Mutex.Lock()
	Queue.Batch = append(Queue.Batch, data.Data...)
	Queue.Mutex.Unlock()

	c.JSON(http.StatusOK, ProcessResponse{
		Success: true,
	})
}

// RunProcess gets server limits and starts the goroutine with infinity processing loop
func RunProcess() {
	req, err := http.NewRequest(http.MethodGet, "http://0.0.0.0:8080/server/limits", nil)
	if err != nil {
		log.Error().Err(err).Msg("")
		return
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("")
		return
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Err(err).Msg("")
		return
	}

	var limits LimitsResponse
	if err := json.Unmarshal(body, &limits); err != nil {
		log.Error().Err(err).Msg("")
		return
	}

	processedItems := make(map[int64]int)
	errorsCount := 0

	go func() {
		for {
			select {
			case <-Channel:
				log.Info().Msg("RunProcess has been stopped")
				return
			default:
				if err := ProcessQueue(client, &limits, processedItems); err != nil {
					errorsCount++
					log.Error().Err(err).Msg("")
				} else {
					errorsCount = 0
				}

				if errorsCount >= 10 {
					log.Info().Msg("ProcessQueue returned 10 errors in a row")
					return
				}
			}
		}
	}()
}

// ProcessQueue is called for sending queued items to server if channel is not closed
func ProcessQueue(client *http.Client, limits *LimitsResponse, processedItems map[int64]int) error {
	if len(Queue.Batch) == 0 {
		return nil
	}

	limit := int(limits.Number)

	for timestamp, item := range processedItems {
		if time.Now().UnixNano() > timestamp+int64(limits.Duration) {
			delete(processedItems, timestamp)
		} else {
			limit -= item
		}
	}

	if limit <= 0 {
		return nil
	}

	Queue.Mutex.Lock()
	if len(Queue.Batch) < limit {
		limit = len(Queue.Batch)
	}

	jsonData, err := json.Marshal(
		ProcessRequest{
			Data: Queue.Batch[:limit],
		},
	)
	Queue.Mutex.Unlock()
	if err != nil {
		return err
	}

	processedItems[time.Now().UnixNano()] = limit

	req, err := http.NewRequest(http.MethodPost, "http://0.0.0.0:8080/server/process", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var process ProcessResponse
	if err := json.Unmarshal(body, &process); err != nil {
		return err
	}

	if !process.Success {
		return errors.New("Error response from server: " + process.Error)
	}

	Queue.Mutex.Lock()
	if len(Queue.Batch) <= limit {
		Queue.Batch = []Item{}
	} else {
		Queue.Batch = Queue.Batch[limit:]
	}
	Queue.Mutex.Unlock()

	TotalProcessed += limit

	log.Info().Msg("Processed " + strconv.FormatInt(int64(limit), 10) + " items")

	return nil
}
