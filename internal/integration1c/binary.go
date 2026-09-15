package integration1c

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

type BinaryResponse struct {
	Body        []byte
	ContentType string
	FileName    string
}

func executeBinary(ctx context.Context, client *Client, command string, params any) (BinaryResponse, error) {
	requestBody, err := marshalCommandRequest(command, params)
	if err != nil {
		return BinaryResponse{}, err
	}

	for attempt := 0; attempt <= client.maxRetries; attempt++ {
		result, retry, err := executeBinaryAttempt(ctx, client, command, requestBody)
		if err == nil {
			return result, nil
		}
		if !retry || attempt == client.maxRetries {
			return BinaryResponse{}, err
		}
		if err := waitRetry(ctx, client.retryDelay, attempt); err != nil {
			return BinaryResponse{}, err
		}
	}

	return BinaryResponse{}, fmt.Errorf("1c binary command %q failed", command)
}

func executeBinaryAttempt(
	ctx context.Context,
	client *Client,
	command string,
	requestBody []byte,
) (BinaryResponse, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.binDataURL, bytes.NewReader(requestBody))
	if err != nil {
		return BinaryResponse{}, false, fmt.Errorf("create 1c binary request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/pdf, application/octet-stream, */*")

	resp, err := client.httpClient.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return BinaryResponse{}, false, ctxErr
		}

		return BinaryResponse{}, isRetriableTransportError(err), fmt.Errorf(
			"execute 1c binary command %q: %w",
			command,
			err,
		)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return BinaryResponse{}, true, fmt.Errorf("read 1c binary command %q response: %w", command, err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		httpErr := &HTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       strings.TrimSpace(string(body)),
		}
		return BinaryResponse{}, isRetriableStatus(resp.StatusCode), httpErr
	}

	if len(body) == 0 {
		return BinaryResponse{}, false, fmt.Errorf("1c binary command %q returned an empty body", command)
	}

	return BinaryResponse{
		Body:        body,
		ContentType: responseContentType(resp.Header.Get("Content-Type")),
		FileName:    responseFileName(resp.Header.Get("Content-Disposition")),
	}, false, nil
}

func responseContentType(value string) string {
	mediaType, _, err := mime.ParseMediaType(value)
	if err == nil && mediaType != "" {
		return mediaType
	}
	return strings.TrimSpace(strings.Split(value, ";")[0])
}

func responseFileName(contentDisposition string) string {
	_, params, err := mime.ParseMediaType(contentDisposition)
	if err != nil {
		return ""
	}
	name := strings.TrimSpace(params["filename"])
	if name == "" {
		return ""
	}
	return filepath.Base(name)
}
