package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"
)

type strictJSONOptions struct {
	description      string
	maximumBytes     int64
	rejectNullFields bool
}

func decodeStrictJSONBody[T any](
	ctx *gin.Context,
	options strictJSONOptions,
) (T, error) {
	var zero T
	contentType, _, err := mime.ParseMediaType(ctx.GetHeader("Content-Type"))
	if err != nil || contentType != "application/json" {
		return zero, fmt.Errorf("Content-Type must be application/json")
	}

	body := http.MaxBytesReader(
		ctx.Writer,
		ctx.Request.Body,
		options.maximumBytes,
	)
	defer body.Close()
	raw, err := io.ReadAll(body)
	if err != nil {
		return zero, fmt.Errorf("read %s: %w", options.description, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var result *T
	if err := decoder.Decode(&result); err != nil {
		return zero, fmt.Errorf("decode %s: %w", options.description, err)
	}
	if result == nil {
		return zero, fmt.Errorf("%s must be a JSON object", options.description)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return zero, fmt.Errorf(
			"%s must contain exactly one JSON object",
			options.description,
		)
	}
	if options.rejectNullFields {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return zero, fmt.Errorf("inspect %s: %w", options.description, err)
		}
		for field, value := range fields {
			if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return zero, fmt.Errorf(
					"%s field %q cannot be null",
					options.description,
					field,
				)
			}
		}
	}
	return *result, nil
}

func parsePaginationQuery(
	ctx *gin.Context,
	defaultPage int,
	defaultPerPage int,
) (int, int, error) {
	query := ctx.Request.URL.Query()
	for key := range query {
		if key != "page" && key != "perPage" {
			return 0, 0, fmt.Errorf("unsupported query parameter %q", key)
		}
	}
	page, err := parseSingleQueryInteger(query["page"], defaultPage)
	if err != nil {
		return 0, 0, fmt.Errorf("page: %w", err)
	}
	perPage, err := parseSingleQueryInteger(
		query["perPage"],
		defaultPerPage,
	)
	if err != nil {
		return 0, 0, fmt.Errorf("perPage: %w", err)
	}
	return page, perPage, nil
}
