package operations

import (
	"context"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var serviceUnitPattern = regexp.MustCompile("^[A-Za-z0-9_.:@-]{1,180}\\.service$")

type Result struct {
	Status string `json:"status"`
	Output string `json:"output"`
}

const maxOutputBytes = 2048

// Execute runs one fixed systemctl verb against a service unit present in the
// agent's local allowlist. It never invokes a shell or accepts command text.
func Execute(parent context.Context, operation, unit string, allowedUnits []string) Result {
	if !serviceUnitPattern.MatchString(unit) {
		return Result{Status: "failed", Output: "Target is not a valid systemd .service unit."}
	}
	allowed := false
	for _, candidate := range allowedUnits {
		if candidate == unit {
			allowed = true
			break
		}
	}
	if !allowed {
		return Result{Status: "failed", Output: "Target is not in the agent's local systemd allowlist."}
	}

	verb := ""
	switch operation {
	case "restart_service":
		verb = "restart"
	case "stop_service":
		verb = "stop"
	default:
		return Result{Status: "failed", Output: "Remote operation type is not supported."}
	}

	ctx, cancel := context.WithTimeout(parent, 25*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "systemctl", verb, unit)
	output, err := command.CombinedOutput()
	message := strings.TrimSpace(string(output))
	if len(message) > maxOutputBytes {
		message = message[:maxOutputBytes]
	}
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return Result{Status: "failed", Output: "systemctl operation timed out after 25 seconds"}
		}
		if message == "" {
			message = "systemctl operation failed"
		}
		return Result{Status: "failed", Output: message}
	}
	if message == "" {
		message = "systemctl " + verb + " completed"
	}
	return Result{Status: "succeeded", Output: message}
}
