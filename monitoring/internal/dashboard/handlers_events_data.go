package dashboard

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/eventstore"
)

// eventsPageData is the template data for the /events page.
// PROTOTYPE-DEBT: [td-dashboard-070] Device dropdown populated via APIClient.GetAssets()
// HTTP round-trip on every page load/refresh; could cache.
// TODO-FUTURE: Add device cache with TTL (Beta 0.7).
type eventsPageData struct {
	Title           string
	ActivePage      string
	Events          []eventRow
	EventsAvailable bool
	TotalCount      int64
	Page            int
	PageSize        int
	TotalPages      int
	PrevPage        int   // Page - 1; 0 means no previous page
	NextPage        int   // Page + 1; 0 means no next page
	ShowingFirst    int64 // first row number displayed (1-based), for "Showing X-Y of Z"
	ShowingLast     int64 // last row number displayed
	FilterDevice    string
	FilterFC        string
	FilterWrite     bool
	DeviceOptions   []string
	FCOptions       []fcOption
}

// eventRow is a display-ready event row for the template.
// UnitID is shown as a tooltip on the Device link per OT-REVIEW: in serial OT
// networks, multiple physical devices share a single IP:port endpoint and are
// distinguished only by Modbus Unit ID (slave address).
type eventRow struct {
	ID            string
	TimeShort     string // "14:23:05.123"
	TimeFull      string // RFC3339 for tooltip
	DeviceID      string
	DeviceName    string
	UnitID        uint8  // Modbus slave address; shown as tooltip on device link
	FC            uint8
	FCName        string // "Read Holding Registers" (full name for tooltip)
	FCShort       string // "FC3 Holding" (abbreviated for column display)
	AddrRange     string // "40001-40125 (125)" (4x/1x/0x/3x notation)
	IsWrite       bool
	WriteValues   string // "1234, 5678" or "ON, OFF" or "--"
	Success       bool
	ExceptionName string // "" or "Illegal Data Address"
	RTTms         string // "1.2ms"
}

// fcOption represents a function code dropdown option.
// Each option displays "FC{N} - {FullName}" to teach trainees the FC number,
// the data type, and the full canonical name together.
type fcOption struct {
	Code uint8
	Name string // full name from funcCodeTable
}

// defaultPageSize is the number of events displayed per page.
// PROTOTYPE-DEBT: [td-dashboard-071] No time range filter; would require date picker JS.
// TODO-FUTURE: Add Flatpickr or native input[type=datetime-local] (Beta 0.7).
const defaultPageSize = 100

// buildEventsData parses query parameters and queries the event store to produce
// the template data for the /events page and the events-table partial.
// When d.events is nil, returns eventsPageData with EventsAvailable=false.
// PROTOTYPE-DEBT: [td-dashboard-073] FC dropdown is static (all known FCs);
// intentionally does not filter to only observed FCs -- showing all FCs teaches
// what is possible on the wire, not just what has been seen. Do NOT change this
// to only show observed FCs.
func (d *Dashboard) buildEventsData(r *http.Request) eventsPageData {
	data := eventsPageData{
		Title:           "Transaction Events",
		ActivePage:      "events",
		PageSize:        defaultPageSize,
		FCOptions:       buildFCOptions(),
		EventsAvailable: d.events != nil,
	}
	if d.events == nil {
		return data
	}
	q := r.URL.Query()
	data.FilterDevice = q.Get("device")
	data.FilterFC = q.Get("fc")
	data.FilterWrite = q.Get("write_only") == "1"
	page := parsePageParam(q.Get("page"))
	data.Page = page
	opts := buildEventFilterOptions(data)
	ctx := r.Context()
	if total, err := d.events.Count(ctx, opts); err == nil {
		data.TotalCount = total
	}
	data.TotalPages = computeTotalPages(data.TotalCount, defaultPageSize)
	if page > 1 {
		data.PrevPage = page - 1
	}
	if page < data.TotalPages {
		data.NextPage = page + 1
	}
	opts.Limit = defaultPageSize
	opts.Offset = (page - 1) * defaultPageSize
	if events, err := d.events.Query(ctx, opts); err == nil {
		data.Events = make([]eventRow, 0, len(events))
		for _, e := range events {
			data.Events = append(data.Events, formatEventRow(e))
		}
	}
	if data.TotalCount > 0 {
		data.ShowingFirst = int64(opts.Offset) + 1
		data.ShowingLast = int64(opts.Offset) + int64(len(data.Events))
	}
	data.DeviceOptions = d.buildDeviceOptions(ctx)
	return data
}

// buildEventFilterOptions constructs FilterOptions from parsed eventsPageData fields.
func buildEventFilterOptions(data eventsPageData) eventstore.FilterOptions {
	opts := eventstore.FilterOptions{}
	if data.FilterDevice != "" {
		opts.DeviceID = &data.FilterDevice
	}
	if data.FilterFC != "" {
		if fc, err := strconv.ParseUint(data.FilterFC, 10, 8); err == nil {
			fc8 := uint8(fc)
			opts.FuncCode = &fc8
		}
	}
	if data.FilterWrite {
		t := true
		opts.IsWrite = &t
	}
	return opts
}

// buildDeviceOptions returns sorted device IDs from the live asset inventory.
func (d *Dashboard) buildDeviceOptions(ctx context.Context) []string {
	assets, err := d.api.GetAssets()
	if err != nil {
		return nil
	}
	deviceSet := make(map[string]bool, len(assets))
	for _, a := range assets {
		if a.ID != "" {
			deviceSet[a.ID] = true
		}
	}
	return sortedKeys(deviceSet)
}

