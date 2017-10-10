// Copyright 2017 Istio Authors.
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

package denier // import "istio.io/mixer/adapter/denier"

// NOTE: This adapter will eventually be auto-generated so that it automatically supports all CHECK and QUOTA
//       templates known to Mixer. For now, it's manually curated.

import (
	"context"
	"time"

	rpc "github.com/googleapis/googleapis/google/rpc"

	"istio.io/mixer/adapter/denier/config"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/template/checknothing"
	"istio.io/mixer/template/listentry"
	"istio.io/mixer/template/quota"
)

type handler struct {
	status rpc.Status
}

////////////////// Runtime Methods //////////////////////////

func (h *handler) HandleCheckNothing(context.Context, *checknothing.Instance) (adapter.CheckResult, error) {
	return adapter.CheckResult{
		Status:        h.status,
		ValidDuration: 1000 * time.Second,
		ValidUseCount: 1000,
	}, nil
}

func (h *handler) HandleListEntry(context.Context, *listentry.Instance) (adapter.CheckResult, error) {
	return adapter.CheckResult{
		Status:        h.status,
		ValidDuration: 1000 * time.Second,
		ValidUseCount: 1000,
	}, nil
}

func (*handler) HandleQuota(context.Context, *quota.Instance, adapter.QuotaArgs) (adapter.QuotaResult, error) {
	return adapter.QuotaResult{}, nil
}

func (*handler) Close() error { return nil }

////////////////// Bootstrap //////////////////////////

// GetInfo returns the Info associated with this adapter implementation.
func GetInfo() adapter.Info {
	return adapter.Info{
		Name:        "denier",
		Impl:        "istio.io/mixer/adapter/denier",
		Description: "Rejects any check and quota request with a configurable error",
		SupportedTemplates: []string{
			checknothing.TemplateName,
			listentry.TemplateName,
			quota.TemplateName,
		},
		DefaultConfig: &config.Params{
			Status: rpc.Status{Code: int32(rpc.FAILED_PRECONDITION)},
		},

		NewBuilder: func() adapter.HandlerBuilder { return &builder{} },
	}
}

type builder struct {
	adapterConfig *config.Params
}

func (*builder) SetCheckNothingTypes(map[string]*checknothing.Type) {}
func (*builder) SetListEntryTypes(map[string]*listentry.Type)       {}
func (*builder) SetQuotaTypes(map[string]*quota.Type)               {}
func (b *builder) SetAdapterConfig(cfg adapter.Config)              { b.adapterConfig = cfg.(*config.Params) }
func (*builder) Validate() (ce *adapter.ConfigErrors)               { return }

func (b *builder) Build(context context.Context, env adapter.Env) (adapter.Handler, error) {
	return &handler{status: b.adapterConfig.Status}, nil
}
