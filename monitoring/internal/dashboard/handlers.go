package dashboard

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// overviewHandler renders the full overview page.
func (d *Dashboard) overviewHandler(w http.ResponseWriter, r *http.Request) {
	data := d.buildOverviewData()
	d.render(w, "overview.html", data)
}

// overviewCardsHandler returns the HTMX partial for auto-refreshing summary cards.
func (d *Dashboard) overviewCardsHandler(w http.ResponseWriter, r *http.Request) {
	data := d.buildOverviewData()
	d.renderPartial(w, "overview_cards_content", data)
}

// assetsHandler renders the full asset inventory page.
func (d *Dashboard) assetsHandler(w http.ResponseWriter, r *http.Request) {
	data := d.buildAssetsData()
	d.render(w, "assets.html", data)
}

// assetsTableHandler returns the HTMX partial for the asset table.
func (d *Dashboard) assetsTableHandler(w http.ResponseWriter, r *http.Request) {
	data := d.buildAssetsData()
	d.renderPartial(w, "assets_table_content", data)
}

// assetDetailHandler renders the device detail page for a single asset.
func (d *Dashboard) assetDetailHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	data := d.buildAssetDetailData(id)
	if data == nil {
		http.NotFound(w, r)
		return
	}
	d.render(w, "asset_detail.html", data)
}

// assetRegistersHandler returns the HTMX partial for live register values.
func (d *Dashboard) assetRegistersHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	data := d.buildRegisterData(id)
	d.renderPartial(w, "asset_registers_content", data)
}

// alertsHandler renders the full alerts page.
func (d *Dashboard) alertsHandler(w http.ResponseWriter, r *http.Request) {
	data := d.buildAlertsData(r)
	d.render(w, "alerts.html", data)
}

// alertsTableHandler returns the HTMX partial for the alerts table.
func (d *Dashboard) alertsTableHandler(w http.ResponseWriter, r *http.Request) {
	data := d.buildAlertsData(r)
	d.renderPartial(w, "alerts_table_content", data)
}

// alertAckHandler proxies a POST to the alert acknowledge API.
func (d *Dashboard) alertAckHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := d.api.AcknowledgeAlert(id); err != nil {
		slog.Warn("acknowledge alert failed", "id", id, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `<span class="badge bg-danger">Error</span>`)
		return
	}
	fmt.Fprintf(w, `<span class="badge bg-secondary">Acknowledged</span>`)
}

// eventsHandler renders the full transaction events page.
// GET /events
func (d *Dashboard) eventsHandler(w http.ResponseWriter, r *http.Request) {
	data := d.buildEventsData(r)
	d.render(w, "events.html", data)
}

// eventsTableHandler returns the HTMX partial for the events table.
// GET /partials/events-table
func (d *Dashboard) eventsTableHandler(w http.ResponseWriter, r *http.Request) {
	data := d.buildEventsData(r)
	d.renderPartial(w, "events_table_content", data)
}

// commsHandler renders the full communication matrix page.
// GET /comms
func (d *Dashboard) commsHandler(w http.ResponseWriter, r *http.Request) {
	data := d.buildCommsData()
	d.render(w, "comms.html", data)
}

// commsTableHandler returns the HTMX partial for the communication matrix table.
// GET /partials/comms-table
func (d *Dashboard) commsTableHandler(w http.ResponseWriter, r *http.Request) {
	data := d.buildCommsData()
	d.renderPartial(w, "comms_table_content", data)
}

// designDevicesHandler renders the device library listing.
func (d *Dashboard) designDevicesHandler(w http.ResponseWriter, r *http.Request) {
	data := d.buildDesignDevicesData()
	d.render(w, "design_devices.html", data)
}

// designDeviceDetailHandler renders the detail page for one device atom.
func (d *Dashboard) designDeviceDetailHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	atom, ok := d.lib.Devices[id]
	if !ok {
		http.NotFound(w, r)
		return
	}
	liveAssets := d.liveAssetsForDevice(id)
	highlighted := HighlightYAML(d.lib.RawYAML["devices/"+id])
	data := map[string]any{
		"Title":      atom.Device.Model,
		"ActivePage": "design-devices",
		"Atom":       atom,
		"LiveAssets": liveAssets,
		"RawYAML":    highlighted,
	}
	d.render(w, "design_device_detail.html", data)
}

