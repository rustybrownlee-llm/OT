// Package syslog provides CEF-formatted syslog forwarding for Modbus
// transaction events. It implements plaintext UDP and TCP transport using only
// the standard library, in accordance with NFR-2 (no external syslog libraries).
//
// Transport security note: plaintext UDP/TCP is acceptable for lab environments.
// Production deployments should use TLS-encrypted syslog per RFC 5425.
//
// PROTOTYPE-DEBT: [td-events-061] Plaintext UDP/TCP only; no TLS transport.
// Production OT environments require RFC 5425 (TLS syslog) or a dedicated
// log shipper (Filebeat, Fluentd). Deferred to a future milestone.
package syslog

// facilityCode maps a syslog facility name to its numeric code as defined in
// RFC 5424. The local0-local7 range (codes 16-23) is recommended for OT
// monitoring events because it leaves standard facilities available for OS logs,
// enabling SIEM correlation rules to separate OT events from host events by
// facility code alone.
var facilityCode = map[string]int{
	"kern":   0,
	"user":   1,
	"mail":   2,
	"daemon": 3,
	"auth":   4,
	"syslog": 5,
	"lpr":    6,
	"news":   7,
	"uucp":   8,
	"cron":   9,
	// local0-local7 (codes 16-23) -- recommended for OT monitoring.
	"local0": 16,
	"local1": 17,
	"local2": 18,
	"local3": 19,
	"local4": 20,
	"local5": 21,
	"local6": 22,
	"local7": 23,
}

// FacilityCode returns the numeric syslog facility code for the given facility
// name. Returns 16 (local0) for unrecognized names. Config validation ensures
// only recognized names reach the emitter, so this fallback is a safety net.
func FacilityCode(name string) int {
	if code, ok := facilityCode[name]; ok {
		return code
	}
	return 16 // local0 default
}
