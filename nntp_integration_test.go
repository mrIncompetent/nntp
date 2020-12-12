// +build integration

package nntp_test

import (
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrincompetent/nntp"
)

var testTime = time.Date(2020, 5, 1, 0, 0, 0, 0, time.Now().Location())

const (
	testGroup = "a.b.binaries.tvseries"
)

type LoggingConnection struct {
	t testing.TB
	c io.ReadWriteCloser
}

func (c *LoggingConnection) Read(p []byte) (n int, err error) {
	pp := make([]byte, len(p))

	n, err = c.c.Read(pp)
	copy(p, pp)

	c.t.Log(string(pp))

	return n, err
}

func (c *LoggingConnection) Write(p []byte) (n int, err error) {
	pp := make([]byte, len(p))
	copy(pp, p)

	c.t.Log(string(pp))

	return c.c.Write(p)
}

func (c *LoggingConnection) Close() error {
	return c.c.Close()
}

func GetIntegrationClient(t testing.TB) *nntp.Client {
	conn, err := net.Dial("tcp", os.Getenv("NNTP_TEST_ADDRESS"))
	require.NoError(t, err, "Failed to get integration test connection")

	client, err := nntp.NewFromConn(&LoggingConnection{c: conn, t: t})
	require.NoError(t, err, "Failed to create client from connection")

	t.Cleanup(func() {
		require.NoError(t, client.Quit(), "Failed to close client")

		require.NoError(t, conn.Close(), "Failed to close connection")
	})

	return client
}

func GetAuthenticatedIntegrationClient(t testing.TB) *nntp.Client {
	client := GetIntegrationClient(t)

	err := client.Authenticate(os.Getenv("NNTP_TEST_USERNAME"), os.Getenv("NNTP_TEST_PASSWORD"))
	require.NoError(t, err, "Failed to authenticate")

	return client
}

func TestClient_Integration_Help(t *testing.T) {
	client := GetAuthenticatedIntegrationClient(t)

	help, err := client.Help()
	require.NoError(t, err, "Failed to call help")

	t.Logf("Help: %s", help)
}

func TestClient_Integration_Date(t *testing.T) {
	client := GetAuthenticatedIntegrationClient(t)

	date, err := client.Date()
	require.NoError(t, err, "Failed to call date")

	t.Logf("Date: %s", date)
}

func TestClient_Integration_Newsgroups(t *testing.T) {
	client := GetAuthenticatedIntegrationClient(t)

	groups, err := client.Newsgroups(testTime)
	require.NoError(t, err, "Failed to list groups")

	t.Log(toJSON(t, groups[0]))
}

func TestClient_Integration_Group(t *testing.T) {
	client := GetAuthenticatedIntegrationClient(t)

	group, err := client.Group(testGroup)
	require.NoError(t, err, "Failed to change group")

	t.Log(toJSON(t, group))
}

func TestClient_Integration_Xover(t *testing.T) {
	client := GetAuthenticatedIntegrationClient(t)

	group, err := client.Group(testGroup)
	require.NoError(t, err, "Failed to change group")

	headers, err := client.Xover(fmt.Sprintf("%d-%d", group.High-100, group.High))
	require.NoError(t, err, "Failed to list headers")

	for _, header := range headers {
		t.Logf("%s: %s", header.MessageID, header.Subject)
	}
}

func TestClient_Integration_XoverChan(t *testing.T) {
	client := GetAuthenticatedIntegrationClient(t)

	group, err := client.Group(testGroup)
	require.NoError(t, err, "Failed to change group")

	headerChan, errChan, err := client.XoverChan(fmt.Sprintf("%d-%d", group.High-100, group.High))
	require.NoError(t, err, "Failed to list headers")

	for header := range headerChan {
		t.Logf("%s: %s", header.MessageID, header.Subject)
	}

	assert.Len(t, errChan, 0)
}
