package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	"github.com/cdargis/grow/internal/model"
	"github.com/cdargis/grow/internal/store"
	"github.com/oklog/ulid/v2"
)

type app struct {
	plants   *store.PlantStore
	envs     *store.EnvironmentStore
	logs     *store.LogStore
	s3       *s3.Client
	presign  *s3.PresignClient
	mediaBkt string
	userID   string
}

func main() {
	ctx := context.Background()
	clients, err := store.NewClients(ctx)
	if err != nil {
		log.Fatalf("init clients: %v", err)
	}

	a := &app{
		plants:   store.NewPlantStore(clients.DDB, os.Getenv("PLANTS_TABLE")),
		envs:     store.NewEnvironmentStore(clients.DDB, os.Getenv("ENVIRONMENTS_TABLE")),
		logs:     store.NewLogStore(clients.DDB, os.Getenv("LOGS_TABLE"), os.Getenv("LOGS_DATE_GSI")),
		s3:       clients.S3,
		presign:  s3.NewPresignClient(clients.S3),
		mediaBkt: os.Getenv("MEDIA_BUCKET"),
		userID:   getEnvOrDefault("USER_ID", "default"),
	}

	mux := http.NewServeMux()
	a.registerRoutes(mux)

	lambda.Start(httpadapter.NewV2(mux).ProxyWithContext)
}

func (a *app) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/plants",                    a.listPlants)
	mux.HandleFunc("POST /api/plants",                   a.createPlant)
	mux.HandleFunc("GET /api/plants/{plantId}",          a.getPlant)
	mux.HandleFunc("DELETE /api/plants/{plantId}",       a.deletePlant)
	mux.HandleFunc("PUT /api/plants/{plantId}/environment", a.assignEnvironment)

	mux.HandleFunc("GET /api/plants/{plantId}/logs",     a.listLogs)
	mux.HandleFunc("POST /api/plants/{plantId}/logs",    a.createLog)
	mux.HandleFunc("DELETE /api/plants/{plantId}/logs/{logId}", a.deleteLog)

	mux.HandleFunc("GET /api/logs",                      a.listLogsByDate)

	mux.HandleFunc("GET /api/environments",              a.listEnvironments)
	mux.HandleFunc("POST /api/environments",             a.createEnvironment)
	mux.HandleFunc("GET /api/environments/{envId}",      a.getEnvironment)
	mux.HandleFunc("DELETE /api/environments/{envId}",   a.deleteEnvironment)

	mux.HandleFunc("POST /api/media/upload",             a.presignUpload)
}

// ── Plants ───────────────────────────────────────────────────────────────────

func (a *app) listPlants(w http.ResponseWriter, r *http.Request) {
	plants, err := a.plants.List(r.Context(), a.userID)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	jsonOK(w, plants)
}

func (a *app) getPlant(w http.ResponseWriter, r *http.Request) {
	plant, err := a.plants.Get(r.Context(), r.PathValue("plantId"))
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	if plant == nil {
		http.NotFound(w, r)
		return
	}
	jsonOK(w, plant)
}

func (a *app) createPlant(w http.ResponseWriter, r *http.Request) {
	var req model.CreatePlantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, err, http.StatusBadRequest)
		return
	}
	plant, err := a.plants.Create(r.Context(), a.userID, req)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, plant)
}

func (a *app) assignEnvironment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EnvironmentID *string `json:"environmentId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, err, http.StatusBadRequest)
		return
	}
	plantID := r.PathValue("plantId")
	fromEnvID, err := a.plants.AssignEnvironment(r.Context(), plantID, req.EnvironmentID)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	data := model.EnvironmentChangeData{FromEnvironmentID: fromEnvID}
	if req.EnvironmentID != nil && *req.EnvironmentID != "" {
		data.ToEnvironmentID = *req.EnvironmentID
	}
	dataBytes, _ := json.Marshal(data)
	if _, err := a.logs.Create(r.Context(), plantID, a.userID, model.CreateLogRequest{
		LogType: model.LogEnvironmentChange,
		Data:    json.RawMessage(dataBytes),
	}); err != nil {
		log.Printf("warn: environment_change log: %v", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *app) deletePlant(w http.ResponseWriter, r *http.Request) {
	if err := a.plants.Delete(r.Context(), r.PathValue("plantId")); err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Logs ─────────────────────────────────────────────────────────────────────

func (a *app) listLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := a.logs.ListForPlant(r.Context(), r.PathValue("plantId"))
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	jsonOK(w, logs)
}

func (a *app) listLogsByDate(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}
	logs, err := a.logs.ListForDate(r.Context(), a.userID, date)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	jsonOK(w, logs)
}

func (a *app) createLog(w http.ResponseWriter, r *http.Request) {
	var req model.CreateLogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, err, http.StatusBadRequest)
		return
	}
	entry, err := a.logs.Create(r.Context(), r.PathValue("plantId"), a.userID, req)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, entry)
}

func (a *app) deleteLog(w http.ResponseWriter, r *http.Request) {
	if err := a.logs.Delete(r.Context(), r.PathValue("plantId"), r.PathValue("logId")); err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Environments ──────────────────────────────────────────────────────────────

func (a *app) listEnvironments(w http.ResponseWriter, r *http.Request) {
	envs, err := a.envs.List(r.Context(), a.userID)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	jsonOK(w, envs)
}

func (a *app) getEnvironment(w http.ResponseWriter, r *http.Request) {
	env, err := a.envs.Get(r.Context(), r.PathValue("envId"))
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	if env == nil {
		http.NotFound(w, r)
		return
	}
	jsonOK(w, env)
}

func (a *app) createEnvironment(w http.ResponseWriter, r *http.Request) {
	var req model.CreateEnvironmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, err, http.StatusBadRequest)
		return
	}
	env, err := a.envs.Create(r.Context(), a.userID, req)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, env)
}

func (a *app) deleteEnvironment(w http.ResponseWriter, r *http.Request) {
	if err := a.envs.Delete(r.Context(), r.PathValue("envId")); err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Media ─────────────────────────────────────────────────────────────────────

func (a *app) presignUpload(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Prefix      string `json:"prefix"`      // e.g. "plants/01JXABC/avatar"
		ContentType string `json:"contentType"` // e.g. "image/jpeg"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, err, http.StatusBadRequest)
		return
	}
	key := fmt.Sprintf("%s/%s", req.Prefix, ulid.Make().String())
	presigned, err := a.presign.PresignPutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      aws.String(a.mediaBkt),
		Key:         aws.String(key),
		ContentType: aws.String(req.ContentType),
	}, func(o *s3.PresignOptions) { o.Expires = 15 * time.Minute })
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{
		"uploadUrl": presigned.URL,
		"key":       key,
	})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, err error, code int) {
	log.Printf("error %d: %v", code, err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
