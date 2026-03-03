package dashboard

import (
	"context"
	"fmt"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/eventstore"
)

// commsDeviceRow is a display-ready row for the communication matrix table.
// FCCodeList is pre-formatted for template rendering.
type commsDeviceRow struct {
	DeviceID    string
	DeviceName  string
	TotalEvents int64
	WriteEvents int64
	LastSeen    string // formatted: "2006-01-02 15:04:05"
	FCCodeList  string // e.g., "FC1, FC3" or "FC3, FC6, FC16"
}

// commsPageData is the template data for the /comms page.
//
// PROTOTYPE-DEBT: [td-dashboard-080] Matrix is device-per-row (star topology);
// becomes NxN matrix in Beta 0.7 with passive capture when multiple sources exist.
type commsPageData struct {
	Title           string
	ActivePage      string
	Devices         []commsDeviceRow
	EventsAvailable bool
}

// buildCommsData queries the event store and assembles commsPageData.
// When d.events is nil, returns commsPageData with EventsAvailable=false.
func (d *Dashboard) buildCommsData() commsPageData {
	data := commsPageData{
		Title:           "Communications",
		ActivePage:      "comms",
		EventsAvailable: d.events != nil,
	}
	if d.events == nil {
		return data
	}
	stats, err := d.events.DeviceStats(context.Background())
	if err != nil {
		return data
	}
	data.Devices = make([]commsDeviceRow, 0, len(stats))
	for _, st := range stats {
		data.Devices = append(data.Devices, formatCommsRow(st))
	}
	return data
}

// formatCommsRow converts a DeviceStat into a display-ready commsDeviceRow.
func formatCommsRow(st eventstore.DeviceStat) commsDeviceRow {
	fcParts := make([]string, 0, len(st.DistinctFCs))
	for _, fc := range st.DistinctFCs {
		fcParts = append(fcParts, fmt.Sprintf("FC%d", fc))
	}
	return commsDeviceRow{
		DeviceID:    st.DeviceID,
		DeviceName:  st.DeviceName,
		TotalEvents: st.TotalEvents,
		WriteEvents: st.WriteEvents,
		LastSeen:    st.LastSeen.Format("2006-01-02 15:04:05"),
		FCCodeList:  joinStrings(fcParts),
	}
}
