package nntp_test

import (
	"testing"
	"time"

	"github.com/go-test/deep"

	"github.com/mrincompetent/nntp"
)

func TestHeaderFormat_ParseHeader(t *testing.T) {
	locationCST, err := time.LoadLocation("America/Panama")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name           string
		line           string
		format         *nntp.HeaderFormat
		expectedHeader nntp.Header
	}{
		{
			name: "successful",
			line: `1	some subject	some author	Sun, 10 May 2020 00:32:22 +0000	<some-msg-id>		67755	519	Xref: news.some-newsserver.com some-alternative-group-1:123 some-alternative-group-2:456 some-alternative-group-3:789`,
			format: nntp.NewHeaderFormat([]string{
				"Subject:",
				"From:",
				"Date:",
				"Message-ID:",
				"References:",
				":bytes",
				":lines",
				"Xref:full",
			}),
			expectedHeader: nntp.Header{
				MessageNumber: 1,
				Subject:       "some subject",
				Author:        "some author",
				Date:          time.Date(2020, 5, 10, 0, 32, 22, 0, time.UTC),
				MessageID:     "<some-msg-id>",
				References:    "",
				Bytes:         67755,
				Lines:         519,
				Additional: map[string]string{
					"Xref": "news.some-newsserver.com some-alternative-group-1:123 some-alternative-group-2:456 some-alternative-group-3:789",
				},
			},
		},
		{
			name: "successful - rfc3977 - 1",
			line: `3000234	some other subject	"Test Author" <test@example.com>	6 Oct 1998 04:38:40 -0500	<some-other-msg-id>	<some-other-ref@example.net>	1234	17	Xref: news.some-newsserver.com some-alternative-group-1:123`,
			format: nntp.NewHeaderFormat([]string{
				"Subject:",
				"From:",
				"Date:",
				"Message-ID:",
				"References:",
				":bytes",
				":lines",
				"Xref:full",
			}),
			expectedHeader: nntp.Header{
				MessageNumber: 3000234,
				Subject:       "some other subject",
				Author:        "\"Test Author\" <test@example.com>",
				Date:          time.Date(1998, 10, 6, 4, 38, 40, 0, locationCST),
				MessageID:     "<some-other-msg-id>",
				References:    "<some-other-ref@example.net>",
				Bytes:         1234,
				Lines:         17,
				Additional: map[string]string{
					"Xref": "news.some-newsserver.com some-alternative-group-1:123",
				},
			},
		},
		{
			name: "successful - rfc3977 - 2",
			line: `0	some other subject	"Test Author" <test@example.com>	6 Oct 1998 04:38:40 -0500	<some-other-msg-id>	<some-other-ref@example.net>	1234	17	Xref: news.some-newsserver.com some-alternative-group-1:123`,
			format: nntp.NewHeaderFormat([]string{
				"Subject:",
				"From:",
				"Date:",
				"Message-ID:",
				"References:",
				":bytes",
				":lines",
				"Xref:full",
			}),
			expectedHeader: nntp.Header{
				MessageNumber: 0,
				Subject:       "some other subject",
				Author:        "\"Test Author\" <test@example.com>",
				Date:          time.Date(1998, 10, 6, 4, 38, 40, 0, locationCST),
				MessageID:     "<some-other-msg-id>",
				References:    "<some-other-ref@example.net>",
				Bytes:         1234,
				Lines:         17,
				Additional: map[string]string{
					"Xref": "news.some-newsserver.com some-alternative-group-1:123",
				},
			},
		},
		{
			name: "successful - rfc3977 - 3",
			line: `3000235	Another test article	<test@example.com> (Test Author)	6 Oct 1998 04:38:45 -0500	<some-other-msg-id>		4818	37		Distribution: fi`,
			format: nntp.NewHeaderFormat([]string{
				"Subject:",
				"From:",
				"Date:",
				"Message-ID:",
				"References:",
				":bytes",
				":lines",
				"Xref:full",
				"Distribution:full",
			}),
			expectedHeader: nntp.Header{
				MessageNumber: 3000235,
				Subject:       "Another test article",
				Author:        "<test@example.com> (Test Author)",
				Date:          time.Date(1998, 10, 6, 4, 38, 45, 0, locationCST),
				MessageID:     "<some-other-msg-id>",
				References:    "",
				Bytes:         4818,
				Lines:         37,
				Additional: map[string]string{
					"Distribution": "fi",
					"Xref":         "",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			header, err := test.format.ParseHeader(test.line)
			if err != nil {
				t.Fatal(err)
			}

			if diff := deep.Equal(header, test.expectedHeader); diff != nil {
				t.Errorf("Diff: %v", diff)
			}
		})
	}
}

func TestParseDate(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Location: %s", loc.String())

	expectedDate := time.Date(2020, 1, 1, 12, 34, 56, 0, loc)

	dates := []string{
		"1 Jan 2020 12:34:56 +0100",
		"Wed, 01 Jan 2020 12:34:56 +0100",
		"Wed, 01 Jan 2020 13:34:56 CEST",
	}

	for _, s := range dates {
		t.Run(s, func(t *testing.T) {
			gotDate, err := nntp.ParseDate(s)
			if err != nil {
				t.Fatalf("Failed to parse date: %s: %v", s, err)
			}

			t.Logf("Got date:      %s", gotDate.Format(time.RFC3339))
			t.Logf("Expected date: %s", expectedDate.Format(time.RFC3339))

			if !gotDate.Equal(expectedDate) {
				t.Error("Returned date does not match expected date")
			}
		})
	}
}
