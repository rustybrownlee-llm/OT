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
