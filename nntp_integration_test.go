// +build integration

package nntp_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/mrincompetent/nntp"
)

var (
	testTime = time.Date(2020, 5, 1, 0, 0, 0, 0, time.Now().Location())
)

const (
	testGroup = "alt.binaries.es"
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
	if err != nil {
		t.Fatalf("failed to create integration test connection: %v", err)
	}

	client, err := nntp.NewFromConn(&LoggingConnection{c: conn, t: t})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := client.Quit(); err != nil {
			t.Fatalf("failed to close client: %v", err)
		}

		if err := conn.Close(); err != nil {
			t.Fatalf("failed to close connection: %v", err)
		}
	})

	return client
}

func GetAuthenticatedIntegrationClient(t testing.TB) *nntp.Client {
	client := GetIntegrationClient(t)

	if err := client.Authenticate(os.Getenv("NNTP_TEST_USERNAME"), os.Getenv("NNTP_TEST_PASSWORD")); err != nil {
		t.Errorf("failed to authenticate: %v", err)
	}

	return client
}

func TestClient_Integration_Help(t *testing.T) {
	client := GetAuthenticatedIntegrationClient(t)

	help, err := client.Help()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Help: %s", help)
}

func TestClient_Integration_Newsgroups(t *testing.T) {
	client := GetAuthenticatedIntegrationClient(t)

	groups, err := client.Newsgroups(testTime)
	if err != nil {
		t.Fatal(err)
	}

	b, err := json.Marshal(groups[0])
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(b))
}

func TestClient_Integration_Group(t *testing.T) {
	client := GetAuthenticatedIntegrationClient(t)

	group, err := client.Group(testGroup)
	if err != nil {
		t.Fatal(err)
	}

	b, err := json.Marshal(group)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(b))
}

func TestClient_Integration_Xover(t *testing.T) {
	client := GetAuthenticatedIntegrationClient(t)

	group, err := client.Group(testGroup)
	if err != nil {
		t.Fatal(err)
	}

	headers, err := client.Xzver(fmt.Sprintf("%d-%d", group.High-100, group.High))
	if err != nil {
		t.Fatal(err)
	}

	for _, header := range headers {
		t.Log(header.Subject)
	}
}