// designNetworksHandler renders the network library listing.
func (d *Dashboard) designNetworksHandler(w http.ResponseWriter, r *http.Request) {
	data := d.buildDesignNetworksData()
	d.render(w, "design_networks.html", data)
}

// designNetworkDetailHandler renders the detail page for one network atom.
func (d *Dashboard) designNetworkDetailHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	atom, ok := d.lib.Networks[id]
	if !ok {
		http.NotFound(w, r)
		return
	}
	envRefs := d.lib.EnvsUsingNetwork(id)
	highlighted := HighlightYAML(d.lib.RawYAML["networks/"+id])
	data := map[string]any{
		"Title":       atom.Network.Name,
		"ActivePage":  "design-networks",
		"Atom":        atom,
		"PurdueLevel": purdueLevel(id),
		"EnvRefs":     envRefs,
		"RawYAML":     highlighted,
	}
	d.render(w, "design_network_detail.html", data)
}

// designEnvsHandler renders the environment library listing.
func (d *Dashboard) designEnvsHandler(w http.ResponseWriter, r *http.Request) {
	data := d.buildDesignEnvsData()
	d.render(w, "design_envs.html", data)
}

// designEnvDetailHandler renders the detail page for one environment.
func (d *Dashboard) designEnvDetailHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	def, ok := d.lib.Environments[id]
	if !ok {
		http.NotFound(w, r)
		return
	}
	portMap := buildPortMap(def)
	highlighted := HighlightYAML(d.lib.RawYAML["environments/"+id])
	data := map[string]any{
		"Title":      def.Env.Name,
		"ActivePage": "design-envs",
		"Def":        def,
		"PortMap":    portMap,
		"RawYAML":    highlighted,
	}
	d.render(w, "design_env_detail.html", data)
}

// topologyHandler renders the full topology page with a default or first environment.
// GET /topology
func (d *Dashboard) topologyHandler(w http.ResponseWriter, r *http.Request) {
	envID := defaultEnvID(d.lib)
	data := d.buildTopologyPageData(envID)
	d.render(w, "topology.html", data)
}

// topologyEnvHandler renders the topology page for a specific environment.
// GET /topology/{env-id}
func (d *Dashboard) topologyEnvHandler(w http.ResponseWriter, r *http.Request) {
	envID := chi.URLParam(r, "env-id")
	if _, ok := d.lib.Environments[envID]; !ok {
		http.NotFound(w, r)
		return
	}
	data := d.buildTopologyPageData(envID)
	d.render(w, "topology.html", data)
}

// topologyPartialHandler returns the HTMX topology-view partial for one environment.
// GET /partials/topology-view/{env-id}
func (d *Dashboard) topologyPartialHandler(w http.ResponseWriter, r *http.Request) {
	envID := chi.URLParam(r, "env-id")
	def := d.lib.Environments[envID]
	td := BuildTopologyData(def, d.lib)
	if td == nil {
		http.NotFound(w, r)
		return
	}
	d.renderPartial(w, "topology_view_content", td)
}

// buildTopologyPageData assembles the topologyPageData for a given environment ID.
func (d *Dashboard) buildTopologyPageData(envID string) topologyPageData {
	def := d.lib.Environments[envID]
	td := BuildTopologyData(def, d.lib)
	if td == nil {
		td = &TopologyData{AllEnvs: buildEnvSummaries(d.lib)}
	}
	return topologyPageData{
		Title:      "Topology",
		ActivePage: "topology",
		Topology:   td,
	}
}

