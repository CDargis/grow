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
	plants       *store.PlantStore
	envs         *store.EnvironmentStore
	logs         *store.LogStore
	milestones   *store.MilestoneStore
	observations *store.ObservationStore
	s3           *s3.Client
	presign      *s3.PresignClient
	mediaBkt     string
	userID       string
}

func main() {
	ctx := context.Background()
	clients, err := store.NewClients(ctx)
	if err != nil {
		log.Fatalf("init clients: %v", err)
	}

	a := &app{
		plants: store.NewPlantStore(clients.DDB, os.Getenv("PLANTS_TABLE")),
		envs:   store.NewEnvironmentStore(clients.DDB, os.Getenv("ENVIRONMENTS_TABLE")),
		logs:   store.NewLogStore(clients.DDB, os.Getenv("LOGS_TABLE"), os.Getenv("LOGS_DATE_GSI")),
		milestones: store.NewMilestoneStore(clients.DDB, os.Getenv("MILESTONES_TABLE")),
		observations: store.NewObservationStore(clients.DDB, os.Getenv("OBSERVATIONS_TABLE")),
		s3:           clients.S3,
		presign:      s3.NewPresignClient(clients.S3),
		mediaBkt:     os.Getenv("MEDIA_BUCKET"),
		userID:       getEnvOrDefault("USER_ID", "default"),
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
	mux.HandleFunc("PUT /api/plants/{plantId}/phase",       a.updatePhase)
	mux.HandleFunc("PATCH /api/plants/{plantId}",            a.updatePlant)
	mux.HandleFunc("PUT /api/plants/{plantId}/avatar",      a.updateAvatar)

	mux.HandleFunc("GET /api/plants/{plantId}/logs",     a.listLogs)
	mux.HandleFunc("POST /api/plants/{plantId}/logs",    a.createLog)
	mux.HandleFunc("PATCH /api/plants/{plantId}/logs/{logId}",  a.updateLog)
	mux.HandleFunc("DELETE /api/plants/{plantId}/logs/{logId}", a.deleteLog)

	mux.HandleFunc("GET /api/logs",                      a.listLogsByDate)

	mux.HandleFunc("GET /api/environments",              a.listEnvironments)
	mux.HandleFunc("POST /api/environments",             a.createEnvironment)
	mux.HandleFunc("GET /api/environments/{envId}",      a.getEnvironment)
	mux.HandleFunc("PATCH /api/environments/{envId}",    a.updateEnvironment)
	mux.HandleFunc("DELETE /api/environments/{envId}",   a.deleteEnvironment)

	mux.HandleFunc("POST /api/media/upload",             a.presignUpload)
	mux.HandleFunc("GET /api/media/url",                 a.presignDownload)

	mux.HandleFunc("GET /api/plants/{plantId}/milestones",                     a.listMilestones)
	mux.HandleFunc("PATCH /api/plants/{plantId}/milestones/{milestoneType}",   a.updateMilestone)
	mux.HandleFunc("GET /api/plants/{plantId}/observations", a.listObservations)
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

func (a *app) updatePlant(w http.ResponseWriter, r *http.Request) {
	var req model.UpdatePlantDetailsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, err, http.StatusBadRequest)
		return
	}
	plant, err := a.plants.UpdateDetails(r.Context(), r.PathValue("plantId"), req)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	jsonOK(w, plant)
}

func (a *app) updateAvatar(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AvatarKey string `json:"avatarKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, err, http.StatusBadRequest)
		return
	}
	if err := a.plants.UpdateAvatar(r.Context(), r.PathValue("plantId"), req.AvatarKey); err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *app) updatePhase(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phase    model.PlantPhase `json:"phase"`
		Date     string           `json:"date"`
		LoggedAt string           `json:"loggedAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, err, http.StatusBadRequest)
		return
	}
	now := time.Now().UTC()
	date := req.Date
	if date == "" {
		date = now.Format("2006-01-02")
	}
	plantID := r.PathValue("plantId")
	fromPhase, err := a.plants.UpdatePhase(r.Context(), plantID, req.Phase, date)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	phaseData := model.PhaseChangeData{ToPhase: req.Phase}
	if date == now.Format("2006-01-02") {
		phaseData.FromPhase = fromPhase
	}
	dataBytes, _ := json.Marshal(phaseData)
	if _, err := a.logs.Create(r.Context(), plantID, a.userID, model.CreateLogRequest{
		LogType:  model.LogPhaseChange,
		Date:     date,
		LoggedAt: req.LoggedAt,
		Data:     json.RawMessage(dataBytes),
	}); err != nil {
		log.Printf("warn: phase_change log: %v", err)
	}
	w.WriteHeader(http.StatusNoContent)
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

