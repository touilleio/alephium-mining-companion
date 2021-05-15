package main

import (
	"github.com/sqooba/go-common/healthchecks"
	"net/http"
)

func initHealthChecks(env envConfig, mux *http.ServeMux) {
	healthchecks.RegisterHealthCheck("always-ok", alwaysOk)
}

// checkAlwaysOK is an always passing check.
func alwaysOk() error {
	return nil
}
