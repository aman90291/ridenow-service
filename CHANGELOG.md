# Changelog

## Unreleased
- SCRUM-171: SQLite persistence via modernc.org/sqlite (pure-Go, no CGO) (DevAgent)
- SCRUM-170: GET /healthz delivered/verified as a 200 JSON liveness+readiness probe returning {"status":"ok","db":"ok"} (503 {"status":"degraded","db":"unreachable"} when the DB ping fails); locked the schema with TestHealthzOK/TestHealthzDegraded and set request timeouts on the main.go http.Server literal (DevAgent)
