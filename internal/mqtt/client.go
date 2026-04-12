package mqtt

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/liskl/coolledux-controller/internal/config"
)

// Client manages the MQTT connection, publishes HA discovery payloads, and
// routes inbound command messages to the CommandHandler.
type Client struct {
	mqttClient pahomqtt.Client
	cfg        *config.MQTTConfig
	deviceID   string
	handler    *CommandHandler
	logger     *slog.Logger
	connected  bool
	mu         sync.Mutex
}

// NewClient creates an MQTT Client but does not connect yet.
func NewClient(cfg *config.MQTTConfig, deviceID string, handler *CommandHandler, logger *slog.Logger) *Client {
	return &Client{
		cfg:      cfg,
		deviceID: deviceID,
		handler:  handler,
		logger:   logger,
	}
}

// Connect establishes the MQTT connection with LWT, auto-reconnect, and
// discovery publishing on (re)connect.
func (c *Client) Connect(ctx context.Context) error {
	opts := pahomqtt.NewClientOptions().
		AddBroker(c.cfg.Broker).
		SetClientID(c.cfg.ClientID).
		SetCleanSession(true).
		SetAutoReconnect(true).
		SetKeepAlive(c.cfg.Keepalive).
		SetWill(c.availabilityTopic(), "offline", 1, true).
		SetOnConnectHandler(func(_ pahomqtt.Client) {
			c.logger.Info("mqtt connected", "broker", c.cfg.Broker)
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

// Disconnect publishes offline status and cleanly disconnects from the broker.
func (c *Client) Disconnect() {
	if c.mqttClient == nil {
		return
	}

	c.publish(c.availabilityTopic(), "offline", true)
	c.mqttClient.Disconnect(250)

	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()

	c.logger.Info("mqtt disconnected")
}

// PublishState publishes the current light state JSON to the state topic with retain.
func (c *Client) PublishState(state []byte) error {
	topic := fmt.Sprintf("%s/%s/state", c.cfg.TopicPrefix, c.deviceID)
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

// publishDiscovery sends all HA auto-discovery config payloads and publishes
// "online" to the availability topic.
func (c *Client) publishDiscovery() {
	prefix := c.cfg.TopicPrefix
	haPrefix := c.cfg.HADiscoveryPrefix

	builders := []func(string, string, string) (string, []byte){
		BuildLightConfig,
		BuildConnectionSensorConfig,
		BuildBrightnessSensorConfig,
	}

	for _, build := range builders {
		topic, payload := build(c.deviceID, prefix, haPrefix)
		c.publish(topic, payload, true)
	}

	c.publish(c.availabilityTopic(), "online", true)
	c.logger.Info("ha discovery published", "device_id", c.deviceID)
}

// subscribeTopics subscribes to all command topics and wires them to handlers.
func (c *Client) subscribeTopics() {
	prefix := c.cfg.TopicPrefix

	type sub struct {
		suffix  string
		handler func([]byte) error
		publish bool // publish state after handling
	}

	subs := []sub{
		{suffix: "set", handler: c.handler.HandleLightCommand, publish: true},
		{suffix: "text/set", handler: c.handler.HandleTextCommand},
		{suffix: "image/set", handler: c.handler.HandleImageCommand},
		{suffix: "gif/set", handler: c.handler.HandleGIFCommand},
	}

	for _, s := range subs {
		topic := fmt.Sprintf("%s/%s/%s", prefix, c.deviceID, s.suffix)
		publishState := s.publish
		handle := s.handler

		token := c.mqttClient.Subscribe(topic, 1, func(_ pahomqtt.Client, msg pahomqtt.Message) {
			c.logger.Info("mqtt message received", "topic", msg.Topic())
			if err := handle(msg.Payload()); err != nil {
				c.logger.Error("command handler failed", "topic", msg.Topic(), "error", err)
				return
			}
			if publishState {
				if err := c.PublishState(c.handler.GetCurrentState()); err != nil {
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

// availabilityTopic returns the LWT / availability topic for this device.
func (c *Client) availabilityTopic() string {
	return fmt.Sprintf("%s/%s/availability", c.cfg.TopicPrefix, c.deviceID)
}

// publish is a small helper that publishes a message and logs errors.
func (c *Client) publish(topic string, payload interface{}, retain bool) {
	token := c.mqttClient.Publish(topic, 1, retain, payload)
	if !token.WaitTimeout(5 * time.Second) {
		c.logger.Error("publish timed out", "topic", topic)
		return
	}
	if err := token.Error(); err != nil {
		c.logger.Error("publish failed", "topic", topic, "error", err)
	}
}
