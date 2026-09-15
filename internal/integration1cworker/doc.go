// Package integration1cworker implements a reusable PostgreSQL-backed asynchronous
// command worker for a goCOM1c HTTP gateway. It owns only the integration_1c
// queue/journal schema and dispatches terminal results to project-provided handlers.
package integration1cworker
