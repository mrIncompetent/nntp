package nntp_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/textproto"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrincompetent/nntp"
)

type bufferConnection struct {
	read  *bytes.Buffer
	write *bytes.Buffer
}

func (r *bufferConnection) Read(p []byte) (n int, err error) {
	return r.read.Read(p)
}

func (r *bufferConnection) Write(p []byte) (n int, err error) {
	return r.write.Write(p)
}

func (r *bufferConnection) Close() error {
	return nil
}

func (r *bufferConnection) RecordPrintfLine(t testing.TB, line string, args ...interface{}) {
	bufWriter := bufio.NewWriter(r.read)
	defer func() {
		require.NoError(t, bufWriter.Flush(), "Failed to flush")
	}()

	require.NoError(t, textproto.NewWriter(bufWriter).PrintfLine(line, args...), "Failed to write")
}

func (r *bufferConnection) RecordDotMessage(t testing.TB, s string) {
	bufWriter := bufio.NewWriter(r.read)
	defer func() {
		require.NoError(t, bufWriter.Flush(), "Failed to flush")
	}()

	txtWriter := textproto.NewWriter(bufWriter).DotWriter()
	defer func() {
		require.NoError(t, txtWriter.Close(), "Failed to close")
	}()

	_, err := txtWriter.Write([]byte(s))
	require.NoError(t, err, "Failed to write")
}

func newBufferConnection() *bufferConnection {
	return &bufferConnection{
		read:  &bytes.Buffer{},
		write: &bytes.Buffer{},
	}
}

func GetClient(t testing.TB) (*nntp.Client, *bufferConnection) {
	conn := newBufferConnection()
	conn.RecordPrintfLine(t, "200 some-newsserver")

	client, err := nntp.NewFromConn(conn)
	require.NoError(t, err, "Failed to create new client from connection")

	return client, conn
}

func GetAuthenticatedClient(t testing.TB) (*nntp.Client, *bufferConnection) {
	client, conn := GetClient(t)
	conn.RecordPrintfLine(t, "381 PASS required")
	conn.RecordPrintfLine(t, "281 Ok")

	err := client.Authenticate("foo", "bar")
	require.NoError(t, err, "Failed to authenticate")

	return client, conn
}

func TestClient_Authenticate(t *testing.T) {
	GetAuthenticatedClient(t)
}

func TestClient_Help(t *testing.T) {
	const expectedHelp = `article [MessageID|Number]
body [MessageID|Number]
date
head [MessageID|Number]
help
ihave
mode reader|stream
slave
quit
group newsgroup
last
next
list [active|active.times|newsgroups|extensions|distributions|distrib.pats|moderators|overview.fmt|subscriptions]
listgroup newsgroup
newgroups yymmdd hhmmss ["GMT"] [<distributions>]
post
stat [MessageID|Number]
xgtitle [group_pattern]
xhdr header [range|MessageID]
xzhdr header [range|MessageID]
hdr header [range|MessageID]
over [range]
xover [range]
xzver [range]
xpat header range|MessageID pat
xpath MessageID
xnumbering rfc3977|rfc977|window
xfeature compress gzip [terminator]
authinfo user Name|pass Password
`

	client, conn := GetAuthenticatedClient(t)
	conn.RecordPrintfLine(t, "100 Legal commands")
	conn.RecordDotMessage(t, expectedHelp)

	gotHelp, err := client.Help()
	require.NoError(t, err, "Failed to call help")

	if gotHelp != expectedHelp {
		t.Logf("Expected help: \n%s", expectedHelp)
		t.Logf("Got help: \n%s", gotHelp)
		t.Error("Got help text does not match expected help")
	}
}

func TestClient_Date(t *testing.T) {
	client, conn := GetAuthenticatedClient(t)
	conn.RecordPrintfLine(t, "111 19990623135624")

	date, err := client.Date()
	require.NoError(t, err, "Failed to call date")

	expectedDate := time.Date(1999, 6, 23, 13, 56, 24, 0, time.UTC)

	assert.Equal(t, expectedDate, date)
}

