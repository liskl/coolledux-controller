package mqtt

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"sync"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/liskl/coolledux-controller/internal/config"
	"github.com/liskl/coolledux-controller/internal/registry"
)

// brokerUserinfoRE strips userinfo (`user:pass@`) from broker URL
// strings that don't parse cleanly as standard URLs. Paho accepts a
// few non-RFC-3986 forms, so this is a fallback when url.Parse fails.
var brokerUserinfoRE = regexp.MustCompile(`(^[a-zA-Z][a-zA-Z0-9+.-]*://)[^/@]*@`)

// sanitizeBroker returns a credential-free representation of the
// broker URL plus its scheme/host/port broken out for OTel semconv
// attributes. Userinfo is dropped; everything else is preserved.
//
// MQTT broker URLs reach us as user-configurable strings from
// config.MQTTConfig.Broker. Paho accepts forms like
// "tcp://user:pass@host:1883", and previously we attached that string
// verbatim to span attributes and slog records — exfiltrating
// credentials to Tempo/Loki and any other OTel destination.
//
// Returns ("", "", "") for everything when the input is empty.
func sanitizeBroker(raw string) (sanitizedURL, host, port string) {
	if raw == "" {
		return "", "", ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return brokerUserinfoRE.ReplaceAllString(raw, "$1"), "", ""
	}
	u.User = nil
	return u.String(), u.Hostname(), u.Port()
}

// Telemetry globals. No-op when OTel is disabled.
var (
	mqttTracer trace.Tracer = otel.Tracer("github.com/liskl/coolledux-controller/internal/mqtt")
	mqttMeter  metric.Meter = otel.Meter("github.com/liskl/coolledux-controller/internal/mqtt")

	mqttMessagesOut metric.Int64Counter
)

func init() {
	var err error
	mqttMessagesOut, err = mqttMeter.Int64Counter(
		"coolledux.mqtt.messages.out",
		metric.WithDescription("MQTT messages published (discovery, state, availability)"),
	)
	if err != nil {
		slog.Default().Warn("mqtt: out counter failed", "error", err)
	}
}

// Client manages the MQTT connection, publishes HA discovery payloads, and
// routes inbound command messages to per-device CommandHandlers.
type Client struct {
	mqttClient pahomqtt.Client
	cfg        *config.MQTTConfig
	reg        *registry.Registry
	handlers   map[string]*CommandHandler
	logger     *slog.Logger
	connected  bool
	mu         sync.Mutex
}

// NewClient creates an MQTT Client but does not connect yet. A separate
// CommandHandler is built for each device in the registry so per-device
// state (brightness, effect, etc.) is tracked independently.
func NewClient(cfg *config.MQTTConfig, reg *registry.Registry, logger *slog.Logger) *Client {
	handlers := make(map[string]*CommandHandler, reg.Len())
	for _, entry := range reg.List() {
		handlers[entry.ID] = NewCommandHandler(entry.Controller, logger.With("device", entry.ID))
	}
	return &Client{
		cfg:      cfg,
		reg:      reg,
		handlers: handlers,
		logger:   logger,
	}
}

// Connect establishes the MQTT connection with LWT, auto-reconnect, and
// discovery publishing on (re)connect.
func (c *Client) Connect(ctx context.Context) (err error) {
	sanitized, host, port := sanitizeBroker(c.cfg.Broker)
	spanAttrs := []attribute.KeyValue{
		attribute.String("messaging.system", "mqtt"),
		attribute.String("server.address", host),
		attribute.String("mqtt.broker", sanitized),
	}
	if port != "" {
		spanAttrs = append(spanAttrs, attribute.String("server.port", port))
	}
	ctx, span := mqttTracer.Start(ctx, "mqtt.connect", trace.WithAttributes(spanAttrs...))
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	opts := pahomqtt.NewClientOptions().
		AddBroker(c.cfg.Broker).
		SetClientID(c.cfg.ClientID).
		SetCleanSession(true).
		SetAutoReconnect(true).
		SetKeepAlive(c.cfg.Keepalive).
		// Service-wide LWT covers the "service crashed" case. Per-device
		// availability topics are updated manually on BLE connect/disconnect;
		// the paho client only supports one LWT per connection.
		SetWill(c.serviceAvailabilityTopic(), "offline", 1, true).
		SetOnConnectHandler(func(_ pahomqtt.Client) {
			c.logger.Info("mqtt connected", "broker", sanitized)
			c.mu.Lock()
			c.connected = true
			c.mu.Unlock()
			c.publishDiscovery()
			c.subscribeTopics()
		}).
		SetConnectionLostHandler(func(_ pahomqtt.Client, err error) {
			c.logger.Error("mqtt connection lost", "error", err)
			c.mu.Lock()
			c.connected = false
			c.mu.Unlock()
		})

	if c.cfg.Username != "" {
		opts.SetUsername(c.cfg.Username)
	}
	if c.cfg.Password != "" {
		opts.SetPassword(c.cfg.Password)
	}

	c.mqttClient = pahomqtt.NewClient(opts)

	token := c.mqttClient.Connect()
	select {
	case <-token.Done():
	case <-ctx.Done():
		return ctx.Err()
	}

	if err := token.Error(); err != nil {
		return fmt.Errorf("mqtt connect: %w", err)
	}

	return nil
}

