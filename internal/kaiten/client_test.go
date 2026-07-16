package kaiten

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestClientUsesBearerAuthAndFiltersCards(t *testing.T) {
	var gotQuery string
	client := NewClientWithHTTP("https://kaiten.test", "token", &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/api/latest/cards" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		return jsonResponse(http.StatusOK, `[{"id":1,"title":"bug"}]`), nil
	})})

	cards, err := client.ListCards(context.Background(), CardFilters{
		SpaceID:            1,
		BoardID:            2,
		LaneID:             3,
		MemberIDs:          "4",
		ResponsibleIDs:     "5",
		States:             "1,2",
		IncludeDescription: true,
		Limit:              50,
		Offset:             10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 {
		t.Fatalf("len(cards) = %d", len(cards))
	}
	for _, want := range []string{
		"space_id=1", "board_id=2", "lane_id=3", "member_ids=4", "responsible_ids=5",
		"states=1%2C2", "additional_card_fields=description", "limit=50", "offset=10", "broken_api=false",
	} {
		if !containsQuery(gotQuery, want) {
			t.Fatalf("query %q missing %q", gotQuery, want)
		}
	}
}

func TestClientWriteBodies(t *testing.T) {
	seen := map[string]map[string]any{}
	client := NewClientWithHTTP("https://kaiten.test", "token", &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		seen[r.Method+" "+r.URL.Path] = body
		return jsonResponse(http.StatusOK, `{"id":1}`), nil
	})})

	if _, err := client.CreateCard(context.Background(), map[string]any{"title": "new"}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.UpdateCard(context.Background(), 1, map[string]any{"title": "updated"}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.CommentCard(context.Background(), 1, "hello"); err != nil {
		t.Fatal(err)
	}
	if seen["POST /api/latest/cards"]["title"] != "new" {
		t.Fatalf("create body = %#v", seen)
	}
	if seen["PATCH /api/latest/cards/1"]["title"] != "updated" {
		t.Fatalf("update body = %#v", seen)
	}
	if seen["POST /api/latest/cards/1/comments"]["text"] != "hello" {
		t.Fatalf("comment body = %#v", seen)
	}
}

func TestClientErrors(t *testing.T) {
	cases := []struct {
		status int
		code   string
	}{
		{http.StatusUnauthorized, "auth_error"},
		{http.StatusForbidden, "auth_error"},
		{http.StatusNotFound, "not_found"},
		{http.StatusTooManyRequests, "kaiten_api_error"},
	}
	for _, tc := range cases {
		client := NewClientWithHTTP("https://kaiten.test", "token", &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(tc.status, `error`), nil
		})})
		_, err := client.CurrentUser(context.Background())
		if err == nil || err.Error() == "" {
			t.Fatalf("status %d: expected error", tc.status)
		}
	}
}

func TestGetCardSpacesBoardsLanes(t *testing.T) {
	paths := map[string]string{
		"/api/latest/cards/7":         `{"id":7}`,
		"/api/latest/spaces":          `[{"id":1}]`,
		"/api/latest/spaces/1/boards": `[{"id":2}]`,
		"/api/latest/boards/2/lanes":  `[{"id":3}]`,
		"/api/latest/users/current":   `{"id":4,"username":"me"}`,
	}
	client := NewClientWithHTTP("https://kaiten.test", "token", &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, ok := paths[r.URL.Path]
		if !ok {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		return jsonResponse(http.StatusOK, body), nil
	})})

	if _, err := client.GetCard(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListSpaces(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListBoards(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListLanes(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	if err := client.VerifyToken(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func containsQuery(query, want string) bool {
	for i := 0; i+len(want) <= len(query); i++ {
		if query[i:i+len(want)] == want {
			return true
		}
	}
	return false
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
