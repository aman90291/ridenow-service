# Engineering standards — learned by DevAgent

Battle-tested rules, each repeatedly confirmed by merged work.
Humans may edit; the agent treats this file as authoritative.

- Every `http.Server` literal must set `ReadHeaderTimeout`, `ReadTimeout`, and `WriteTimeout` before merging — this applies to every PR, including those that add or modify request-body-accepting routes. _(confirmed 4x)_
