# Engineering standards — learned by DevAgent

Battle-tested rules, each repeatedly confirmed by merged work.
Humans may edit; the agent treats this file as authoritative.

- Every `http.Server` literal must set `ReadHeaderTimeout`, `ReadTimeout`, and `WriteTimeout` before merging. _(confirmed 2x)_
- Every http.Server literal must set ReadHeaderTimeout, ReadTimeout, and WriteTimeout before any PR that adds or modifies request-body-accepting routes is merged. _(confirmed 2x)_
