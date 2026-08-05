// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"strings"
	"testing"

	pb "github.com/opentelemetry/opentelemetry-demo/src/product-catalog/genproto/oteldemo"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// These tests exercise the handlers against the real catalog, which init()
// loads from ./products before any test runs.
//
// knownProductID is deliberately NOT OLJCESPC7Z: GetProduct consults
// OpenFeature for that one ID only, so avoiding it keeps these tests free of a
// flagd dependency. Covering the productCatalogFailure path needs an in-memory
// provider and is left as a follow-up.
const (
	knownProductID   = "66VCHSJNUP"
	knownProductName = "Starsense Explorer Refractor Telescope"
)

func TestReadProductFiles(t *testing.T) {
	products, err := readProductFiles()
	if err != nil {
		t.Fatalf("readProductFiles() returned error: %v", err)
	}
	if len(products) == 0 {
		t.Fatal("readProductFiles() returned no products")
	}

	for _, p := range products {
		if p.Id == "" {
			t.Errorf("product %q has an empty ID", p.Name)
		}
		if p.Name == "" {
			t.Errorf("product %q has an empty name", p.Id)
		}
		if p.PriceUsd == nil {
			t.Errorf("product %s (%q) has no price", p.Id, p.Name)
		}
	}
}

// A duplicate ID would make GetProduct silently unreachable for one of them,
// since the lookup breaks on the first match.
func TestReadProductFilesHasUniqueIDs(t *testing.T) {
	products, err := readProductFiles()
	if err != nil {
		t.Fatalf("readProductFiles() returned error: %v", err)
	}

	seen := make(map[string]string, len(products))
	for _, p := range products {
		if prev, dup := seen[p.Id]; dup {
			t.Errorf("duplicate product ID %s shared by %q and %q", p.Id, prev, p.Name)
		}
		seen[p.Id] = p.Name
	}
}

func TestListProducts(t *testing.T) {
	svc := &productCatalog{}

	res, err := svc.ListProducts(context.Background(), &pb.Empty{})
	if err != nil {
		t.Fatalf("ListProducts() returned error: %v", err)
	}
	if got, want := len(res.Products), len(catalog); got != want {
		t.Errorf("ListProducts() returned %d products, want %d", got, want)
	}
}

func TestGetProduct(t *testing.T) {
	svc := &productCatalog{}

	tests := []struct {
		name     string
		id       string
		wantCode codes.Code
		wantName string
	}{
		{name: "known product", id: knownProductID, wantCode: codes.OK, wantName: knownProductName},
		{name: "unknown product", id: "NO-SUCH-PRODUCT", wantCode: codes.NotFound},
		{name: "empty id", id: "", wantCode: codes.NotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.GetProduct(context.Background(), &pb.GetProductRequest{Id: tt.id})
			if code := status.Code(err); code != tt.wantCode {
				t.Fatalf("GetProduct(%q) returned code %v, want %v (err: %v)", tt.id, code, tt.wantCode, err)
			}
			if tt.wantCode != codes.OK {
				return
			}
			if got.Id != tt.id {
				t.Errorf("GetProduct(%q) returned ID %q", tt.id, got.Id)
			}
			if got.Name != tt.wantName {
				t.Errorf("GetProduct(%q) returned name %q, want %q", tt.id, got.Name, tt.wantName)
			}
		})
	}
}

func TestSearchProducts(t *testing.T) {
	svc := &productCatalog{}

	tests := []struct {
		name      string
		query     string
		wantEmpty bool
	}{
		{name: "matches on name", query: "Telescope"},
		{name: "matches on description", query: "solar"},
		{name: "matches nothing", query: "definitely-not-a-product", wantEmpty: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := svc.SearchProducts(context.Background(), &pb.SearchProductsRequest{Query: tt.query})
			if err != nil {
				t.Fatalf("SearchProducts(%q) returned error: %v", tt.query, err)
			}

			if tt.wantEmpty {
				if len(res.Results) != 0 {
					t.Errorf("SearchProducts(%q) returned %d results, want 0", tt.query, len(res.Results))
				}
				return
			}

			if len(res.Results) == 0 {
				t.Fatalf("SearchProducts(%q) returned no results", tt.query)
			}
			q := strings.ToLower(tt.query)
			for _, p := range res.Results {
				if !strings.Contains(strings.ToLower(p.Name), q) &&
					!strings.Contains(strings.ToLower(p.Description), q) {
					t.Errorf("SearchProducts(%q) returned %q, which matches neither its name nor its description", tt.query, p.Name)
				}
			}
		})
	}
}

func TestSearchProductsIsCaseInsensitive(t *testing.T) {
	svc := &productCatalog{}
	ctx := context.Background()

	lower, err := svc.SearchProducts(ctx, &pb.SearchProductsRequest{Query: "telescope"})
	if err != nil {
		t.Fatalf("SearchProducts(lowercase) returned error: %v", err)
	}
	upper, err := svc.SearchProducts(ctx, &pb.SearchProductsRequest{Query: "TELESCOPE"})
	if err != nil {
		t.Fatalf("SearchProducts(uppercase) returned error: %v", err)
	}

	if len(lower.Results) == 0 {
		t.Fatal(`SearchProducts("telescope") returned no results`)
	}
	if len(lower.Results) != len(upper.Results) {
		t.Errorf("query case changed the result count: %d for %q vs %d for %q",
			len(lower.Results), "telescope", len(upper.Results), "TELESCOPE")
	}
}

// Documents current behaviour rather than endorsing it: the handler uses
// strings.Contains, and strings.Contains(s, "") is always true, so an empty
// query returns the entire catalog instead of nothing.
func TestSearchProductsEmptyQueryReturnsEverything(t *testing.T) {
	svc := &productCatalog{}

	res, err := svc.SearchProducts(context.Background(), &pb.SearchProductsRequest{Query: ""})
	if err != nil {
		t.Fatalf(`SearchProducts("") returned error: %v`, err)
	}
	if got, want := len(res.Results), len(catalog); got != want {
		t.Errorf(`SearchProducts("") returned %d results, want %d`, got, want)
	}
}

// The gRPC health server backs the readiness and liveness probes in
// kubernetes/productcatalog/deploy.yaml.
func TestHealthCheckReportsServing(t *testing.T) {
	svc := &productCatalog{}

	res, err := svc.Check(context.Background(), &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("Check() returned error: %v", err)
	}
	if res.Status != healthpb.HealthCheckResponse_SERVING {
		t.Errorf("Check() returned status %v, want SERVING", res.Status)
	}
}

// List backs the same probes as Check and is required by the HealthServer
// interface from grpc-go 1.83 onward.
func TestHealthListReportsServing(t *testing.T) {
	svc := &productCatalog{}

	res, err := svc.List(context.Background(), &healthpb.HealthListRequest{})
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if len(res.Statuses) == 0 {
		t.Fatal("List() reported no services")
	}
	for name, st := range res.Statuses {
		if st.Status != healthpb.HealthCheckResponse_SERVING {
			t.Errorf("List() reported %q as %v, want SERVING", name, st.Status)
		}
	}
}

func TestWatchIsUnimplemented(t *testing.T) {
	svc := &productCatalog{}

	err := svc.Watch(&healthpb.HealthCheckRequest{}, nil)
	if code := status.Code(err); code != codes.Unimplemented {
		t.Errorf("Watch() returned code %v, want Unimplemented", code)
	}
}