func TestClient_Newsgroups(t *testing.T) {
	t.Run("successful", func(t *testing.T) {
		client, conn := GetAuthenticatedClient(t)
		conn.RecordPrintfLine(t, "231 list of new newsgroups follows")
		conn.RecordDotMessage(t, `group1 4 1 y
group2 89 56 n
group3 99 80 m
`)
		expectedGroups := []nntp.NewsgroupOverview{
			{
				Name:   "group1",
				Low:    1,
				High:   4,
				Status: nntp.NewsgroupStatusPostingPermitted,
			},
			{
				Name:   "group2",
				Low:    56,
				High:   89,
				Status: nntp.NewsgroupStatusPostingProhibited,
			},
			{
				Name:   "group3",
				Low:    80,
				High:   99,
				Status: nntp.NewsgroupStatusPostingModerated,
			},
		}

		gotGroups, err := client.Newsgroups(time.Now())
		require.NoError(t, err, "Failed to list newsgroups")

		if !reflect.DeepEqual(gotGroups, expectedGroups) {
			t.Logf("Expected: %s", toJSON(t, expectedGroups))
			t.Logf("Got: %s", toJSON(t, gotGroups))
			t.Fatal("Returned groups do not match expected groups")
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		client, conn := GetAuthenticatedClient(t)
		conn.RecordPrintfLine(t, "231 list of new newsgroups follows")
		conn.RecordDotMessage(t, "group3 99 80 zzz")

		_, gotErr := client.Newsgroups(time.Now())
		expectedErr := fmt.Errorf("invalid newsgroup status '%s'", "zzz")
		if !errors.Is(gotErr, gotErr) {
			t.Logf("Expected: %v", expectedErr)
			t.Logf("Got: %v", gotErr)
			t.Error("Invalid error returned")
		}
	})

	t.Run("invalid high", func(t *testing.T) {
		client, conn := GetAuthenticatedClient(t)
		conn.RecordPrintfLine(t, "231 list of new newsgroups follows")
		conn.RecordDotMessage(t, "group3 a 80 y")

		_, gotErr := client.Newsgroups(time.Now())
		var expectedErr *strconv.NumError
		if !errors.As(gotErr, &expectedErr) {
			t.Logf("Expected: %T", expectedErr)
			t.Logf("Got: %T", gotErr)
			t.Error("Invalid error returned")
		}
	})

	t.Run("invalid low", func(t *testing.T) {
		client, conn := GetAuthenticatedClient(t)
		conn.RecordPrintfLine(t, "231 list of new newsgroups follows")
		conn.RecordDotMessage(t, "group3 99 a y")

		_, gotErr := client.Newsgroups(time.Now())
		var expectedErr *strconv.NumError
		if !errors.As(gotErr, &expectedErr) {
			t.Logf("Expected: %T: %v", expectedErr, expectedErr)
			t.Logf("Got: %T: %v", gotErr, gotErr)
			t.Error("Invalid error returned")
		}
	})
}

func TestClient_Group(t *testing.T) {
	t.Run("successful", func(t *testing.T) {
		client, conn := GetAuthenticatedClient(t)
		conn.RecordPrintfLine(t, "211 491902 1 491902 group1")

		expectedGroup := nntp.NewsgroupDetail{
			Name:   "group1",
			Low:    1,
			High:   491902,
			Number: 491902,
		}

		gotGroup, err := client.Group("group1")
		require.NoError(t, err, "Failed to change to group")

		if !reflect.DeepEqual(gotGroup, expectedGroup) {
			t.Logf("Expected: %s", toJSON(t, expectedGroup))
			t.Logf("Got: %s", toJSON(t, gotGroup))
			t.Fatal("Returned group do not match expected group")
		}
	})

	t.Run("invalid num", func(t *testing.T) {
		client, conn := GetAuthenticatedClient(t)
		conn.RecordPrintfLine(t, "211 y 1 491902 group1")

		_, gotErr := client.Group("group1")
		var expectedErr *strconv.NumError
		if !errors.As(gotErr, &expectedErr) {
			t.Logf("Expected: %T: %v", expectedErr, expectedErr)
			t.Logf("Got: %T: %v", gotErr, gotErr)
			t.Error("Invalid error returned")
		}
	})

	t.Run("invalid low", func(t *testing.T) {
		client, conn := GetAuthenticatedClient(t)
		conn.RecordPrintfLine(t, "211 491902 y 491902 group1")

		_, gotErr := client.Group("group1")
		var expectedErr *strconv.NumError
		if !errors.As(gotErr, &expectedErr) {
			t.Logf("Expected: %T: %v", expectedErr, expectedErr)
			t.Logf("Got: %T: %v", gotErr, gotErr)
			t.Error("Invalid error returned")
		}
	})

	t.Run("invalid high", func(t *testing.T) {
		client, conn := GetAuthenticatedClient(t)
		conn.RecordPrintfLine(t, "211 491902 1 y group1")

		_, gotErr := client.Group("group1")
		var expectedErr *strconv.NumError
		if !errors.As(gotErr, &expectedErr) {
			t.Logf("Expected: %T: %v", expectedErr, expectedErr)
			t.Logf("Got: %T: %v", gotErr, gotErr)
			t.Error("Invalid error returned")
		}
	})
}

func toJSON(t testing.TB, i interface{}) string {
	b, err := json.MarshalIndent(i, "", "  ")
	require.NoError(t, err, "Failed to marshal %T to JSON", i)

	return string(b)
}

func TestClient_Xzver(t *testing.T) {
	client, conn := GetAuthenticatedClient(t)
	client.SetOverviewFormat(nntp.DefaultOverviewFormat())
	conn.RecordPrintfLine(t, "224 Overview information follows")
	conn.RecordDotMessage(t, `1	some subject	some author	Sun, 10 May 2020 00:32:22 +0000	<some-msg-id>		67755	519
2	some subject	some author	Sun, 10 May 2020 00:32:22 +0000	<some-msg-id>		67755	519
`)

	gotHeaders, err := client.Xover("1-1000")
	require.NoError(t, err, "Failed to list compressed headers")

	expectedHeaders := []nntp.Header{
		{
			MessageNumber: 1,
			Subject:       "some subject",
			Author:        "some author",
			Date:          time.Date(2020, 5, 10, 0, 32, 22, 0, time.FixedZone("", 0)),
			MessageID:     "<some-msg-id>",
			References:    "",
			Bytes:         67755,
			Lines:         519,
		},
		{
			MessageNumber: 2,
			Subject:       "some subject",
			Author:        "some author",
			Date:          time.Date(2020, 5, 10, 0, 32, 22, 0, time.FixedZone("", 0)),
			MessageID:     "<some-msg-id>",
			References:    "",
			Bytes:         67755,
			Lines:         519,
		},
	}

	t.Logf("Expected headers: \n%s", toJSON(t, expectedHeaders))
	t.Logf("Got headers: %v", toJSON(t, gotHeaders))

	assert.Equal(t, expectedHeaders, gotHeaders)
}
