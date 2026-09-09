package api

import (
	"encoding/json"
	"errors"
	"net/http"
)

// settingsDTO 是 GET/PUT /admin/api/settings 的入参/出参。
//
// Application configuration, including preview.enabled, is owned by config.yaml.
// This endpoint only retains database-backed UI preferences.
type settingsDTO struct {
	Theme                   string `json:"theme"`
	AutoGenerateTagsEnabled bool   `json:"autoGenerateTagsEnabled"`
}

func (a *AdminServer) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	theme := "dark"
	if a.GetTheme != nil {
		if v := a.GetTheme(); v != "" {
			theme = v
		}
	}
	autoGenerateTagsEnabled := false
	if a.Catalog != nil {
		enabled, err := a.Catalog.AutoGenerateTagsEnabled(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		autoGenerateTagsEnabled = enabled
	}
	writeJSON(w, http.StatusOK, settingsDTO{
		Theme:                   theme,
		AutoGenerateTagsEnabled: autoGenerateTagsEnabled,
	})
}

func (a *AdminServer) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if _, ok := raw["builtinTagsEnabled"]; ok {
		writeErr(w, http.StatusBadRequest, errors.New("builtinTagsEnabled is managed by config.yaml"))
		return
	}

	if v, ok := raw["theme"]; ok && a.SetTheme != nil {
		var theme string
		if err := json.Unmarshal(v, &theme); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		if theme != "" {
			if err := a.SetTheme(theme); err != nil {
				writeErr(w, http.StatusBadRequest, err)
				return
			}
		}
	}
	if v, ok := raw["autoGenerateTagsEnabled"]; ok && a.Catalog != nil {
		var enabled bool
		if err := json.Unmarshal(v, &enabled); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		if err := a.Catalog.SetAutoGenerateTagsEnabled(r.Context(), enabled); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
	}
	// 回显当前值
	resp := settingsDTO{
		AutoGenerateTagsEnabled: false,
	}
	if a.GetTheme != nil {
		resp.Theme = a.GetTheme()
	}
	if a.Catalog != nil {
		enabled, err := a.Catalog.AutoGenerateTagsEnabled(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		resp.AutoGenerateTagsEnabled = enabled
	}
	writeJSON(w, http.StatusOK, resp)
}
