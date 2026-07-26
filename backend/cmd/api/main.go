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
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/awslabs/aws-lambda-go-api-proxy/core"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	"github.com/cdargis/grow/internal/mcpserver"
	"github.com/cdargis/grow/internal/model"
	"github.com/cdargis/grow/internal/store"
	"github.com/oklog/ulid/v2"
)

type app struct {
	plants   *store.PlantStore
	envs     *store.EnvironmentStore
	logs     *store.LogStore
	settings *store.SettingsStore
	s3       *s3.Client
	presign  *s3.PresignClient
	mediaBkt string
	mcp      mcpserver.Deps
	// localUserID is only used when running with LOCAL=1, where there's no
	// API Gateway JWT authorizer in front to supply a real userId.
	localUserID string
}

// userID returns the authenticated user's id (the Cognito "sub" claim) from
// the JWT that API Gateway's authorizer validated before invoking this
// Lambda. Falls back to localUserID when running outside API Gateway
// (LOCAL=1 dev server), since there's no authorizer to supply claims there.
func (a *app) userID(r *http.Request) string {
	if reqCtx, ok := core.GetAPIGatewayV2ContextFromContext(r.Context()); ok {
		if reqCtx.Authorizer != nil && reqCtx.Authorizer.JWT != nil {
			if sub := reqCtx.Authorizer.JWT.Claims["sub"]; sub != "" {
				return sub
			}
		}
	}
	return a.localUserID
}

func main() {
	ctx := context.Background()
	clients, err := store.NewClients(ctx)
	if err != nil {
		log.Fatalf("init clients: %v", err)
	}

	a := &app{
		plants:      store.NewPlantStore(clients.DDB, os.Getenv("PLANTS_TABLE")),
		envs:        store.NewEnvironmentStore(clients.DDB, os.Getenv("ENVIRONMENTS_TABLE")),
		logs:        store.NewLogStore(clients.DDB, os.Getenv("LOGS_TABLE"), os.Getenv("LOGS_DATE_GSI"), os.Getenv("LOGS_LOGTYPE_DATE_GSI")),
		settings:    store.NewSettingsStore(clients.DDB, os.Getenv("SETTINGS_TABLE")),
		s3:          clients.S3,
		presign:     s3.NewPresignClient(clients.S3),
		mediaBkt:    os.Getenv("MEDIA_BUCKET"),
		localUserID: getEnvOrDefault("USER_ID", "default"),
	}
	a.mcp = mcpserver.Deps{
		Plants:           a.plants,
		Logs:             a.logs,
		PublicBaseURL:    os.Getenv("PUBLIC_BASE_URL"),
		Region:           os.Getenv("AWS_REGION"),
		UserPoolID:       os.Getenv("USER_POOL_ID"),
		UserPoolClientID: os.Getenv("MCP_USER_POOL_CLIENT_ID"),
	}

	mux := http.NewServeMux()
	a.registerRoutes(mux)

	if os.Getenv("LOCAL") == "1" {
		log.Println("running local HTTP server on :3000")
		log.Fatal(http.ListenAndServe(":3000", mux))
	} else {
		lambda.Start(httpadapter.NewV2(mux).ProxyWithContext)
	}
}

