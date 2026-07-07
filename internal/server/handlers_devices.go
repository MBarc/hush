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
	if len(req.Scopes) == 0 {
		httpError(w, http.StatusBadRequest, "trusted devices require explicit scopes")
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
	s.audit(r, "device.trust", "", fmt.Sprintf("hostname=%s scopes=%v allowWrite=%v ttlDays=%d",
		hostname, req.Scopes, req.AllowWrite, req.TTLDays))
	writeJSON(w, http.StatusOK, map[string]any{
		"hostname": hostname, "status": store.DeviceTrusted,
		"scopes": req.Scopes, "allowWrite": req.AllowWrite, "expiresAt": expiresAt,
	})
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
