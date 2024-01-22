package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Max batch length for processing
var BatchLengthLimit uint64

// Duration for batch limit
var Duration time.Duration

// ErrBlocked reports if service is blocked.
var ErrBlocked = errors.New("blocked")

// Service defines external service that can process batches of items.
type Service interface {
	GetLimits() (n uint64, p time.Duration)
	Process(ctx context.Context, batch Batch) error
}

// Batch is a batch of items.
type Batch []Item

// Item is some abstract item.
type Item struct{}

type GetLimitsResponse struct {
	Number   uint64        `json:"number"`
	Duration time.Duration `json:"duration"`
}

type ProcessRequest struct {
	Data Batch `json:"data"`
}

type ProcessResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

func (b Batch) GetLimits() (n uint64, p time.Duration) {
	return BatchLengthLimit, Duration
}

// Process is a mock function
func (b Batch) Process(ctx context.Context, batch Batch) error {
	if len(batch) > int(BatchLengthLimit) {
		return ErrBlocked
	}

	return nil
}

func main() {
	log.Logger = zerolog.New(os.Stdout).With().Stack().Timestamp().Logger()
	log.Info().Msg("Mock server started")

	Duration = 15 * time.Second
	BatchLengthLimit = uint64(10)

	r := gin.New()
	public := r.Group("server")
	public.GET("/limits", HandleGetLimits)
	public.POST("/process", HandleProcess)

	s := &http.Server{
		Addr:         "0.0.0.0:8080",
		ReadTimeout:  100 * time.Second,
		WriteTimeout: 100 * time.Second,
		Handler:      r,
	}

	if err := s.ListenAndServe(); err != nil {
		log.Error().Err(err).Msg("")
	}
}

func HandleGetLimits(c *gin.Context) {
	p, n := GetLimits(Batch{})

	c.JSON(http.StatusOK, GetLimitsResponse{
		Number:   p,
		Duration: n,
	})
}

func HandleProcess(c *gin.Context) {
	var data ProcessRequest
	if err := c.ShouldBindJSON(&data); err != nil {
		ErrorResponse(c, err)
		return
	}

	if err := Process(Batch{}, data.Data); err != nil {
		ErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, ProcessResponse{
		Success: true,
	})
}

func Process(s Service, batch Batch) error {
	return s.Process(context.Background(), batch)
}

func GetLimits(s Service) (uint64, time.Duration) {
	return s.GetLimits()
}

func ErrorResponse(c *gin.Context, err error) {
	log.Error().Err(err).Msg("")
	c.JSON(http.StatusBadRequest, ProcessResponse{
		Success: false,
		Error:   err.Error(),
	})
}
