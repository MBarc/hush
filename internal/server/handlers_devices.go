package server

import (
	"fmt"
	"net/http"

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

func (s *Server) handleDeviceUnblock(w http.ResponseWriter, r *http.Request) {
	hostname := r.PathValue("hostname")
	if err := s.st.UnblockDevice(hostname); err != nil {
		storeError(w, err)
		return
	}
	s.audit(r, "device.unblock", "", "hostname="+hostname)
	writeJSON(w, http.StatusOK, map[string]string{"hostname": hostname, "status": "unblocked"})
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

// handleDeviceUpdate patches a device's label and/or write access. Only the
// fields present in the body are changed.
func (s *Server) handleDeviceUpdate(w http.ResponseWriter, r *http.Request) {
	hostname := r.PathValue("hostname")
	var req struct {
		Label      *string `json:"label"`
		AllowWrite *bool   `json:"allowWrite"`
	}
	if err := readJSON(r, &req); err != nil {
		httpError(w, http.StatusBadRequest, "bad request body")
		return
	}
	if req.Label != nil {
		if err := s.st.SetDeviceLabel(hostname, *req.Label); err != nil {
			storeError(w, err)
			return
		}
		s.audit(r, "device.name", "", fmt.Sprintf("hostname=%s label=%q", hostname, *req.Label))
	}
	if req.AllowWrite != nil {
		if err := s.st.SetDeviceWrite(hostname, *req.AllowWrite); err != nil {
			storeError(w, err)
			return
		}
		s.audit(r, "device.write", "", fmt.Sprintf("hostname=%s allowWrite=%v", hostname, *req.AllowWrite))
	}
	writeJSON(w, http.StatusOK, map[string]string{"hostname": hostname, "status": "updated"})
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