// Disconnect publishes offline status on every per-device availability
// topic and on the service-wide topic, then cleanly disconnects.
func (c *Client) Disconnect() {
	if c.mqttClient == nil {
		return
	}

	for _, entry := range c.reg.List() {
		c.publish(c.deviceAvailabilityTopic(entry.ID), "offline", true)
	}
	c.publish(c.serviceAvailabilityTopic(), "offline", true)
	c.mqttClient.Disconnect(250)

	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()

	c.logger.Info("mqtt disconnected")
}

// PublishState publishes the light state JSON for a specific device.
func (c *Client) PublishState(deviceID string, state []byte) error {
	topic := fmt.Sprintf("%s/%s/state", c.cfg.TopicPrefix, deviceID)
	token := c.mqttClient.Publish(topic, 1, true, state)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("publish state timed out")
	}
	return token.Error()
}

// IsConnected returns the current MQTT connection status.
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// publishDiscovery sends the HA auto-discovery config payloads for every
// registered device and flips availability topics to "online".
func (c *Client) publishDiscovery() {
	prefix := c.cfg.TopicPrefix
	haPrefix := c.cfg.HADiscoveryPrefix

	builders := []func(string, string, string) (string, []byte){
		BuildLightConfig,
		BuildConnectionSensorConfig,
		BuildBrightnessSensorConfig,
		BuildColorModeSelectConfig,
		BuildColorSpeedNumberConfig,
		BuildShowDeviceIDSwitchConfig,
		BuildRemoteEnableSwitchConfig,
	}

	for _, entry := range c.reg.List() {
		for _, build := range builders {
			topic, payload := build(entry.ID, prefix, haPrefix)
			c.publish(topic, payload, true)
		}
		c.publish(c.deviceAvailabilityTopic(entry.ID), "online", true)
	}
	c.publish(c.serviceAvailabilityTopic(), "online", true)

	c.logger.Info("ha discovery published", "devices", c.reg.IDs())
}

// subscribeTopics subscribes to each device's command topics and wires
// them to that device's CommandHandler.
func (c *Client) subscribeTopics() {
	prefix := c.cfg.TopicPrefix

	for _, entry := range c.reg.List() {
		handler, ok := c.handlers[entry.ID]
		if !ok {
			c.logger.Warn("no handler for device during subscribe", "id", entry.ID)
			continue
		}
		c.subscribeDeviceTopics(prefix, entry.ID, handler)
	}
}

// subscribeDeviceTopics wires the eight MQTT command topics for one device.
func (c *Client) subscribeDeviceTopics(prefix, deviceID string, handler *CommandHandler) {
	type sub struct {
		suffix  string
		handler func([]byte) error
		publish bool // republish light state after handling
	}
	subs := []sub{
		{suffix: "set", handler: handler.HandleLightCommand, publish: true},
		{suffix: "text/set", handler: handler.HandleTextCommand},
		{suffix: "image/set", handler: handler.HandleImageCommand},
		{suffix: "gif/set", handler: handler.HandleGIFCommand},
		{suffix: "color/mode/set", handler: handler.HandleColorModeCommand},
		{suffix: "color/speed/set", handler: handler.HandleColorSpeedCommand},
		{suffix: "show_id/set", handler: handler.HandleShowDeviceIDCommand},
		{suffix: "remote/set", handler: handler.HandleRemoteCommand},
	}

	for _, s := range subs {
		topic := fmt.Sprintf("%s/%s/%s", prefix, deviceID, s.suffix)
		publishState := s.publish
		handle := s.handler
		id := deviceID
		h := handler

		token := c.mqttClient.Subscribe(topic, 1, func(_ pahomqtt.Client, msg pahomqtt.Message) {
			c.logger.Info("mqtt message received", "topic", msg.Topic())
			if err := handle(msg.Payload()); err != nil {
				c.logger.Error("command handler failed", "topic", msg.Topic(), "error", err)
				return
			}
			if publishState {
				if err := c.PublishState(id, h.GetCurrentState()); err != nil {
					c.logger.Error("failed to publish state", "error", err)
				}
			}
		})

		if !token.WaitTimeout(5 * time.Second) {
			c.logger.Error("subscribe timed out", "topic", topic)
			continue
		}
		if err := token.Error(); err != nil {
			c.logger.Error("subscribe failed", "topic", topic, "error", err)
			continue
		}

		c.logger.Debug("subscribed", "topic", topic)
	}
}

// serviceAvailabilityTopic is the single LWT target; flips to "offline"
// automatically if the broker loses us without a clean disconnect.
func (c *Client) serviceAvailabilityTopic() string {
	return fmt.Sprintf("%s/availability", c.cfg.TopicPrefix)
}

// deviceAvailabilityTopic is the per-device availability topic HA entities
// watch. Published manually on connect/disconnect; not covered by LWT.
func (c *Client) deviceAvailabilityTopic(deviceID string) string {
	return fmt.Sprintf("%s/%s/availability", c.cfg.TopicPrefix, deviceID)
}

// publish is a small helper that publishes a message and logs errors.
func (c *Client) publish(topic string, payload any, retain bool) {
	token := c.mqttClient.Publish(topic, 1, retain, payload)
	if !token.WaitTimeout(5 * time.Second) {
		c.logger.Error("publish timed out", "topic", topic)
		return
	}
	if err := token.Error(); err != nil {
		c.logger.Error("publish failed", "topic", topic, "error", err)
		return
	}
	if mqttMessagesOut != nil {
		mqttMessagesOut.Add(context.Background(), 1,
			metric.WithAttributes(attribute.String("messaging.destination.name", topic)))
	}
}