func (a *app) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/plants", a.listPlants)
	mux.HandleFunc("POST /api/plants", a.createPlant)
	mux.HandleFunc("GET /api/plants/{plantId}", a.getPlant)
	mux.HandleFunc("DELETE /api/plants/{plantId}", a.deletePlant)
	mux.HandleFunc("PUT /api/plants/{plantId}/environment", a.assignEnvironment)
	mux.HandleFunc("PUT /api/plants/{plantId}/phase", a.updatePhase)
	mux.HandleFunc("PATCH /api/plants/{plantId}", a.updatePlant)
	mux.HandleFunc("PUT /api/plants/{plantId}/avatar", a.updateAvatar)

	mux.HandleFunc("GET /api/plants/{plantId}/logs", a.listLogs)
	mux.HandleFunc("POST /api/plants/{plantId}/logs", a.createLog)
	mux.HandleFunc("PATCH /api/plants/{plantId}/logs/{logId}", a.updateLog)
	mux.HandleFunc("DELETE /api/plants/{plantId}/logs/{logId}", a.deleteLog)

	mux.HandleFunc("GET /api/logs", a.listLogsByDate)
	mux.HandleFunc("GET /api/logs/last-by-type", a.listLastByType)

	mux.HandleFunc("GET /api/environments", a.listEnvironments)
	mux.HandleFunc("POST /api/environments", a.createEnvironment)
	mux.HandleFunc("GET /api/environments/{envId}", a.getEnvironment)
	mux.HandleFunc("PATCH /api/environments/{envId}", a.updateEnvironment)
	mux.HandleFunc("DELETE /api/environments/{envId}", a.deleteEnvironment)

	mux.HandleFunc("POST /api/media/upload", a.presignUpload)
	mux.HandleFunc("GET /api/media/url", a.presignDownload)

	mux.HandleFunc("GET /api/settings", a.getSettings)
	mux.HandleFunc("PUT /api/settings", a.updateSettings)

	mux.HandleFunc("GET /api/auth-config", a.getAuthConfig)

	// MCP: authenticates itself (see mcpserver package comment), excluded
	// from the API Gateway JWT authorizer at the CDK route level.
	mux.Handle("/api/mcp", a.mcp.Handler())
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", a.mcp.ProtectedResourceMetadata)
}

// ── Auth ──────────────────────────────────────────────────────────────────────

// getAuthConfig is intentionally unauthenticated (excluded from the JWT
// authorizer at the API Gateway route level) -- the frontend needs these
// non-secret values to kick off the Cognito login flow before it has a
// token to present.
func (a *app) getAuthConfig(w http.ResponseWriter, r *http.Request) {
	region := os.Getenv("AWS_REGION")
	jsonOK(w, map[string]string{
		"authority": fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", region, os.Getenv("USER_POOL_ID")),
		"clientId":  os.Getenv("USER_POOL_CLIENT_ID"),
	})
}

// ── Plants ───────────────────────────────────────────────────────────────────

