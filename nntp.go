package nntp

import (
	"fmt"
	"io"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	connection *textproto.Conn

	headerFormat *HeaderFormat
}

func NewFromConn(conn io.ReadWriteCloser) (*Client, error) {
	c := &Client{
		connection: textproto.NewConn(conn),
	}

	code, msg, err := c.connection.ReadCodeLine(0)
	if err != nil {
		return nil, fmt.Errorf("failed to read 'Service Ready' message: %w", err)
	}

	if code != 200 && code != 201 {
		return nil, fmt.Errorf("server did not respond with valid greeting. Allowed codes: 200, 201. Got: %s", msg)
	}

	return c, nil
}

func (c *Client) Authenticate(username, password string) error {
	id := c.connection.Next()

	c.connection.StartRequest(id)
	defer c.connection.EndRequest(id)

	c.connection.StartResponse(id)
	defer c.connection.EndResponse(id)

	if err := c.connection.PrintfLine("AUTHINFO USER %s", username); err != nil {
		return err
	}

	if _, _, err := c.connection.ReadCodeLine(381); err != nil {
		return err
	}

	if err := c.connection.PrintfLine("AUTHINFO PASS %s", password); err != nil {
		return err
	}

	if _, _, err := c.connection.ReadCodeLine(281); err != nil {
		return err
	}

	return nil
}

func (c *Client) Quit() error {
	id, err := c.connection.Cmd("QUIT")
	if err != nil {
		return err
	}

	c.connection.StartResponse(id)
	defer c.connection.EndResponse(id)

	if _, _, err := c.connection.ReadCodeLine(205); err != nil {
		return err
	}

	return nil
}

func (c *Client) Help() (string, error) {
	id, err := c.connection.Cmd("HELP")
	if err != nil {
		return "", err
	}

	c.connection.StartResponse(id)
	defer c.connection.EndResponse(id)

	if _, _, err := c.connection.ReadCodeLine(100); err != nil {
		return "", err
	}

	lines, err := c.connection.ReadDotLines()
	if err != nil {
		return "", err
	}

	b := &strings.Builder{}

	for _, l := range lines {
		b.WriteString(l + "\n")
	}

	return b.String(), err
}

func (c *Client) Date() (time.Time, error) {
	const nntpDateLayout = "20060102150405"

	id, err := c.connection.Cmd("DATE")
	if err != nil {
		return time.Time{}, err
	}

	c.connection.StartResponse(id)
	defer c.connection.EndResponse(id)

	_, s, err := c.connection.ReadCodeLine(111)
	if err != nil {
		return time.Time{}, err
	}

	date, err := time.ParseInLocation(nntpDateLayout, s, time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse returned date: %w", err)
	}

	return date, nil
}

type NewsgroupStatus string

const (
	// Posting is permitted
	NewsgroupStatusPostingPermitted NewsgroupStatus = "y"
	// Posting is not permitted
	NewsgroupStatusPostingProhibited NewsgroupStatus = "n"
	// Postings will be forwarded to the newsgroup moderator
	NewsgroupStatusPostingModerated NewsgroupStatus = "m"
)

type NewsgroupOverview struct {
	Name   string
	Low    uint64
	High   uint64
	Status NewsgroupStatus
}

func (c *Client) Newsgroups(since time.Time) ([]NewsgroupOverview, error) {
	id, err := c.connection.Cmd("NEWGROUPS %s", since.UTC().Format("060102 150405 GMT"))
	if err != nil {
		return nil, err
	}

	c.connection.StartResponse(id)
	defer c.connection.EndResponse(id)

	if _, _, err := c.connection.ReadCodeLine(231); err != nil {
		return nil, err
	}

	lines, err := c.connection.ReadDotLines()
	if err != nil {
		return nil, err
	}

	groups := make([]NewsgroupOverview, len(lines))
	for i := range lines {
		groups[i], err = parseNewsgroupOverview(lines[i])
		if err != nil {
			return nil, fmt.Errorf("failed to parse newsgroup line '%s'. %w", lines[i], err)
		}
	}

	return groups, nil
}

