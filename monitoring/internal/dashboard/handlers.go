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
