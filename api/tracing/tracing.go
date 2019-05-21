package tracing

import (
	"os"

	"github.com/okteto/app/api/log"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go/config"
	jlog "github.com/uber/jaeger-client-go/log"
	"github.com/uber/jaeger-lib/metrics"
)

const envAgentHost = "JAEGER_AGENT_HOST"

func Get() (opentracing.Tracer, error) {
	if len(os.Getenv(envAgentHost)) == 0 {
		return opentracing.NoopTracer{}, nil
	}

	cfg, err := config.FromEnv()
	if err != nil {
		return nil, err
	}

	cfg.ServiceName = "api"
	cfg.Sampler.Type = "const"
	cfg.Sampler.Param = 1

	jLogger := jlog.StdLogger
	jMetricsFactory := metrics.NullFactory

	tracer, _, err := cfg.NewTracer(
		config.Logger(jLogger),
		config.Metrics(jMetricsFactory),
	)

	if err != nil {
		return nil, err
	}

	log.Infof("tracing to %s", os.Getenv(envAgentHost))
	return tracer, nil
}