// defaultEnvID returns the first environment ID alphabetically, preferring
// "brownfield-wastewater" if present.
func defaultEnvID(lib *DesignLibrary) string {
	if lib == nil {
		return ""
	}
	if _, ok := lib.Environments["brownfield-wastewater"]; ok {
		return "brownfield-wastewater"
	}
	ids := sortedKeys(lib.Environments)
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

// processHandler renders the full process view page with the default environment.
// GET /process
func (d *Dashboard) processHandler(w http.ResponseWriter, r *http.Request) {
	envID := defaultProcessEnvID(d.lib)
	data := d.buildProcessPageData(envID)
	d.render(w, "process.html", data)
}

// processEnvHandler renders the process view for a specific environment.
// GET /process/{env-id}
func (d *Dashboard) processEnvHandler(w http.ResponseWriter, r *http.Request) {
	envID := chi.URLParam(r, "env-id")
	if _, ok := d.lib.Schematics[envID]; !ok {
		http.NotFound(w, r)
		return
	}
	data := d.buildProcessPageData(envID)
	d.render(w, "process.html", data)
}

// processValuesPartialHandler returns HTMX OOB swap elements for live instrument
// value refresh. Called every 2 seconds by the HTMX polling container in process.html.
// [OT-REVIEW] Guard on schematic map lookup explicitly before calling data builders.
// Return empty partial (not 404) when API is down or returns no data -- stale data
// on the display is preferable to a broken display (OT availability principle).
// A 404 would cause HTMX to stop polling, freezing the live view permanently.
// GET /partials/process-values/{env-id}
// PROTOTYPE-DEBT: [td-dashboard-055] Each 2-second poll makes a fresh BuildProcessViewData
// call, hitting GetAssetRegisters once per unique placement. For greenfield-water-mfg
// (3 PLCs), that is 3 outbound HTTP calls per poll. Acceptable for educational prototype.
// TODO-FUTURE: Add server-side caching with TTL matching the poll interval (Beta 0.6+).
func (d *Dashboard) processValuesPartialHandler(w http.ResponseWriter, r *http.Request) {
	envID := chi.URLParam(r, "env-id")
	sc, ok := d.lib.Schematics[envID]
	if !ok {
		http.NotFound(w, r)
		return
	}
	pv := d.BuildProcessViewData(envID, sc)
	instruments := d.BuildProcessValuesPartial(pv)
	d.renderPartial(w, "svg_instrument_oob", instruments)
}

// buildProcessPageData assembles ProcessViewPageData for the given environment ID.
func (d *Dashboard) buildProcessPageData(envID string) ProcessViewPageData {
	sc := d.lib.Schematics[envID]
	pv := d.BuildProcessViewData(envID, sc)
	return ProcessViewPageData{
		Title:      "Process View",
		ActivePage: "process",
		EnvID:      envID,
		Process:    pv,
		AllEnvs:    buildProcessEnvSummaries(d.lib),
	}
}

// defaultProcessEnvID returns the first environment alphabetically that has a
// process schematic. Prefers "greenfield-water-mfg" if it has a schematic.
// [OT-REVIEW] The process view default prefers "greenfield-water-mfg" because it
// shows a modern Purdue-segmented environment with clean ISA-5.1 tagging and no
// legacy complexity. This is the right starting point for a trainee seeing a process
// view for the first time.
func defaultProcessEnvID(lib *DesignLibrary) string {
	if lib == nil {
		return ""
	}
	if _, ok := lib.Schematics["greenfield-water-mfg"]; ok {
		return "greenfield-water-mfg"
	}
	ids := sortedKeys(lib.Schematics)
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

// buildProcessEnvSummaries returns EnvSummary entries only for environments
// that have a process schematic, sorted alphabetically by ID.
func buildProcessEnvSummaries(lib *DesignLibrary) []EnvSummary {
	if lib == nil {
		return nil
	}
	ids := sortedKeys(lib.Schematics)
	summaries := make([]EnvSummary, 0, len(ids))
	for _, id := range ids {
		env, ok := lib.Environments[id]
		if !ok {
			// [OT-REVIEW] Intentional silent skip: handles edge case where a schematic was
			// loaded for a directory that has no corresponding environment.yaml. This can
			// happen if a process.yaml exists in a directory without a valid environment def.
			// Do NOT remove this guard -- it is not dead code.
			continue
		}
		summaries = append(summaries, EnvSummary{
			ID:        env.Env.ID,
			Name:      env.Env.Name,
			Archetype: env.Env.Archetype,
		})
	}
	return summaries
}
