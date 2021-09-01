package nntp

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func NewOverviewFormat(fields []string) *OverviewFormat {
	format := &OverviewFormat{
		fieldNames:          make([]string, len(fields)),
		lowercaseFieldNames: make([]string, len(fields)),
	}

	for idx := range fields {
		format.fieldNames[idx] = fields[idx]
		format.lowercaseFieldNames[idx] = strings.ToLower(fields[idx])
	}

	return format
}

type OverviewFormat struct {
	fieldNames          []string
	lowercaseFieldNames []string
}

var ErrInvalidHeaderCount = errors.New("invalid number of headers given")

func (h *OverviewFormat) FieldToHeader(idx int, value string, header *Header) (err error) {
	if idx+1 > len(h.fieldNames) {
		return fmt.Errorf(
			"%w: header format only knows about %d field(s). %dth field given",
			ErrInvalidHeaderCount,
			len(h.fieldNames),
			idx+1,
		)
	}

	fieldName := h.fieldNames[idx]
	lowercaseFieldName := h.lowercaseFieldNames[idx]

	switch lowercaseFieldName {
	case "subject:":
		header.Subject = value
	case "from:":
		header.Author = value
	case "date:":
		if header.Date, err = ParseDate(value); err != nil {
			return fmt.Errorf("failed to parse date '%s': %w", value, err)
		}
	case "message-id:":
		header.MessageID = value
	case "references:":
		header.References = value
	case "bytes:", ":bytes":
		if header.Bytes, err = strconv.ParseUint(value, 10, 64); err != nil {
			return fmt.Errorf("failed to parse bytes '%s': %w", value, err)
		}
	case "lines:", ":lines":
		// For some reason it's not always set
		if strings.TrimSpace(value) == "" {
			header.Lines = 0
		} else {
			if header.Lines, err = strconv.ParseUint(value, 10, 64); err != nil {
				return fmt.Errorf("failed to parse 'lines' field '%s': %w", value, err)
			}
		}

	default:
		if header.Additional == nil {
			header.Additional = map[string]string{}
		}

		// Remove the 'full' prefix & suffix
		if strings.HasSuffix(lowercaseFieldName, ":full") {
			fieldName = fieldName[0 : len(fieldName)-4]

			value = strings.TrimPrefix(value, fieldName)
		}

		header.Additional[strings.TrimSuffix(fieldName, ":")] = strings.TrimSpace(value)
	}

	return nil
}

func (h *OverviewFormat) ParseXoverLine(line string) (header Header, err error) {
	fields := strings.Split(line, "\t")
	// MessageNumber doesn't get mentioned in the format, but it's always the first field.
	if header.MessageNumber, err = strconv.ParseUint(fields[0], 10, 64); err != nil {
		return header, fmt.Errorf("failed to parse message number '%s': %w", fields[0], err)
	}

	fields = fields[1:]
	for idx := range fields {
		if err := h.FieldToHeader(idx, fields[idx], &header); err != nil {
			return header, fmt.Errorf("failed to map field %d ('%s'): %w", idx, fields[idx], err)
		}
	}

	return header, err
}

var ErrInvalidDateFormat = errors.New("invalid date format")

func ParseDate(s string) (time.Time, error) {
	layouts := []string{
		time.RFC1123Z,
		time.RFC1123,
		"2 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 -0700 (MST)",
		"Mon, 02 Jan 06 15:04:05 MST",
		"02 Jan 06 15:04:05 MST",
		"02 Jan 2006 15:04:05 MST",
	}

	for _, layout := range layouts {
		t, err := time.Parse(layout, s)
		if err != nil {
			continue
		}

		return t, nil
	}

	return time.Time{}, fmt.Errorf("%w: does not match known format. Known formats: %v", ErrInvalidDateFormat, layouts)
}

func DefaultOverviewFormat() *OverviewFormat {
	return NewOverviewFormat([]string{
		"Subject:",
		"From:",
		"Date:",
		"Message-ID:",
		"References:",
		":bytes",
		":lines",
	})
}
