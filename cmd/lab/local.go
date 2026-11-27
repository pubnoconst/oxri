package main

import (
	"context"
	"net/http/httptest"

	"github.com/oxri/oxri-go/internal/labsite"
)

// startLocal serves the seeded site (internal/labsite) in this process on
// loopback, for -local.
func startLocal(ctx context.Context, entries, pageSize int) (base string, stop func(), err error) {
	site, err := labsite.New(ctx, entries, pageSize)
	if err != nil {
		return "", nil, err
	}
	localSite = site
	srv := httptest.NewServer(site.Handler())
	return srv.URL, func() { srv.Close(); site.Close() }, nil
}

// localSite is the seeded site of -local, for its count of the photos it
// served in place of X.
var localSite *labsite.Site
