package api

import (
	"net/http"
)

// handleAreaPollen serves one area's pollen forecast table. Not breadth-counted
// and publicly cacheable: it is a model field per area, with no sensor data in it.
func (d Deps) handleAreaPollen(w http.ResponseWriter, r *http.Request) {
	snap := d.Snapshots.Load()
	if snap == nil {
		writeUnavailable(w)
		return
	}
	slug := r.PathValue("slug")
	if _, known := snap.KnownSlugs[slug]; !known {
		writeError(w, http.StatusNotFound, "not_found", "No such area.")
		return
	}
	body, ok := snap.PollenBody(slug)
	if !ok {
		writeUnavailable(w)
		return
	}
	serveBody(w, r, body, cachePublic, int(d.Config.Cache.DataMaxAge.Seconds()))
}

// handlePollenMap serves today's worst level per province for the map layer.
// Publicly cacheable for the same reason as the per-area table.
func (d Deps) handlePollenMap(w http.ResponseWriter, r *http.Request) {
	snap := d.Snapshots.Load()
	if snap == nil {
		writeUnavailable(w)
		return
	}
	body, ok := snap.PollenMapBody()
	if !ok {
		writeUnavailable(w)
		return
	}
	serveBody(w, r, body, cachePublic, int(d.Config.Cache.DataMaxAge.Seconds()))
}
