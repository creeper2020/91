package api

import (
	"errors"
	"io"
	"net/http"

	"github.com/video-site/backend/internal/config"
)

const maxConfigYAMLBytes = 2 << 20

func (a *AdminServer) previewEnabled() bool {
	return a.ConfigManager.LiveSettings().PreviewEnabled
}

func (a *AdminServer) requirePreviewEnabled(w http.ResponseWriter) bool {
	if !a.previewEnabled() {
		http.Error(w, "全局预览视频生成已关闭", http.StatusConflict)
		return false
	}
	return true
}

func (a *AdminServer) handleGetConfigYAML(w http.ResponseWriter, _ *http.Request) {
	if a.ConfigManager == nil {
		writeErr(w, http.StatusServiceUnavailable, errors.New("configuration manager is unavailable"))
		return
	}
	data, version, err := a.ConfigManager.ReadYAML()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("ETag", `"`+version+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (a *AdminServer) handlePutConfigYAML(w http.ResponseWriter, r *http.Request) {
	if a.ConfigManager == nil {
		writeErr(w, http.StatusServiceUnavailable, errors.New("configuration manager is unavailable"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxConfigYAMLBytes)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeErr(w, http.StatusRequestEntityTooLarge, errors.New("config.yaml exceeds 2 MiB"))
			return
		}
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	expectedVersion := r.Header.Get("If-Match")
	if expectedVersion == "*" {
		expectedVersion = ""
	}
	result, err := a.ConfigManager.ReplaceYAML(data, expectedVersion)
	if err != nil {
		switch {
		case errors.Is(err, config.ErrVersionConflict):
			writeErr(w, http.StatusConflict, err)
		case errors.Is(err, config.ErrInvalidNightlyStartTime),
			errors.Is(err, config.ErrInvalidNightlyTimezone):
			writeErr(w, http.StatusBadRequest, err)
		default:
			// YAML syntax and type errors are validation failures too. Disk I/O
			// errors retain a 500 response; the parser prefixes its errors.
			if configYAMLInvalid(data) {
				writeErr(w, http.StatusBadRequest, err)
			} else {
				writeErr(w, http.StatusInternalServerError, err)
			}
		}
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("ETag", `"`+result.Version+`"`)
	writeJSON(w, http.StatusOK, result)
}

func configYAMLInvalid(data []byte) bool {
	_, err := config.Parse(data)
	return err != nil
}
