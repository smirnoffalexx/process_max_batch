package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func SetUpRouter() *gin.Engine {
	router := gin.Default()
	return router
}

func TestHandleTotalProcessed(t *testing.T) {
	r := SetUpRouter()
	r.GET("/total", HandleTotalProcessed)

	req, _ := http.NewRequest("GET", "/total", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	responseData, _ := ioutil.ReadAll(w.Body)
	var totalProcessedResponse TotalProcessedResponse
	json.Unmarshal(responseData, &totalProcessedResponse)

	assert.Equal(t, 0, totalProcessedResponse.TotalProcessed)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEmptyRequestHandleProcess(t *testing.T) {
	r := SetUpRouter()
	r.POST("/process", HandleProcess)

	req, _ := http.NewRequest("POST", "/process", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	responseData, _ := ioutil.ReadAll(w.Body)
	var processResponse ProcessResponse
	json.Unmarshal(responseData, &processResponse)

	assert.Equal(t, false, processResponse.Success)
	assert.Equal(t, "invalid request", processResponse.Error)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNoItemsHandleProcess(t *testing.T) {
	r := SetUpRouter()
	r.POST("/process", HandleProcess)

	data := ProcessRequest{Data: []Item{}}
	jsonData, _ := json.Marshal(data)
	req, _ := http.NewRequest("POST", "/process", bytes.NewBuffer(jsonData))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	responseData, _ := ioutil.ReadAll(w.Body)
	var processResponse ProcessResponse
	json.Unmarshal(responseData, &processResponse)

	assert.Equal(t, false, processResponse.Success)
	assert.Equal(t, "No items provided", processResponse.Error)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSuccessfulItemsProcessing(t *testing.T) {
	r := SetUpRouter()
	r.POST("/process", HandleProcess)
	r.GET("/total", HandleTotalProcessed)

	RunProcess()

	data := ProcessRequest{Data: []Item{{}, {}, {}, {}, {}}}
	jsonData, _ := json.Marshal(data)
	req, _ := http.NewRequest("POST", "/process", bytes.NewBuffer(jsonData))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	responseData, _ := ioutil.ReadAll(w.Body)
	var processResponse ProcessResponse
	json.Unmarshal(responseData, &processResponse)

	assert.Equal(t, true, processResponse.Success)
	assert.Equal(t, "", processResponse.Error)
	assert.Equal(t, http.StatusOK, w.Code)

	time.Sleep(5 * time.Millisecond)

	req, _ = http.NewRequest("GET", "/total", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	responseData, _ = ioutil.ReadAll(w.Body)
	var totalProcessedResponse TotalProcessedResponse
	json.Unmarshal(responseData, &totalProcessedResponse)

	assert.Equal(t, len(data.Data), totalProcessedResponse.TotalProcessed)
}
