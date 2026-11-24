//go:build windows

package logcollector

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"example.com/monitor-me/agent/internal/logspool"
)

const maxPowerShellOutput = 2 << 20

var (
	windowsAssignmentSecret = regexp.MustCompile(`(?i)\b(password|passwd|token|secret|api[_-]?key|authorization)\b(\s*[:=]\s*)(?:"[^"]*"|'[^']*'|[^\s,;]+)`)
	windowsBearerSecret = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]+`)
)

type Journal struct {
	binary string
}

type windowsEventRecord struct {
	LogName string `json:"log_name"`
	RecordID int64 `json:"record_id"`
	EventID uint32 `json:"event_id"`
	EventTimeUnixNanos int64 `json:"event_time_unix_nanos"`
	Provider string `json:"provider"`
	Level int `json:"level"`
	Message string `json:"message"`
	Computer string `json:"computer"`
}

type windowsEventResponse struct {
	Cursor string `json:"cursor"`
	Events []windowsEventRecord `json:"events"`
}

func NewJournal() *Journal {
	return &Journal{binary: "powershell.exe"}
}

func (j *Journal) Available() bool {
	_, err := exec.LookPath(j.binary)
	return err == nil
}

func (j *Journal) Capability() string { return "logs.windows_eventlog" }

func (j *Journal) PollInterval() time.Duration { return 10 * time.Second }

// Collect reads bounded Application and System event-channel pages. The
// Security channel is intentionally excluded because it contains sensitive
// audit data and may require a separate access-policy decision.
func (j *Journal) Collect(ctx context.Context, cursor string, limit int) ([]logspool.Event, string, error) {
	if limit < 1 || limit > 32 {
		return nil, cursor, errors.New("Windows Event Log batch limit must be between 1 and 32")
	}
	if !j.Available() {
		return nil, cursor, errors.New("PowerShell is not available for Windows Event Log collection")
	}

	state := make(map[string]int64)
	if cursor != "" {
		if err := json.Unmarshal([]byte(cursor), &state); err != nil {
			return nil, cursor, fmt.Errorf("decode Windows Event Log cursor: %w", err)
		}
		for channel, recordID := range state {
			if (channel != "Application" && channel != "System") || recordID < 0 {
				return nil, cursor, errors.New("Windows Event Log cursor contains an unsupported channel or record ID")
			}
		}
	}
	cursorJSON, err := json.Marshal(state)
	if err != nil {
		return nil, cursor, fmt.Errorf("encode Windows Event Log cursor: %w", err)
	}
	script := strings.ReplaceAll(windowsEventLogScript, "__CURSOR_BASE64__", base64.StdEncoding.EncodeToString(cursorJSON))
	script = strings.ReplaceAll(script, "__MAX_EVENTS__", strconv.Itoa(limit))

	command := exec.CommandContext(ctx, j.binary, "-NoLogo", "-NoProfile", "-NonInteractive", "-EncodedCommand", encodePowerShell(script))
	var stdout, stderr limitedBuffer
	stdout.limit = maxPowerShellOutput
	stderr.limit = 64 << 10
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, cursor, ctx.Err()
		}
		message := strings.TrimSpace(stderr.String())
		if len(message) > 512 {
			message = message[:512]
		}
		if message == "" {
			return nil, cursor, fmt.Errorf("Windows Event Log query failed: %w", err)
		}
		return nil, cursor, fmt.Errorf("Windows Event Log query failed: %s", message)
	}
	if ctx.Err() != nil {
		return nil, cursor, ctx.Err()
	}

	var response windowsEventResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		return nil, cursor, fmt.Errorf("decode Windows Event Log response: %w", err)
	}
	if len(response.Events) > limit {
		return nil, cursor, errors.New("Windows Event Log reader exceeded its page limit")
	}
	if len(response.Events) == 0 {
		return nil, cursor, nil
	}
	if len(response.Cursor) == 0 || len(response.Cursor) > 1024 {
		return nil, cursor, errors.New("Windows Event Log reader returned an invalid cursor")
	}

	events := make([]logspool.Event, 0, len(response.Events))
	observedAt := time.Now().UTC()
	for _, record := range response.Events {
		event, ok := normalizeWindowsEvent(record, observedAt)
		if ok {
			events = append(events, event)
		}
	}
	if len(events) == 0 {
		return nil, cursor, errors.New("Windows Event Log page contained no valid event records")
	}
	return events, response.Cursor, nil
}

func normalizeWindowsEvent(record windowsEventRecord, observedAt time.Time) (logspool.Event, bool) {
	if record.LogName != "Application" && record.LogName != "System" {
		return logspool.Event{}, false
	}
	if record.RecordID <= 0 || record.EventTimeUnixNanos <= 0 {
		return logspool.Event{}, false
	}

	source := record.LogName
	if record.Provider != "" {
		source += "/" + record.Provider
	}
	source = boundedWindows(source, 128)
	body := strings.ReplaceAll(record.Message, "\x00", "")
	if body == "" {
		body = "[Windows event without a rendered message]"
	}
	body = windowsBearerSecret.ReplaceAllString(body, "Bearer [REDACTED]")
	body = windowsAssignmentSecret.ReplaceAllString(body, "$1$2[REDACTED]")
	if len(body) > 16<<10 {
		body = strings.ToValidUTF8(body[:16<<10], "")
	}

	fingerprint := fmt.Sprintf("%s|%d|%d|%d", record.LogName, record.RecordID, record.EventID, record.EventTimeUnixNanos)
	hash := sha256.Sum256([]byte(fingerprint))
	attributes := map[string]string{
		"event_log": record.LogName,
		"record_id": strconv.FormatInt(record.RecordID, 10),
		"event_id": strconv.FormatUint(uint64(record.EventID), 10),
	}
	if record.Provider != "" {
		attributes["provider"] = boundedWindows(record.Provider, 256)
	}
	if record.Computer != "" {
		attributes["hostname"] = boundedWindows(record.Computer, 253)
	}
	return logspool.Event{
		ID: hex.EncodeToString(hash[:]),
		EventTimeUnixNanos: record.EventTimeUnixNanos,
		ObservedAtUnixNanos: observedAt.UnixNano(),
		Source: source,
		Severity: windowsSeverity(record.Level),
		Body: body,
		Attributes: attributes,
	}, true
}

func windowsSeverity(level int) string {
	switch level {
	case 1:
		return "critical"
	case 2:
		return "error"
	case 3:
		return "warning"
	case 4:
		return "info"
	case 5:
		return "debug"
	default:
		return "unknown"
	}
}

func boundedWindows(value string, limit int) string {
	value = strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == 0 {
			return -1
		}
		return r
	}, value)
	if len(value) > limit {
		value = strings.ToValidUTF8(value[:limit], "")
	}
	return value
}

func encodePowerShell(script string) string {
	units := utf16.Encode([]rune(script))
	encoded := make([]byte, len(units)*2)
	for index, unit := range units {
		binary.LittleEndian.PutUint16(encoded[index*2:], unit)
	}
	return base64.StdEncoding.EncodeToString(encoded)
}

type limitedBuffer struct {
	buffer strings.Builder
	limit int
}

func (buffer *limitedBuffer) Write(value []byte) (int, error) {
	if buffer.limit > 0 && buffer.buffer.Len()+len(value) > buffer.limit {
		return 0, errors.New("PowerShell output exceeded its safety limit")
	}
	return buffer.buffer.Write(value)
}

func (buffer *limitedBuffer) Bytes() []byte { return []byte(buffer.buffer.String()) }
func (buffer *limitedBuffer) String() string { return buffer.buffer.String() }

const windowsEventLogScript = `
$ErrorActionPreference = 'Stop'
$cursorJson = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('__CURSOR_BASE64__'))
$state = @{}
if ($cursorJson.Length -gt 0) {
    $saved = ConvertFrom-Json -InputObject $cursorJson
    foreach ($property in $saved.PSObject.Properties) {
        $state[$property.Name] = [long]$property.Value
    }
}
$records = New-Object 'System.Collections.Generic.List[object]'
foreach ($logName in @('Application', 'System')) {
    $items = @()
    try {
        if ($state.ContainsKey($logName)) {
            $after = [long]$state[$logName]
            $filter = "*[System[EventRecordID > $after]]"
            $items = @(Get-WinEvent -LogName $logName -FilterXPath $filter -MaxEvents 32 -Oldest -ErrorAction Stop)
        }
        else {
            $items = @(Get-WinEvent -LogName $logName -MaxEvents 32 -ErrorAction Stop | Sort-Object RecordId)
        }
    }
    catch {
        if ($_.FullyQualifiedErrorId -like '*NoMatchingEventsFound*') { continue }
        throw
    }
    foreach ($item in $items) {
        if ($null -eq $item -or $null -eq $item.TimeCreated) { continue }
        $message = [string]$item.Message
        if ($message.Length -gt 16384) { $message = $message.Substring(0, 16384) }
        $eventTime = [long]([DateTimeOffset]$item.TimeCreated).ToUnixTimeMilliseconds() * 1000000L
        $level = 0
        if ($null -ne $item.Level) { $level = [int]$item.Level }
        $records.Add([pscustomobject]@{
            log_name = $logName
            record_id = [long]$item.RecordId
            event_id = [uint32]$item.Id
            event_time_unix_nanos = $eventTime
            provider = [string]$item.ProviderName
            level = $level
            message = $message
            computer = [string]$item.MachineName
        })
    }
}
$ordered = @($records | Sort-Object event_time_unix_nanos, log_name, record_id | Select-Object -First __MAX_EVENTS__)
foreach ($record in $ordered) { $state[$record.log_name] = [long]$record.record_id }
$result = [pscustomobject]@{ cursor = (ConvertTo-Json -InputObject $state -Compress); events = $ordered }
ConvertTo-Json -InputObject $result -Compress -Depth 4
`
