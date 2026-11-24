package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"monitor-me.internal/backend/internal/identity"
	"monitor-me.internal/backend/internal/telemetry"
	"github.com/gorilla/websocket"
)

var errSlowLiveClient = errors.New("live client exceeded its bounded output buffer")

var liveUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (api *API) createLiveTicket(writer http.ResponseWriter, request *http.Request) {
	ticket, err := api.identities.CreateLiveTicket(request.Context(), principalFromRequest(request))
	if err != nil {
		if errors.Is(err, identity.ErrUnauthorized) {
			writeAPIError(writer, http.StatusUnauthorized, "unauthorized", "The current user session is no longer valid.")
			return
		}
		writeAPIError(writer, http.StatusInternalServerError, "live_ticket_failed", "A live connection ticket could not be created.")
		return
	}
	writeJSON(writer, http.StatusCreated, map[string]any{
		"ticket":             ticket.Token,
		"expires_at":         ticket.ExpiresAt,
		"expires_in_seconds": 30,
		"single_use":         true,
	})
}

func (api *API) liveOriginAllowed(request *http.Request) bool {
	origin, err := url.Parse(request.Header.Get("Origin"))
	if err != nil || origin.Host == "" || origin.User != nil ||
		origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" ||
		(origin.Scheme != "http" && origin.Scheme != "https") {
		return false
	}
	originValue := origin.Scheme + "://" + strings.ToLower(origin.Host)

	// Reverse proxies must set X-Forwarded-Proto to the scheme they terminate.
	// The API port is private in production, so this header is trusted only
	// from the configured reverse-proxy/network boundary.
	scheme := strings.TrimSpace(strings.Split(request.Header.Get("X-Forwarded-Proto"), ",")[0])
	if scheme == "" {
		scheme = "http"
		if request.TLS != nil {
			scheme = "https"
		}
	}
	if originValue == scheme+"://"+strings.ToLower(request.Host) {
		return true
	}
	_, allowed := api.allowedLiveOrigins[originValue]
	return allowed
}

func (api *API) liveStream(writer http.ResponseWriter, request *http.Request) {
	token := request.URL.Query().Get("ticket")
	principal, err := api.identities.ConsumeLiveTicket(request.Context(), token)
	if err != nil {
		writeAPIError(writer, http.StatusUnauthorized, "invalid_live_ticket", "The live ticket is missing, expired, already used, or no longer authorized.")
		return
	}

	channel := liveOrganizationChannel(principal.OrganizationID)
	ctx, cancel := context.WithCancel(request.Context())
	defer cancel()
	pubsub := api.cache.Subscribe(ctx, channel)
	defer pubsub.Close()
	if _, err := pubsub.Receive(ctx); err != nil {
		writeAPIError(writer, http.StatusServiceUnavailable, "live_unavailable", "The live event service is temporarily unavailable.")
		return
	}

	upgrader := liveUpgrader
	upgrader.CheckOrigin = api.liveOriginAllowed
	connection, err := upgrader.Upgrade(writer, request, nil)
	if err != nil {
		return
	}
	defer connection.Close()
	// net/http's WriteTimeout also covers the upgrade handshake. Clear that
	// inherited deadline after hijacking; this handler applies bounded deadlines
	// explicitly to every subsequent WebSocket write.
	if err := connection.UnderlyingConn().SetWriteDeadline(time.Time{}); err != nil {
		return
	}
	connection.SetReadLimit(1024)
	_ = connection.SetReadDeadline(time.Now().Add(60 * time.Second))
	connection.SetPongHandler(func(string) error {
		return connection.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	// The Redis reader and network writer are separated by a fixed-size queue.
	// A slow browser is disconnected rather than consuming unbounded memory.
	outbound := make(chan []byte, 32)
	receiveErrors := make(chan error, 1)
	go func() {
		for {
			message, receiveErr := pubsub.ReceiveMessage(ctx)
			if receiveErr != nil {
				select {
				case receiveErrors <- receiveErr:
				default:
				}
				cancel()
				return
			}
			payload := []byte(message.Payload)
			if !enqueueLivePayload(outbound, payload) {
				select {
				case receiveErrors <- errSlowLiveClient:
				default:
				}
				cancel()
				return
			}
		}
	}()

	go func() {
		for {
			if _, _, readErr := connection.ReadMessage(); readErr != nil {
				cancel()
				return
			}
		}
	}()

	pingTicker := time.NewTicker(25 * time.Second)
	defer pingTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case receiveErr := <-receiveErrors:
			if errors.Is(receiveErr, errSlowLiveClient) {
				_ = connection.WriteControl(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "live client is too slow; reconnect and fetch history"),
					time.Now().Add(time.Second),
				)
			}
			return
		case payload := <-outbound:
			if err := connection.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
				return
			}
			if err := connection.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		case <-pingTicker.C:
			if err := connection.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
				return
			}
		}
	}
}

func enqueueLivePayload(queue chan<- []byte, payload []byte) bool {
	select {
	case queue <- payload:
		return true
	default:
		return false
	}
}

func liveOrganizationChannel(organizationID string) string {
	return telemetry.LiveChannel(organizationID)
}
