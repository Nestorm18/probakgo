package domain

import (
	"strings"
	"testing"
)

func TestReportValidation(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"pve valid", (&PVEReportRequest{ReportID: "abc-123", Hostname: "pve-01"}).Validate()},
		{"pve negative", (&PVEReportRequest{Hostname: "pve-01", SwapUsed: -1}).Validate()},
		{"pbs excessive history", (&PBSReportRequest{Hostname: "pbs-01", PBSInformation: PBSInformation{Data: []PBSDatastorePayload{{History: make([]*float64, 10001)}}}}).Validate()},
		{"windows negative disk", (&WindowsReportRequest{Hostname: "win-01", Disks: []WindowsDiskPayload{{Name: "C:", Free: -1}}}).Validate()},
		{"invalid report id", (&PVEReportRequest{ReportID: "bad id", Hostname: "pve-01"}).Validate()},
		{"long hostname", (&PVEReportRequest{Hostname: strings.Repeat("x", 256)}).Validate()},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wantErr := tc.name != "pve valid"
			if (tc.err != nil) != wantErr {
				t.Fatalf("error = %v, wantErr %v", tc.err, wantErr)
			}
		})
	}
}
