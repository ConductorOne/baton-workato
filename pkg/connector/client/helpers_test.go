package client

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

// documentedDataCenters is a dated transcription of the vendor base-URL table:
// https://docs.workato.com/workato-api.html#base-url
//
// Transcribed from the vendor base-URL table as audited for CXH-2003
// (fetched 2026-07-06); verified live 2026-11-02, which additionally documents
// UK (https://app.uk.workato.com/api/) and an Enterprise US host change — both
// deliberately out of this PR's scope; UK is tracked as a follow-up.
//
// This table is intentionally hardcoded (not derived from WorkatoDataCenters)
// so the select options and the URL map stay in lockstep with the vendor doc.
// Adding a region to WorkatoDataCenters requires adding a row here,
// which forces a doc check at review time.
var documentedDataCenters = map[string]string{
	// us: kept at www.workato.com — the connector's live default, which still
	// serves the API. The vendor docs (live 2026-11-02) now list
	// https://app.workato.com/api/ under "Workato Enterprise customers";
	// migrating the default host is a deliberate behavior change, out of scope
	// for this PR and not part of this transcription.
	"us":      "https://www.workato.com",
	"eu":      "https://app.eu.workato.com",
	"jp":      "https://app.jp.workato.com",
	"sg":      "https://app.sg.workato.com",
	"au":      "https://app.au.workato.com",
	"il":      "https://app.il.workato.com",
	"sandbox": "https://app.trial.workato.com",
	"kr":      "https://app.kr.workato.com",
	"cn":      "https://app.workatoapp.cn",
}

func TestWorkatoDataCenters(t *testing.T) {
	require.Len(t, WorkatoDataCenters, len(documentedDataCenters),
		"WorkatoDataCenters must have exactly the regions documented in documentedDataCenters")

	for region, want := range documentedDataCenters {
		t.Run(region, func(t *testing.T) {
			got, ok := WorkatoDataCenters[region]
			require.True(t, ok, "WorkatoDataCenters is missing documented region %q", region)
			require.Equal(t, want, got)

			u, err := url.Parse(got)
			require.NoError(t, err)
			require.Equal(t, "https", u.Scheme, "region %q URL must use https", region)
		})
	}

	require.Equal(t, "https://app.kr.workato.com", WorkatoDataCenters["kr"])
	require.Equal(t, "https://app.workatoapp.cn", WorkatoDataCenters["cn"])
}
