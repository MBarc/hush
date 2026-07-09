package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/MBarc/hush/internal/store"
)

func (s *Server) handleDeviceList(w http.ResponseWriter, r *http.Request) {
	devices, err := s.st.ListDevices()
	if err != nil {
		storeError(w, err)
		return
	}
	if devices == nil {
		devices = []store.Device{}
	}
	writeJSON(w, http.StatusOK, devices)
}

func (s *Server) handleDeviceTrust(w http.ResponseWriter, r *http.Request) {
	hostname := r.PathValue("hostname")
	var req struct {
		Scopes     []string `json:"scopes"`
		AllowWrite bool     `json:"allowWrite"`
		TTLDays    int      `json:"ttlDays"`
	}
	if err := readJSON(r, &req); err != nil {
		httpError(w, http.StatusBadRequest, "bad request body")
		return
	}
	var expiresAt int64
	if req.TTLDays > 0 {
		expiresAt = time.Now().Add(time.Duration(req.TTLDays) * 24 * time.Hour).Unix()
	}
	if err := s.st.SetDeviceTrust(hostname, store.DeviceTrusted, req.Scopes, req.AllowWrite, expiresAt); err != nil {
		storeError(w, err)
		return
	}
	s.audit(r, "device.trust", "", fmt.Sprintf("hostname=%s allowWrite=%v ttlDays=%d",
		hostname, req.AllowWrite, req.TTLDays))
	writeJSON(w, http.StatusOK, map[string]any{
		"hostname": hostname, "status": store.DeviceTrusted,
		"allowWrite": req.AllowWrite, "expiresAt": expiresAt,
	})
}

// --- device access grants (managed from the folder/secret side) ---

func (s *Server) handlePathGrants(w http.ResponseWriter, r *http.Request) {
	devices, err := s.st.DevicesForPath(r.PathValue("path"))
	if err != nil {
		storeError(w, err)
		return
	}
	if devices == nil {
		devices = []store.DeviceAccess{}
	}
	writeJSON(w, http.StatusOK, devices)
}

func (s *Server) handlePathGrantAdd(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	var req struct {
		Hostname string `json:"hostname"`
	}
	if err := readJSON(r, &req); err != nil {
		httpError(w, http.StatusBadRequest, "bad request body")
		return
	}
	if err := s.st.GrantDevice(req.Hostname, path); err != nil {
		storeError(w, err)
		return
	}
	s.audit(r, "device.grant", path, "hostname="+req.Hostname)
	writeJSON(w, http.StatusCreated, map[string]string{"hostname": req.Hostname, "path": path})
}

func (s *Server) handlePathGrantRemove(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	hostname := r.URL.Query().Get("hostname")
	if hostname == "" {
		httpError(w, http.StatusBadRequest, "hostname query parameter required")
		return
	}
	if err := s.st.RevokeDeviceGrant(hostname, path); err != nil {
		storeError(w, err)
		return
	}
	s.audit(r, "device.revoke", path, "hostname="+hostname)
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (s *Server) handleDeviceName(w http.ResponseWriter, r *http.Request) {
	hostname := r.PathValue("hostname")
	var req struct {
		Label string `json:"label"`
	}
	if err := readJSON(r, &req); err != nil {
		httpError(w, http.StatusBadRequest, "bad request body")
		return
	}
	if err := s.st.SetDeviceLabel(hostname, req.Label); err != nil {
		storeError(w, err)
		return
	}
	s.audit(r, "device.name", "", fmt.Sprintf("hostname=%s label=%q", hostname, req.Label))
	writeJSON(w, http.StatusOK, map[string]string{"hostname": hostname, "label": req.Label})
}

func (s *Server) handleDeviceBlock(w http.ResponseWriter, r *http.Request) {
	hostname := r.PathValue("hostname")
	if err := s.st.SetDeviceTrust(hostname, store.DeviceBlocked, nil, false, 0); err != nil {
		storeError(w, err)
		return
	}
	s.audit(r, "device.block", "", "hostname="+hostname)
	writeJSON(w, http.StatusOK, map[string]string{"hostname": hostname, "status": store.DeviceBlocked})
}

func (s *Server) handleDeviceDelete(w http.ResponseWriter, r *http.Request) {
	hostname := r.PathValue("hostname")
	if err := s.st.DeleteDevice(hostname); err != nil {
		storeError(w, err)
		return
	}
	s.audit(r, "device.forget", "", "hostname="+hostname)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
