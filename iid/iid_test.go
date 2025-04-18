// Copyright 2017 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package iid

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/galinkhq/firebase-admin-go/errorutils"
	"github.com/galinkhq/firebase-admin-go/internal"
	"google.golang.org/api/option"
)

var testIIDConfig = &internal.InstanceIDConfig{
	ProjectID: "test-project",
	Opts: []option.ClientOption{
		option.WithTokenSource(&internal.MockTokenSource{AccessToken: "test-token"}),
	},
	Version: "test-version",
}

func TestNoProjectID(t *testing.T) {
	client, err := NewClient(context.Background(), &internal.InstanceIDConfig{})
	if client != nil || err == nil {
		t.Errorf("NewClient() = (%v, %v); want = (nil, error)", client, err)
	}
}

func TestInvalidInstanceID(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(ctx, testIIDConfig)
	if err != nil {
		t.Fatal(err)
	}

	if err := client.DeleteInstanceID(ctx, ""); err == nil {
		t.Errorf("DeleteInstanceID(empty) = nil; want error")
	}
}

func TestDeleteInstanceID(t *testing.T) {
	var tr *http.Request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr = r
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testIIDConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.endpoint = ts.URL
	if err := client.DeleteInstanceID(ctx, "test-iid"); err != nil {
		t.Errorf("DeleteInstanceID() = %v; want nil", err)
	}

	if tr == nil {
		t.Fatalf("Request = nil; want non-nil")
	}
	if tr.Method != http.MethodDelete {
		t.Errorf("Method = %q; want = %q", tr.Method, http.MethodDelete)
	}
	if tr.URL.Path != "/project/test-project/instanceId/test-iid" {
		t.Errorf("Path = %q; want = %q", tr.URL.Path, "/project/test-project/instanceId/test-iid")
	}
	if h := tr.Header.Get("Authorization"); h != "Bearer test-token" {
		t.Errorf("Authorization = %q; want = %q", h, "Bearer test-token")
	}
	xGoogAPIClientHeader := internal.GetMetricsHeader(testIIDConfig.Version)
	if h := tr.Header.Get("x-goog-api-client"); h != xGoogAPIClientHeader {
		t.Errorf("x-goog-api-client header = %q; want = %q", h, xGoogAPIClientHeader)
	}
}

func TestDeleteInstanceIDError(t *testing.T) {
	status := http.StatusOK
	var tr *http.Request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr = r
		w.WriteHeader(status)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testIIDConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.endpoint = ts.URL
	client.client.RetryConfig = nil

	errorHandlers := map[int]func(error) bool{
		http.StatusBadRequest:          errorutils.IsInvalidArgument,
		http.StatusUnauthorized:        errorutils.IsUnauthenticated,
		http.StatusForbidden:           errorutils.IsPermissionDenied,
		http.StatusNotFound:            errorutils.IsNotFound,
		http.StatusConflict:            errorutils.IsConflict,
		http.StatusTooManyRequests:     errorutils.IsResourceExhausted,
		http.StatusInternalServerError: errorutils.IsInternal,
		http.StatusServiceUnavailable:  errorutils.IsUnavailable,
	}

	deprecatedErrorHandlers := map[int]func(error) bool{
		http.StatusBadRequest:          IsInvalidArgument,
		http.StatusUnauthorized:        IsInsufficientPermission,
		http.StatusForbidden:           IsInsufficientPermission,
		http.StatusNotFound:            IsNotFound,
		http.StatusConflict:            IsAlreadyDeleted,
		http.StatusTooManyRequests:     IsTooManyRequests,
		http.StatusInternalServerError: IsInternal,
		http.StatusServiceUnavailable:  IsServerUnavailable,
	}

	for code, check := range errorHandlers {
		status = code
		want := fmt.Sprintf("instance id %q: %s", "test-iid", errorMessages[code])
		err := client.DeleteInstanceID(ctx, "test-iid")
		if err == nil || !check(err) || err.Error() != want {
			t.Errorf("DeleteInstanceID() = %v; want = %v", err, want)
		}

		resp := errorutils.HTTPResponse(err)
		if resp.StatusCode != code {
			t.Errorf("HTTPResponse().StatusCode = %d; want = %d", resp.StatusCode, code)
		}

		deprecatedCheck := deprecatedErrorHandlers[code]
		if !deprecatedCheck(err) {
			t.Errorf("DeleteInstanceID() = %v; want = %v", err, want)
		}

		if tr == nil {
			t.Fatalf("Request = nil; want non-nil")
		}
		if tr.Method != http.MethodDelete {
			t.Errorf("Method = %q; want = %q", tr.Method, http.MethodDelete)
		}
		if tr.URL.Path != "/project/test-project/instanceId/test-iid" {
			t.Errorf("Path = %q; want = %q", tr.URL.Path, "/project/test-project/instanceId/test-iid")
		}
		if h := tr.Header.Get("Authorization"); h != "Bearer test-token" {
			t.Errorf("Authorization = %q; want = %q", h, "Bearer test-token")
		}
		xGoogAPIClientHeader := internal.GetMetricsHeader(testIIDConfig.Version)
		if h := tr.Header.Get("x-goog-api-client"); h != xGoogAPIClientHeader {
			t.Errorf("x-goog-api-client header = %q; want = %q", h, xGoogAPIClientHeader)
		}
		tr = nil
	}
}

func TestDeleteInstanceIDUnexpectedError(t *testing.T) {
	var tr *http.Request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr = r
		w.WriteHeader(511)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	}))
	defer ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testIIDConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.endpoint = ts.URL

	want := "unexpected http response with status: 511\n{}"
	err = client.DeleteInstanceID(ctx, "test-iid")
	if err == nil || err.Error() != want {
		t.Errorf("DeleteInstanceID() = %v; want = %v", err, want)
	}
	if !IsUnknown(err) {
		t.Errorf("IsUnknown() = false; want = true")
	}
	if !errorutils.IsUnknown(err) {
		t.Errorf("errorutils.IsUnknown() = false; want = true")
	}

	if tr == nil {
		t.Fatalf("Request = nil; want non-nil")
	}
	if tr.Method != http.MethodDelete {
		t.Errorf("Method = %q; want = %q", tr.Method, http.MethodDelete)
	}
	if tr.URL.Path != "/project/test-project/instanceId/test-iid" {
		t.Errorf("Path = %q; want = %q", tr.URL.Path, "/project/test-project/instanceId/test-iid")
	}
	if h := tr.Header.Get("Authorization"); h != "Bearer test-token" {
		t.Errorf("Authorization = %q; want = %q", h, "Bearer test-token")
	}
	xGoogAPIClientHeader := internal.GetMetricsHeader(testIIDConfig.Version)
	if h := tr.Header.Get("x-goog-api-client"); h != xGoogAPIClientHeader {
		t.Errorf("x-goog-api-client header = %q; want = %q", h, xGoogAPIClientHeader)
	}
}

func TestDeleteInstanceIDConnectionError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do nothing
	}))
	ts.Close()

	ctx := context.Background()
	client, err := NewClient(ctx, testIIDConfig)
	if err != nil {
		t.Fatal(err)
	}
	client.endpoint = ts.URL
	client.client.RetryConfig = nil

	if err := client.DeleteInstanceID(ctx, "test-iid"); err == nil {
		t.Fatalf("DeleteInstanceID() = nil; want = error")
	}
}
