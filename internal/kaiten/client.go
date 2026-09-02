package kaiten

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Depik400/agent-kaiten-proxy/internal/apperr"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type User struct {
	ID       int    `json:"id"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

type CardFilters struct {
	SpaceID            int
	BoardID            int
	LaneID             int
	MemberIDs          string
	ResponsibleIDs     string
	States             string
	IncludeDescription bool
	Limit              int
	Offset             int
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func NewClientWithHTTP(baseURL, token string, httpClient *http.Client) *Client {
	c := NewClient(baseURL, token)
	c.httpClient = httpClient
	return c
}

func (c *Client) VerifyToken(ctx context.Context) error {
	_, err := c.CurrentUser(ctx)
	return err
}

func (c *Client) CurrentUser(ctx context.Context) (User, error) {
	var user User
	if err := c.get(ctx, "/users/current", nil, &user); err != nil {
		return User{}, err
	}
	return user, nil
}

func (c *Client) ListCards(ctx context.Context, filters CardFilters) ([]json.RawMessage, error) {
	values := url.Values{}
	if filters.SpaceID > 0 {
		values.Set("space_id", strconv.Itoa(filters.SpaceID))
	}
	if filters.BoardID > 0 {
		values.Set("board_id", strconv.Itoa(filters.BoardID))
	}
	if filters.LaneID > 0 {
		values.Set("lane_id", strconv.Itoa(filters.LaneID))
	}
	if filters.MemberIDs != "" {
		values.Set("member_ids", filters.MemberIDs)
	}
	if filters.ResponsibleIDs != "" {
		values.Set("responsible_ids", filters.ResponsibleIDs)
	}
	if filters.States != "" {
		values.Set("states", filters.States)
	}
	if filters.IncludeDescription {
		values.Set("additional_card_fields", "description")
	}
	values.Set("broken_api", "false")
	limit := filters.Limit
	if limit <= 0 {
		limit = 100
	}
	values.Set("limit", strconv.Itoa(limit))
	if filters.Offset > 0 {
		values.Set("offset", strconv.Itoa(filters.Offset))
	}
	var out []json.RawMessage
	if err := c.get(ctx, "/cards", values, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetCard(ctx context.Context, id int) (json.RawMessage, error) {
	values := url.Values{}
	values.Set("broken_api", "false")
	var card json.RawMessage
	if err := c.get(ctx, fmt.Sprintf("/cards/%d", id), values, &card); err != nil {
		return nil, err
	}
	return card, nil
}

func (c *Client) ListSpaces(ctx context.Context) ([]json.RawMessage, error) {
	var spaces []json.RawMessage
	if err := c.get(ctx, "/spaces", nil, &spaces); err != nil {
		return nil, err
	}
	return spaces, nil
}

func (c *Client) ListBoards(ctx context.Context, spaceID int) ([]json.RawMessage, error) {
	var boards []json.RawMessage
	if err := c.get(ctx, fmt.Sprintf("/spaces/%d/boards", spaceID), nil, &boards); err != nil {
		return nil, err
	}
	return boards, nil
}

func (c *Client) ListLanes(ctx context.Context, boardID int) ([]json.RawMessage, error) {
	values := url.Values{}
	values.Set("condition", "1")
	var lanes []json.RawMessage
	if err := c.get(ctx, fmt.Sprintf("/boards/%d/lanes", boardID), values, &lanes); err != nil {
		return nil, err
	}
	return lanes, nil
}

func (c *Client) ListColumns(ctx context.Context, boardID int) ([]json.RawMessage, error) {
	var columns []json.RawMessage
	if err := c.get(ctx, fmt.Sprintf("/boards/%d/columns", boardID), nil, &columns); err != nil {
		return nil, err
	}
	return columns, nil
}

func (c *Client) CardComments(ctx context.Context, id int) ([]json.RawMessage, error) {
	var comments []json.RawMessage
	if err := c.get(ctx, fmt.Sprintf("/cards/%d/comments", id), nil, &comments); err != nil {
		return nil, err
	}
	return comments, nil
}

func (c *Client) CardLocationHistory(ctx context.Context, id int) ([]json.RawMessage, error) {
	var history []json.RawMessage
	if err := c.get(ctx, fmt.Sprintf("/cards/%d/location-history", id), nil, &history); err != nil {
		return nil, err
	}
	return history, nil
}

func (c *Client) CardBaselines(ctx context.Context, id int) ([]json.RawMessage, error) {
	var baselines []json.RawMessage
	if err := c.get(ctx, fmt.Sprintf("/cards/%d/baselines", id), nil, &baselines); err != nil {
		return nil, err
	}
	return baselines, nil
}

func (c *Client) CreateCard(ctx context.Context, input map[string]any) (json.RawMessage, error) {
	return c.postJSON(ctx, "/cards", input)
}

func (c *Client) UpdateCard(ctx context.Context, id int, input map[string]any) (json.RawMessage, error) {
	return c.patchJSON(ctx, fmt.Sprintf("/cards/%d", id), input)
}

func (c *Client) CommentCard(ctx context.Context, id int, text string) (json.RawMessage, error) {
	return c.postJSON(ctx, fmt.Sprintf("/cards/%d/comments", id), map[string]any{"text": text})
}

func (c *Client) CardFiles(ctx context.Context, cardID int) ([]json.RawMessage, error) {
	var files []json.RawMessage
	if err := c.get(ctx, fmt.Sprintf("/cards/%d/files", cardID), nil, &files); err != nil {
		return nil, err
	}
	return files, nil
}

// AttachFile uploads a file to a card as multipart/form-data (field name "file").
func (c *Client) AttachFile(ctx context.Context, cardID int, filename string, content []byte) (json.RawMessage, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInvalidArgs, "build multipart body", err, nil)
	}
	if _, err := part.Write(content); err != nil {
		return nil, apperr.Wrap(apperr.CodeInvalidArgs, "write multipart body", err, nil)
	}
	if err := writer.Close(); err != nil {
		return nil, apperr.Wrap(apperr.CodeInvalidArgs, "close multipart body", err, nil)
	}
	data, err := c.requestRaw(ctx, http.MethodPost, fmt.Sprintf("/cards/%d/files", cardID), &buf, writer.FormDataContentType())
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

// DownloadFile fetches raw bytes from an absolute file URL (a card file's "url").
// It sends the Kaiten bearer token, which presigned storage URLs simply ignore.
func (c *Client) DownloadFile(ctx context.Context, fileURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, "", apperr.Wrap(apperr.CodeInvalidArgs, "build file download request", err, nil)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", apperr.Wrap(apperr.CodeKaitenAPI, "download file", err, nil)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", apperr.Wrap(apperr.CodeKaitenAPI, "read downloaded file", err, nil)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, "", apperr.New(apperr.CodeAuth, "file download authentication failed", map[string]any{"status": resp.StatusCode})
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, "", apperr.New(apperr.CodeNotFound, "file not found", map[string]any{"status": resp.StatusCode})
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", apperr.New(apperr.CodeKaitenAPI, "file download returned non-success status", map[string]any{"status": resp.StatusCode})
	}
	return body, resp.Header.Get("Content-Type"), nil
}

func (c *Client) get(ctx context.Context, path string, values url.Values, dst any) error {
	data, err := c.request(ctx, http.MethodGet, path, values, nil)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return apperr.Wrap(apperr.CodeKaitenAPI, "decode Kaiten response", err, nil)
	}
	return nil
}

func (c *Client) postJSON(ctx context.Context, path string, body any) (json.RawMessage, error) {
	data, err := c.doJSON(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func (c *Client) patchJSON(ctx context.Context, path string, body any) (json.RawMessage, error) {
	data, err := c.doJSON(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func (c *Client) doJSON(ctx context.Context, method string, path string, body any) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInvalidArgs, "encode request json", err, nil)
	}
	return c.request(ctx, method, path, nil, bytes.NewReader(data))
}

func (c *Client) request(ctx context.Context, method string, path string, values url.Values, bodyReader io.Reader) ([]byte, error) {
	return c.doRequest(ctx, method, path, values, bodyReader, "application/json")
}

// requestRaw sends a request with a caller-supplied Content-Type (e.g. multipart uploads).
func (c *Client) requestRaw(ctx context.Context, method, path string, bodyReader io.Reader, contentType string) ([]byte, error) {
	return c.doRequest(ctx, method, path, nil, bodyReader, contentType)
}

func (c *Client) doRequest(ctx context.Context, method string, path string, values url.Values, bodyReader io.Reader, contentType string) ([]byte, error) {
	u, err := url.Parse(c.baseURL + "/api/latest" + path)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeKaitenAPI, "build Kaiten request url", err, nil)
	}
	if values != nil {
		u.RawQuery = values.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeKaitenAPI, "build Kaiten request", err, nil)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeKaitenAPI, "call Kaiten API", err, map[string]string{
			"method": method,
			"url":    u.Redacted(),
			"error":  err.Error(),
		})
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeKaitenAPI, "read Kaiten response", err, nil)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, apperr.New(apperr.CodeAuth, "Kaiten authentication failed", map[string]any{"status": resp.StatusCode, "body": string(body)})
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, apperr.New(apperr.CodeNotFound, "Kaiten resource not found", map[string]any{"status": resp.StatusCode, "body": string(body)})
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		details := map[string]any{"status": resp.StatusCode, "body": string(body)}
		if resp.StatusCode == http.StatusTooManyRequests {
			details["rate_limit_remaining"] = resp.Header.Get("X-RateLimit-Remaining")
			details["rate_limit_reset"] = resp.Header.Get("X-RateLimit-Reset")
		}
		return nil, apperr.New(apperr.CodeKaitenAPI, "Kaiten API returned non-success status", details)
	}
	return body, nil
}

func CardID(card json.RawMessage) (int, error) {
	var data struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(card, &data); err != nil {
		return 0, apperr.Wrap(apperr.CodeKaitenAPI, "decode card id", err, nil)
	}
	if data.ID == 0 {
		return 0, apperr.New(apperr.CodeKaitenAPI, "card response has no id", nil)
	}
	return data.ID, nil
}
