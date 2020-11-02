package cortexexporter

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	prw "go.opentelemetry.io/collector/exporter/prometheusremotewriteexporter"
)

// TODO: issues to file upstream:
// 		- fix translation from OC to internal so that temporality is set
//      - add logging support in the Prometheus remote write exporter upstream
//      - add a README to the exporterhelper package
//      - support some sort of plugin mechanism for authentication for different components

const (
	typeStr = "cortex" // The value of "type" key in configuration.
)

// NewFactory returns a factory of the Cortex exporter that can be registered to the Collector.
func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithMetrics(createMetricsExporter))
}

func createMetricsExporter(_ context.Context, params component.ExporterCreateParams,
	cfg configmodels.Exporter) (component.MetricsExporter, error) {
	// check if the configuration is valid
	prwCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("invalid configuration")
	}
	client, cerr := prwCfg.HTTPClientSettings.ToClient()
	if cerr != nil {
		return nil, cerr
	}

	// load AWS auth configurations and create interceptor based on configuration
	if prwCfg.AuthSettings.Enabled {
		roundTripper, err := NewAuth(prwCfg.AuthSettings, client)
		if err != nil {
			return nil, err
		}
		client.Transport = roundTripper
	}

	// initialize an upstream exporter and pass it an http.Client with interceptor
	prwe, err := prw.NewPrwExporter(prwCfg.Namespace, prwCfg.HTTPClientSettings.Endpoint, client)
	if err != nil {
		return nil, err
	}

	// use upstream helper package to return an exporter that implements the required interface, and has timeout,
	// queueing and retry feature enabled
	prwexp, err := exporterhelper.NewMetricsExporter(
		cfg,
		params.Logger,
		prwe.PushMetrics,
		exporterhelper.WithTimeout(prwCfg.TimeoutSettings),
		exporterhelper.WithQueue(prwCfg.QueueSettings),
		exporterhelper.WithRetry(prwCfg.RetrySettings),
		exporterhelper.WithShutdown(prwe.Shutdown),
	)

	return prwexp, err
}

func createDefaultConfig() configmodels.Exporter {
	qs := exporterhelper.CreateDefaultQueueSettings()
	qs.Enabled = false

	// TODO: re-enable retry by default after componenterror.combineErrors upstream can return a permanent error if it is combined from permanent errors
	ts := exporterhelper.CreateDefaultRetrySettings()
	ts.Enabled = false

	return &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
		Namespace:       "",
		TimeoutSettings: exporterhelper.CreateDefaultTimeoutSettings(),
		RetrySettings:   ts,
		QueueSettings:   qs,
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "http://some.url:9411/api/prom/push",
			// We almost read 0 bytes, so no need to tune ReadBufferSize.
			ReadBufferSize:  0,
			WriteBufferSize: 512 * 1024,
			Timeout:         exporterhelper.CreateDefaultTimeoutSettings().Timeout,
			Headers:         map[string]string{},
		},
	}
}
