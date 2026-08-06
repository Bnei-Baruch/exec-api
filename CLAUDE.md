# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`exec-api` is a small Go service deployed on individual capture/ingest stations (Bnei-Baruch broadcast pipeline). It has two jobs:

1. **Process runner** — start/stop/monitor OS-level commands (almost always `ffmpeg`) over both a REST API and MQTT, keyed by an `id` string.
2. **Capture workflow** — react to MQTT `start`/`line`/`stop` events for a recording, persist capture state to disk, and report progress to two external services (MDB and WFDB) plus a workflow API.

There are no tests, no Makefile, no Dockerfile, and no CI config in this repo — build/lint/run only via the Go toolchain directly.

## Commands

```sh
go build ./...          # compile everything
go run .                # run the server (needs config.* — see Configuration below)
go vet ./...            # static checks
gofmt -l .              # list files needing formatting
go test ./...           # no test files currently exist; this is a no-op
```

`go.mod` declares `go 1.15`, but the toolchain installed in this environment is newer — `go vet` currently flags two pre-existing `%s`/int64 format warnings in `wf/flows.go` (lines ~218, ~224) that are not related to work you're doing unless you touch that code.

There is no `main_test.go` pattern to follow yet; if you add tests, standard `go test ./<pkg>/... -run TestName` applies.

## Configuration

Config is loaded by [cmd/exec.go](cmd/exec.go) via Viper: it reads `config.<ext>` (yaml/json/toml/...) from the current working directory, and env vars override it with `.`/`-` replaced by `_` (e.g. `mqtt.client_id` → `MQTT_CLIENT_ID`). No sample config file is checked into the repo — reconstruct one from the keys actually read in code:

- `server.log`, `server.mode`, `server.addr`, `server.cfg_url`
- `authentication.enable`, `authentication.issuer` (OIDC/Keycloak issuer URL)
- `mqtt.debug`, `mqtt.url`, `mqtt.client_id`, `mqtt.user`, `mqtt.password`, `mqtt.status_topic`, `mqtt.exec_service_topic`, `mqtt.exec_state_topic`, `mqtt.exec_data_topic`, `mqtt.wf_service_topic`, `mqtt.wf_state_topic`, `mqtt.wf_data_topic`
- `workflow.capture_path`, `workflow.mdb_url`, `workflow.wfdb_url`, `workflow.wfapi_url`, `workflow.mlt_main`, `workflow.mlt_backup`, `workflow.main_cap`, `workflow.backup_cap`, `workflow.arch_cap`, `workflow.arch_backup`

`mqtt.client_id` is the most load-bearing value: it identifies which station role this instance plays (`mltcap`, `mltbackup`, `maincap`, `backupcap`, `archcap`, `archbackup`, `testcap`, `livecap1`, `livecap2`, ...), and multiple call sites (`api/exec.go`, `wf/flows.go`) branch on it — usually with a regex or a literal switch — to decide whether "ingest capture" logic applies and which capture/backup ID (`wf.CaptureState.CaptureID` vs `BackupID`) and MDB "station" (`wf.GetStationID`) to use. When adding a new station role, grep for existing role names (e.g. `archcap`) across `api/` and `wf/` — the set of places that need updating is not centralized.

## Architecture

**Startup path**: `main.go` → `cmd.Exec()` (loads Viper config) → `cmd.Init()` ([cmd/init.go](cmd/init.go)), which wires up logging, CORS, an optional OIDC verifier, the MQTT client (`api.InitMQTT`), a Gin router with a fixed middleware chain, and `api.SetupRoutes`, then blocks on `http.ListenAndServe`.

**Two parallel control planes drive the same state.** Every "start/stop/status a process" action is reachable both as an HTTP endpoint ([api/routes.go](api/routes.go), handlers in [api/rest.go](api/rest.go)) and as an MQTT message (subscribed in [api/mqtt.go](api/mqtt.go) `SubMQTT`, handlers in [api/exec.go](api/exec.go)). Both paths bottleneck on the same package-level `api.Cmd map[string]*cmd.Cmd` (from `go-cmd/cmd`), keyed by service `id`. This map is unguarded by a mutex — concurrent start/stop of the same id races on it. MQTT topics follow the pattern `.../<action>` or `.../<action>/<id>`; REST routes are `/:ep/<action>[/:id]` where `ep` is an "endpoint"/config namespace.

**Where "what to run" comes from**: a `Config` (with a list of `Service{ID, Name, Description, Args}`) is read either from a local `conf.json` (`getConf` in [api/tools.go](api/tools.go)) or, if that's missing, fetched over HTTP from `server.cfg_url + ep` (`getJson`). `Name` is the binary to exec (e.g. `ffmpeg`) and `Args` its argv. For `ffmpeg` specifically, `FFREPORT` env var is set so ffmpeg writes a `report_<id>.log`, and `-progress stat_<id>.log`-style output is tailed by the `*progress`/`*Stat` endpoints to answer status/progress queries. These log files live in the process's working directory, not under `workflow.capture_path`.

**Ingest capture workflow** ([wf/flows.go](wf/flows.go), [wf/capture.go](wf/capture.go)): triggered by MQTT messages on `mqtt.wf_service_topic` (`wf.MqttMessage` → `StartFlow`/`LineFlow`/`StopFlow`). Capture state (`CaptureState`, including the current `CaptureID`/`BackupID` and `Line` metadata like lecturer, collection, part) is pushed into this process from elsewhere over `mqtt.wf_state_topic` (`wf.SetState`) and persisted to `state.json` in the working directory; `GetState` re-reads that file, while `GetMemState` re-decodes the last in-memory MQTT payload (`wf.Data`) instead of hitting disk. `StartFlow`/`LineFlow`/`StopFlow` each POST/PUT to two external systems — MDB (`workflow.mdb_url`, capture start/stop events) and WFDB (`workflow.wfdb_url`, the `WfdbCapture`/`Line` record) — and `StopFlow` additionally computes the finished file's sha1, renames it in `workflow.capture_path`, and PUTs a `CaptureFlow` record to `workflow.wfapi_url`. Each flow step also shells out to `/opt/exec-api/<start|line|stop>.sh` (`CheckJson`) as a side-effect hook — those scripts live outside this repo.

Source-specific branching in `StopFlow` (main multi-capture vs. archive vs. backup-of-archive vs. multi-backup) changes `ContentType`/`Part`/`StopName` differently per `mqtt.client_id` role — read that function fully before modifying capture-stop behavior for any one role, since the branches are not independent of each other (backup roles rewrite `StopName` to a "full" variant that the primary-capture roles don't).

**Auth**: OIDC bearer-token verification is optional (`authentication.enable`) and, when on, is enforced in [utils/authentication.go](utils/authentication.go)'s `AuthenticationMiddleware`, which expects the verifier injected into the Gin context by `EnvMiddleware` ([utils/middleware.go](utils/middleware.go)). Middleware order in `cmd/init.go` matters: CORS → env (verifier injection) → error handling → authentication → recovery.

**Uploads**: `POST /upload/:ep` ([api/rest.go](api/rest.go) `HandleUploadGin`) streams a multipart request straight to disk under a per-`ep` directory (`getUploadPath` switch — `insert`, `jobs`, `products`, `aricha`, `aklada`, `gibuy`, `carbon`, `dgima`, `proxy`, `youtube`, `coder`, `muxer`, default `/backup/tmp/upload/`), then computes sha1/size after the fact. These paths are hardcoded, not config-driven.