func (a *app) listPlants(w http.ResponseWriter, r *http.Request) {
	plants, err := a.plants.List(r.Context(), a.userID(r))
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
	plant, err := a.plants.Create(r.Context(), a.userID(r), req)
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
	if _, err := a.logs.Create(r.Context(), plantID, a.userID(r), model.CreateLogRequest{
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
	if _, err := a.logs.Create(r.Context(), plantID, a.userID(r), model.CreateLogRequest{
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
	logs, err := a.logs.ListForDate(r.Context(), a.userID(r), date)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	jsonOK(w, logs)
}

func (a *app) listLastByType(w http.ResponseWriter, r *http.Request) {
	logType := r.URL.Query().Get("logType")
	if logType == "" {
		httpError(w, fmt.Errorf("logType required"), http.StatusBadRequest)
		return
	}
	items, err := a.logs.LastByLogType(r.Context(), a.userID(r), logType)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	if items == nil {
		items = []model.LastActivityItem{}
	}
	jsonOK(w, items)
}

func (a *app) createLog(w http.ResponseWriter, r *http.Request) {
	var req model.CreateLogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, err, http.StatusBadRequest)
		return
	}
	entry, err := a.logs.Create(r.Context(), r.PathValue("plantId"), a.userID(r), req)
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
	entry, err := a.logs.Update(r.Context(), r.PathValue("plantId"), r.PathValue("logId"), a.userID(r), req)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	jsonOK(w, entry)
}

func (a *app) deleteLog(w http.ResponseWriter, r *http.Request) {
	deleted, err := a.logs.Delete(r.Context(), r.PathValue("plantId"), r.PathValue("logId"))
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	if deleted != nil {
		if keys := photoKeysForLog(deleted.LogType, deleted.Data); len(keys) > 0 {
			a.deleteMedia(r.Context(), keys)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// photoKeysForLog extracts any S3 media keys referenced by a log entry's
// data payload, so they can be cleaned up when the entry is deleted.
func photoKeysForLog(logType model.LogType, data json.RawMessage) []string {
	switch logType {
	case model.LogPhoto:
		var d model.PhotoData
		if err := json.Unmarshal(data, &d); err != nil {
			return nil
		}
		return d.PhotoKeys
	case model.LogTraining:
		var d model.TrainingData
		if err := json.Unmarshal(data, &d); err != nil || d.PhotoKey == "" {
			return nil
		}
		return []string{d.PhotoKey}
	case model.LogTrimming:
		var d model.TrimmingData
		if err := json.Unmarshal(data, &d); err != nil || d.PhotoKey == "" {
			return nil
		}
		return []string{d.PhotoKey}
	default:
		return nil
	}
}

// deleteMedia best-effort deletes media objects from S3. Failures are
// logged, not surfaced — the log entry itself is already gone, and a
// dangling S3 object is preferable to reporting the delete as failed.
func (a *app) deleteMedia(ctx context.Context, keys []string) {
	objects := make([]s3types.ObjectIdentifier, len(keys))
	for i, k := range keys {
		objects[i] = s3types.ObjectIdentifier{Key: aws.String(k)}
	}
	if _, err := a.s3.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(a.mediaBkt),
		Delete: &s3types.Delete{Objects: objects},
	}); err != nil {
		log.Printf("delete media objects %v: %v", keys, err)
	}
}

// ── Environments ──────────────────────────────────────────────────────────────

func (a *app) listEnvironments(w http.ResponseWriter, r *http.Request) {
	envs, err := a.envs.List(r.Context(), a.userID(r))
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
	env, err := a.envs.Create(r.Context(), a.userID(r), req)
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
	old, env, err := a.envs.Update(r.Context(), envID, req)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}

	lightChanged := old.LightSchedule != env.LightSchedule
	vpdChanged := (old.TargetVPD == nil) != (env.TargetVPD == nil) ||
		(old.TargetVPD != nil && env.TargetVPD != nil && *old.TargetVPD != *env.TargetVPD)

	if lightChanged || vpdChanged {
		plants, err := a.plants.List(r.Context(), a.userID(r))
		if err != nil {
			log.Printf("warn: list plants for env change: %v", err)
		} else {
			now := time.Now().UTC()
			date := now.Format("2006-01-02")
			loggedAt := now.Format(time.RFC3339)

			var lightBytes, vpdBytes []byte
			if lightChanged {
				lightBytes, _ = json.Marshal(model.LightingChangeData{FromSchedule: old.LightSchedule, ToSchedule: env.LightSchedule})
			}
			if vpdChanged {
				vpdBytes, _ = json.Marshal(model.VPDChangeData{FromVPD: old.TargetVPD, ToVPD: env.TargetVPD})
			}

			for _, p := range plants {
				if p.EnvironmentID != envID {
					continue
				}
				if lightChanged {
					if _, err := a.logs.Create(r.Context(), p.PlantID, a.userID(r), model.CreateLogRequest{
						LogType: model.LogLightingChange, Date: date, LoggedAt: loggedAt,
						Data: json.RawMessage(lightBytes),
					}); err != nil {
						log.Printf("warn: lighting_change log for plant %s: %v", p.PlantID, err)
					}
				}
				if vpdChanged {
					if _, err := a.logs.Create(r.Context(), p.PlantID, a.userID(r), model.CreateLogRequest{
						LogType: model.LogVPDChange, Date: date, LoggedAt: loggedAt,
						Data: json.RawMessage(vpdBytes),
					}); err != nil {
						log.Printf("warn: vpd_change log for plant %s: %v", p.PlantID, err)
					}
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

// ── Settings ──────────────────────────────────────────────────────────────────

func (a *app) getSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.settings.Get(r.Context(), a.userID(r))
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	jsonOK(w, settings)
}

func (a *app) updateSettings(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, err, http.StatusBadRequest)
		return
	}
	settings, err := a.settings.Update(r.Context(), a.userID(r), req)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	jsonOK(w, settings)
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