// parsePageParam parses the page query parameter.
// Returns 1 for missing, non-numeric, or out-of-range values.
func parsePageParam(s string) int {
	if s == "" {
		return 1
	}
	p, err := strconv.Atoi(s)
	if err != nil || p < 1 {
		return 1
	}
	return p
}

// computeTotalPages returns the number of pages required to display count items
// at pageSize items per page. Returns 1 when count is 0.
func computeTotalPages(count int64, pageSize int) int {
	if count == 0 || pageSize <= 0 {
		return 1
	}
	pages := int(count) / pageSize
	if int(count)%pageSize != 0 {
		pages++
	}
	return pages
}

// formatEventRow converts a TransactionEvent into a display-ready eventRow.
// All formatting is done here so templates remain logic-free.
func formatEventRow(e *eventstore.TransactionEvent) eventRow {
	info := eventstore.LookupFuncCode(e.FunctionCode)
	excName := ""
	if !e.Success && e.ExceptionCode != 0 {
		excName = eventstore.LookupExceptionCode(e.ExceptionCode).Name
	}
	return eventRow{
		ID:            e.ID,
		TimeShort:     e.Timestamp.Format("15:04:05.000"),
		TimeFull:      e.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		DeviceID:      e.DeviceID,
		DeviceName:    e.DeviceName,
		UnitID:        e.UnitID,
		FC:            e.FunctionCode,
		FCName:        info.Name,
		FCShort:       fcShortName(e.FunctionCode),
		AddrRange:     formatAddrRange(e.FunctionCode, e.AddressStart, e.AddressCount),
		IsWrite:       e.IsWrite,
		WriteValues:   formatWriteValues(e.WriteDetail),
		Success:       e.Success,
		ExceptionName: excName,
		RTTms:         fmt.Sprintf("%.1fms", float64(e.ResponseTimeUs)/1000.0),
	}
}

// fcShortName returns the abbreviated FC label used in the FC column.
// Format is "FC{N} {DataType}" per OT-REVIEW: the FC number prefix disambiguates
// visually; the data type word identifies the Modbus address space.
// FC short names match how SCADA HMI diagnostics abbreviate function codes.
func fcShortName(fc uint8) string {
	switch fc {
	case 1:
		return "FC1 Coils"
	case 2:
		return "FC2 Disc.In"
	case 3:
		return "FC3 Holding"
	case 4:
		return "FC4 Input"
	case 5:
		return "FC5 WrCoil"
	case 6:
		return "FC6 WrReg"
	case 15:
		return "FC15 WrCoils"
	case 16:
		return "FC16 WrRegs"
	case 43:
		return "FC43 MEI"
	default:
		return fmt.Sprintf("FC%d", fc)
	}
}

// addrPrefix maps Modbus function codes to the register address space prefix used
// in one-based notation. Per OT-REVIEW: every OT device manual uses this notation.
// FC1/FC5/FC15 -> coils (0x), FC2 -> discrete inputs (1x),
// FC3/FC6/FC16 -> holding registers (4x), FC4 -> input registers (3x).
func addrPrefix(fc uint8) string {
	switch fc {
	case 1, 5, 15:
		return "0"
	case 2:
		return "1"
	case 3, 6, 16:
		return "4"
	case 4:
		return "3"
	default:
		return "4"
	}
}

// formatAddrRange formats an address range in Modbus one-based prefix notation.
// Example: FC3, addrStart=0, addrCount=125 -> "40001-40125 (125)".
// [OT-REVIEW] PDU zero-based offsets are not displayed; one-based prefix notation
// matches every real device data sheet a trainee will encounter in the field.
func formatAddrRange(fc uint8, addrStart, addrCount uint16) string {
	prefix := addrPrefix(fc)
	first := uint32(addrStart) + 1
	last := uint32(addrStart) + uint32(addrCount)
	return fmt.Sprintf("%s%d-%s%d (%d)", prefix, first, prefix, last, addrCount)
}

// formatWriteValues formats the write detail for display in the Value column.
// Returns "--" for read operations (nil detail), "ON"/"OFF" for coil writes,
// or comma-separated uint16 values for register writes.
func formatWriteValues(detail *eventstore.WriteDetail) string {
	if detail == nil {
		return "--"
	}
	if len(detail.CoilValues) > 0 {
		parts := make([]string, len(detail.CoilValues))
		for i, v := range detail.CoilValues {
			if v {
				parts[i] = "ON"
			} else {
				parts[i] = "OFF"
			}
		}
		return strings.Join(parts, ", ")
	}
	if len(detail.Values) > 0 {
		parts := make([]string, len(detail.Values))
		for i, v := range detail.Values {
			parts[i] = strconv.FormatUint(uint64(v), 10)
		}
		return strings.Join(parts, ", ")
	}
	return "--"
}

// buildFCOptions returns all known function codes as dropdown options.
// Static list intentionally (not filtered to observed FCs) -- see td-dashboard-073.
func buildFCOptions() []fcOption {
	infos := eventstore.AllFuncCodes()
	opts := make([]fcOption, 0, len(infos))
	for _, info := range infos {
		opts = append(opts, fcOption{Code: info.Code, Name: info.Name})
	}
	return opts
}
