package client

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

// documentedDataCenters is transcribed from the vendor documentation:
// https://docs.workato.com/workato-api.html#base-url
//
// This table is intentionally hardcoded (not derived from WorkatoDataCenters)
// so the select options and the URL map stay in lockstep with the vendor doc.
// Adding a tenth region to WorkatoDataCenters requires adding a row here,
// which forces a doc check at review time.
var documentedDataCenters = map[string]string{
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
	require.Len(t, WorkatoDataCenters, 9, "WorkatoDataCenters must have exactly the nine documented regions")

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