func parseNewsgroupOverview(line string) (group NewsgroupOverview, err error) {
	parts := strings.Split(line, " ")
	if len(parts) != 4 {
		return group, fmt.Errorf(
			"invalid number of parts returned from newsgroup line. Expected 4 parts separated by space. Got %d",
			len(parts),
		)
	}

	group.Name = parts[0]

	switch strings.ToLower(parts[3]) {
	case "y", "n", "m":
		group.Status = NewsgroupStatus(strings.ToLower(parts[3]))
	default:
		return group, fmt.Errorf("invalid newsgroup status '%s'", parts[3])
	}

	if group.High, err = strconv.ParseUint(parts[1], 10, 64); err != nil {
		return group, fmt.Errorf("failed to parse high '%s': %w", parts[1], err)
	}

	if group.Low, err = strconv.ParseUint(parts[2], 10, 64); err != nil {
		return group, fmt.Errorf("failed to parse low '%s': %w", parts[2], err)
	}

	return group, err
}

type NewsgroupDetail struct {
	Name   string
	Low    uint64
	High   uint64
	Number uint64
}

func (c *Client) Group(g string) (group NewsgroupDetail, err error) {
	id, err := c.connection.Cmd("GROUP %s", g)
	if err != nil {
		return group, err
	}

	c.connection.StartResponse(id)
	defer c.connection.EndResponse(id)

	_, line, err := c.connection.ReadCodeLine(211)
	if err != nil {
		return group, err
	}

	parts := strings.Split(line, " ")
	if len(parts) != 4 {
		return group, fmt.Errorf(
			"invalid number of parts returned from newsgroup line. Expected 4 parts separated by space. Got %d",
			len(parts),
		)
	}

	group.Name = parts[3]

	if group.Number, err = strconv.ParseUint(parts[0], 10, 64); err != nil {
		return group, fmt.Errorf("failed to parse number '%s': %w", parts[0], err)
	}

	if group.Low, err = strconv.ParseUint(parts[1], 10, 64); err != nil {
		return group, fmt.Errorf("failed to parse low '%s': %w", parts[1], err)
	}

	if group.High, err = strconv.ParseUint(parts[2], 10, 64); err != nil {
		return group, fmt.Errorf("failed to parse high '%s': %w", parts[2], err)
	}

	return group, err
}

func (c *Client) SetOverviewFormat(format *HeaderFormat) {
	c.headerFormat = format
}

func (c *Client) InitializeOverviewFormat() error {
	id, err := c.connection.Cmd("LIST OVERVIEW.FMT")
	if err != nil {
		return err
	}

	c.connection.StartResponse(id)
	defer c.connection.EndResponse(id)

	if _, _, err = c.connection.ReadCodeLine(215); err != nil {
		return err
	}

	lines, err := c.connection.ReadDotLines()
	if err != nil {
		return err
	}

	c.headerFormat = NewHeaderFormat(lines)

	return nil
}

func (c *Client) Xover(r string) ([]Header, error) {
	if c.headerFormat == nil {
		if err := c.InitializeOverviewFormat(); err != nil {
			return nil, fmt.Errorf("failed to initialize overview format: %w", err)
		}
	}

	id, err := c.connection.Cmd("XOVER %s", r)
	if err != nil {
		return nil, err
	}

	c.connection.StartResponse(id)
	defer c.connection.EndResponse(id)

	if _, _, err = c.connection.ReadCodeLine(224); err != nil {
		return nil, err
	}

	lines, err := c.connection.ReadDotLines()
	if err != nil {
		return nil, err
	}

	headers := make([]Header, len(lines))
	for idx := range lines {
		headers[idx], err = c.headerFormat.ParseHeader(lines[idx])
		if err != nil {
			return nil, fmt.Errorf("failed to parse line '%s': %w", lines[idx], err)
		}
	}

	return headers, nil
}

type Header struct {
	MessageNumber uint64
	Subject       string
	Author        string
	Date          time.Time
	MessageID     string
	References    string
	Bytes         uint64
	Lines         uint64
	Additional    map[string]string
}
