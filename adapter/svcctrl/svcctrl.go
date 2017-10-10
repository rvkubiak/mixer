// Copyright 2017 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package svcctrl // import "istio.io/mixer/adapter/svcctrl"

import (
	"bytes"
	"context"
	"time"

	"github.com/pborman/uuid"
	sc "google.golang.org/api/servicecontrol/v1"

	"istio.io/mixer/adapter/svcctrl/config"
	"istio.io/mixer/pkg/adapter"
	pkgHndlr "istio.io/mixer/pkg/handler"
	"istio.io/mixer/template/metric"
)

type handler struct {
	serviceControlClient *sc.Service
	env                  adapter.Env
	configParams         *config.Params
}

func (h *handler) HandleMetric(ctx context.Context, instances []*metric.Instance) error {
	buf := bytes.NewBufferString("mixer-metric-report-id-")
	_, err := buf.WriteString(uuid.New())
	if err != nil {
		return err
	}

	opID := buf.String()
	reportReq, err := handleMetric(time.Now().Format(time.RFC3339Nano), opID)
	if err != nil {
		return err
	}
	_, err = h.serviceControlClient.Services.Report(h.configParams.ServiceName, reportReq).Do()
	return err
}

func handleMetric(timeNow, opID string) (*sc.ReportRequest, error) {
	op := &sc.Operation{
		OperationId:   opID,
		OperationName: "reportMetrics",
		StartTime:     timeNow,
		EndTime:       timeNow,
		Labels: map[string]string{
			"cloud.googleapis.com/location": "global",
		},
	}

	value := int64(1)
	metricValue := sc.MetricValue{
		StartTime:  timeNow,
		EndTime:    timeNow,
		Int64Value: &value,
	}

	op.MetricValueSets = []*sc.MetricValueSet{
		{
			MetricName:   "serviceruntime.googleapis.com/api/producer/request_count",
			MetricValues: []*sc.MetricValue{&metricValue},
		},
	}

	reportReq := &sc.ReportRequest{
		Operations: []*sc.Operation{op},
	}
	return reportReq, nil
}

func (h *handler) Close() error {
	h.serviceControlClient = nil
	return nil
}

////////////////// Config //////////////////////////

// GetInfo registers Adapter with Mixer.
func GetInfo() pkgHndlr.Info {
	return pkgHndlr.Info{
		Name:        "svcctrl",
		Impl:        "istio.io/mixer/adapter/svcctrl",
		Description: "Interface to Google Service Control",
		SupportedTemplates: []string{
			metric.TemplateName,
		},
		DefaultConfig: &config.Params{
			ServiceName: "library-example.sandbox.googleapis.com",
		},

		NewBuilder: func() adapter.Builder2 { return &builder{} },

		// TO BE DELETED
		CreateHandlerBuilder: func() adapter.HandlerBuilder { return &obuilder{&builder{}} },
		ValidateConfig:       func(cfg adapter.Config) *adapter.ConfigErrors { return nil },
	}
}

type builder struct {
	adapterConfig *config.Params
}

func (*builder) SetMetricTypes(map[string]*metric.Type) {}
func (b *builder) SetAdapterConfig(cfg adapter.Config)  { b.adapterConfig = cfg.(*config.Params) }
func (*builder) Validate() (ce *adapter.ConfigErrors)   { return }

func (b *builder) Build(context context.Context, env adapter.Env) (adapter.Handler, error) {
	client, err := createClient(env.Logger())
	if err != nil {
		return nil, err
	}

	return &handler{
		serviceControlClient: client,
		env:                  env,
		configParams:         b.adapterConfig,
	}, nil
}

// EVERYTHING BELOW IS TO BE DELETED

type obuilder struct {
	b *builder
}

func (o *obuilder) Build(cfg adapter.Config, env adapter.Env) (adapter.Handler, error) {
	o.b.SetAdapterConfig(cfg)
	return o.b.Build(context.Background(), env)
}

// ConfigureMetricHandler is to be deleted
func (*obuilder) ConfigureMetricHandler(map[string]*metric.Type) error {
	return nil
}
