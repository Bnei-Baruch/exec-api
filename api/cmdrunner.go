package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Default MQTT topic templates for the generic command runner. They can be
// overridden via the `mqtt.exec_cmd_topic` (subscribe prefix) and
// `mqtt.exec_cmd_data_topic` (publish prefix) viper keys. The defaults match
// the topics used by shidur-admin (the current frontend consumer).
const (
	defaultExecCmdTopic     = "exec/cmd/"
	defaultExecCmdDataTopic = "exec/cmd/data/"
)

// CmdRequest is the JSON payload accepted on the MQTT command topic and on
// the HTTP endpoint body.
type CmdRequest struct {
	RequestID string `json:"request_id,omitempty"`
	ID        string `json:"id"`
}

// CmdResponse is the JSON payload published on the MQTT response topic. The
// same shape (minus RequestID for HTTP) is returned by the HTTP endpoint.
//
// exit_code semantics:
//   - 0..N    : real exit code from the executed process
//   - -1      : the command was not run (rejected/timeout/system error)
type CmdResponse struct {
	RequestID  string `json:"request_id,omitempty"`
	ID         string `json:"id"`
	ExitCode   int    `json:"exit_code"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
	Timestamp  string `json:"timestamp"`
}

// execCmdSubscribeTopic returns the MQTT topic this instance must subscribe
// to in order to receive command requests for itself.
func execCmdSubscribeTopic() string {
	base := viper.GetString("mqtt.exec_cmd_topic")
	if base == "" {
		base = defaultExecCmdTopic
	}
	return base + viper.GetString("mqtt.client_id")
}

// execCmdResponseTopic returns the topic on which a response should be
// published. We mirror the QoS=0, Retained=false behaviour used by the rest
// of exec-api responses.
func execCmdResponseTopic() string {
	base := viper.GetString("mqtt.exec_cmd_data_topic")
	if base == "" {
		base = defaultExecCmdDataTopic
	}
	return base + viper.GetString("mqtt.client_id")
}

// CmdMessage is the MQTT subscription callback for the generic command runner.
// It parses the request, executes the matching whitelisted command and
// publishes the response back. Each invocation runs in its own goroutine so
// commands cannot block the MQTT receive loop.
func CmdMessage(c mqtt.Client, m mqtt.Message) {
	payload := m.Payload()
	log.Debugf("[CmdMessage] topic: %s | message: %s", m.Topic(), string(payload))

	go func(raw []byte) {
		resp := executeCmdPayload(raw)
		publishCmdResponse(resp)
	}(payload)
}

// publishCmdResponse marshals the response and publishes it on the cmd/data
// topic with QoS=0, Retained=false (matching SendRespond semantics).
func publishCmdResponse(resp CmdResponse) {
	topic := execCmdResponseTopic()
	body, err := json.Marshal(resp)
	if err != nil {
		log.Errorf("[CmdMessage] response marshal: %s", err)
		return
	}

	log.Debugf("[CmdMessage] respond topic: %s | message: %s", topic, string(body))
	if MQTT == nil {
		log.Errorf("[CmdMessage] MQTT client is nil, cannot publish response")
		return
	}
	if token := MQTT.Publish(topic, byte(0), false, body); token.Wait() && token.Error() != nil {
		log.Errorf("[CmdMessage] publish: %s", token.Error())
	}
}

// executeCmdPayload parses the request, validates it against the whitelist
// and runs the command. It always returns a populated CmdResponse — even on
// error — so the caller can publish a well-formed reply.
func executeCmdPayload(raw []byte) CmdResponse {
	resp := CmdResponse{
		ExitCode:  -1,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	var req CmdRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		resp.Error = fmt.Sprintf("invalid payload: %s", err)
		log.Errorf("[cmdrunner] invalid payload: %s | raw: %s", err, string(raw))
		return resp
	}
	resp.RequestID = req.RequestID
	resp.ID = req.ID

	if req.ID == "" {
		resp.Error = "invalid payload: missing id"
		log.Errorf("[cmdrunner] missing id in payload: %s", string(raw))
		return resp
	}

	return runAllowedCommandByID(req.RequestID, req.ID)
}

// runAllowedCommandByID looks up the command in the whitelist, runs it with
// the configured timeout and returns a fully-populated CmdResponse.
func runAllowedCommandByID(requestID, id string) CmdResponse {
	cfg, err := loadConfigForCmdRunner()
	if err != nil {
		start := time.Now()
		log.Errorf("[cmdrunner] failed to load config: %s", err)
		return CmdResponse{
			RequestID:  requestID,
			ID:         id,
			ExitCode:   -1,
			Error:      fmt.Sprintf("failed to load config: %s", err),
			Timestamp:  start.UTC().Format(time.RFC3339),
			DurationMs: 0,
		}
	}
	return runAllowedCommandFromConfig(cfg, requestID, id)
}

// runAllowedCommandFromConfig is the testable core: given an already-loaded
// config, validate the id and run the matching command. Exposed (lowercase
// but package-scoped) so tests can drive it without touching disk.
func runAllowedCommandFromConfig(cfg *Config, requestID, id string) CmdResponse {
	start := time.Now()
	resp := CmdResponse{
		RequestID: requestID,
		ID:        id,
		ExitCode:  -1,
		Timestamp: start.UTC().Format(time.RFC3339),
	}

	allowed := FindAllowedCommandByID(cfg, id)
	if allowed == nil {
		resp.Error = fmt.Sprintf("command not allowed: %s", id)
		log.Warnf("[cmdrunner] command not allowed: %s", id)
		resp.DurationMs = time.Since(start).Milliseconds()
		return resp
	}

	timeoutSec := normalizeTimeout(allowed.TimeoutSeconds)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, allowed.Cmd, allowed.Args...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	runErr := cmd.Run()
	resp.DurationMs = time.Since(start).Milliseconds()
	resp.Stdout = stdoutBuf.String()
	resp.Stderr = stderrBuf.String()
	resp.Timestamp = time.Now().UTC().Format(time.RFC3339)

	// Timeout takes priority over the generic exec error reporting.
	if ctx.Err() == context.DeadlineExceeded {
		resp.ExitCode = -1
		resp.Error = fmt.Sprintf("timeout after %ds", timeoutSec)
		log.Errorf("[cmdrunner] id=%s timeout after %ds (cmd=%s args=%v)",
			id, timeoutSec, allowed.Cmd, allowed.Args)
		return resp
	}

	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			resp.ExitCode = exitErr.ExitCode()
			log.Warnf("[cmdrunner] id=%s non-zero exit=%d duration=%dms stderr=%q",
				id, resp.ExitCode, resp.DurationMs, resp.Stderr)
			return resp
		}
		// System/start error (binary missing, perm denied, etc.).
		resp.ExitCode = -1
		resp.Error = runErr.Error()
		log.Errorf("[cmdrunner] id=%s system error: %s", id, runErr)
		return resp
	}

	resp.ExitCode = 0
	log.Infof("[cmdrunner] id=%s success duration=%dms", id, resp.DurationMs)
	return resp
}

// loadConfigForCmdRunner mirrors the dual-source config strategy used by
// existing exec-api handlers: local conf.json first, fall back to the
// configured remote cfg_url keyed on client_id.
func loadConfigForCmdRunner() (*Config, error) {
	cfg, err := getConf()
	if err == nil {
		return cfg, nil
	}
	clientID := viper.GetString("mqtt.client_id")
	cfg, err2 := getJson(clientID)
	if err2 != nil {
		return nil, fmt.Errorf("local: %v; remote: %v", err, err2)
	}
	return cfg, nil
}

// runCmdHTTP serves POST /:ep/cmd/:id. The endpoint is intentionally simple:
// the :id selects the whitelisted command, no request body is required.
// Any body containing a JSON request_id is honoured and echoed back.
func runCmdHTTP(c *gin.Context) {
	id := c.Params.ByName("id")

	var req CmdRequest
	// Body is optional. Ignore parse errors when it's empty; surface them
	// otherwise so callers get a clean 400.
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			NewBadRequestError(fmt.Errorf("invalid payload: %s", err)).Abort(c)
			return
		}
	}

	resp := runAllowedCommandByID(req.RequestID, id)

	// Map error categories to sensible HTTP statuses. Successful exec
	// (including non-zero process exit codes) returns 200 — the client
	// inspects exit_code/stderr to decide what to do.
	if resp.Error != "" && resp.ExitCode == -1 {
		// command not allowed → 404; everything else → 500.
		if isNotAllowedError(resp.Error, id) {
			c.JSON(http.StatusNotFound, resp)
			return
		}
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func isNotAllowedError(errMsg, id string) bool {
	return errMsg == fmt.Sprintf("command not allowed: %s", id)
}