func (a *app) updateLog(w http.ResponseWriter, r *http.Request) {
	var req model.CreateLogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, err, http.StatusBadRequest)
		return
	}
	entry, err := a.logs.Update(r.Context(), r.PathValue("plantId"), r.PathValue("logId"), a.userID, req)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
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

func (a *app) updateEnvironment(w http.ResponseWriter, r *http.Request) {
	var req model.CreateEnvironmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, err, http.StatusBadRequest)
		return
	}
	envID := r.PathValue("envId")
	oldSchedule, env, err := a.envs.Update(r.Context(), envID, req)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	if oldSchedule != env.LightSchedule {
		plants, err := a.plants.List(r.Context(), a.userID)
		if err != nil {
			log.Printf("warn: list plants for lighting change: %v", err)
		} else {
			now := time.Now().UTC()
			date := now.Format("2006-01-02")
			loggedAt := now.Format(time.RFC3339)
			data := model.LightingChangeData{FromSchedule: oldSchedule, ToSchedule: env.LightSchedule}
			dataBytes, _ := json.Marshal(data)
			for _, p := range plants {
				if p.EnvironmentID != envID {
					continue
				}
				if _, err := a.logs.Create(r.Context(), p.PlantID, a.userID, model.CreateLogRequest{
					LogType:  model.LogLightingChange,
					Date:     date,
					LoggedAt: loggedAt,
					Data:     json.RawMessage(dataBytes),
				}); err != nil {
					log.Printf("warn: lighting_change log for plant %s: %v", p.PlantID, err)
				}
			}
		}
	}
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

func (a *app) presignDownload(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		httpError(w, fmt.Errorf("key required"), http.StatusBadRequest)
		return
	}
	presigned, err := a.presign.PresignGetObject(r.Context(), &s3.GetObjectInput{
		Bucket: aws.String(a.mediaBkt),
		Key:    aws.String(key),
	}, func(o *s3.PresignOptions) { o.Expires = time.Hour })
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"url": presigned.URL})
}

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

// ── Milestones ────────────────────────────────────────────────────────────────

func (a *app) listMilestones(w http.ResponseWriter, r *http.Request) {
	milestones, err := a.milestones.ListForPlant(r.Context(), r.PathValue("plantId"))
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	jsonOK(w, milestones)
}

func (a *app) updateMilestone(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action        string `json:"action"` // "confirm" | "skip"
		ConfirmedDate string `json:"confirmedDate,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, err, http.StatusBadRequest)
		return
	}
	plantID       := r.PathValue("plantId")
	milestoneType := model.MilestoneType(r.PathValue("milestoneType"))

	switch req.Action {
	case "confirm":
		date := req.ConfirmedDate
		if date == "" {
			date = time.Now().UTC().Format("2006-01-02")
		}
		if err := a.milestones.Confirm(r.Context(), plantID, milestoneType, date); err != nil {
			httpError(w, err, http.StatusInternalServerError)
			return
		}
	case "skip":
		if err := a.milestones.Skip(r.Context(), plantID, milestoneType); err != nil {
			httpError(w, err, http.StatusInternalServerError)
			return
		}
	default:
		httpError(w, fmt.Errorf("unknown action: %s", req.Action), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Observations ──────────────────────────────────────────────────────────────

func (a *app) listObservations(w http.ResponseWriter, r *http.Request) {
	obs, err := a.observations.ListForPlant(r.Context(), r.PathValue("plantId"))
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	jsonOK(w, obs)
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
