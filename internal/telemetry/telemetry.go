package telemetry

import (
	"fmt"

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
)

type Level int

const (
	Verbose Level = iota
	Info
	Warn
	Error
	Critical
)

type Client interface {
	Trace(level Level, msg string)
	Close()
}

var client = NewNullClient()

func SetClient(c Client) {
	client = c
}

func Trace(level Level, msg string) {
	client.Trace(level, msg)
}

type NullClient struct{}

func NewNullClient() Client {
	return NullClient{}
}

func (NullClient) Trace(Level, string) {}
func (NullClient) Close()              {}

type AppInsightClient struct {
	appinsights.TelemetryClient
	uuid string
}

func NewApplication(uuid string) Client {
	const instrumentKey = "8c362287-ebec-4724-8505-1b91ba73b825"
	return AppInsightClient{
		TelemetryClient: appinsights.NewTelemetryClient(instrumentKey),
		uuid:            uuid,
	}
}

func (c AppInsightClient) Trace(level Level, msg string) {
	msg = fmt.Sprintf("[%s] %s", c.uuid, msg)
	c.TrackTrace(msg, contracts.SeverityLevel(level))
}

func (c AppInsightClient) Close() {
	<-c.Channel().Close()
}
