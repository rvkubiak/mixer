// Copyright 2016 Istio Authors
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

package api

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	rpc "github.com/googleapis/googleapis/google/rpc"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
	legacyContext "golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	mixerpb "istio.io/api/mixer/v1"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/adapterManager"
	"istio.io/mixer/pkg/aspect"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/pkg/pool"
	"istio.io/mixer/pkg/runtime"
	"istio.io/mixer/pkg/status"
)

// We have a slightly messy situation around the use of context objects. gRPC stubs are
// generated to expect the old "x/net/context" types instead of the more modern "context".
// We end up doing a quick switcharoo from the gRPC defined type to the modern type so we can
// use the modern type elsewhere in the code.

type (
	// grpcServer holds the dispatchState for the gRPC API server.
	grpcServer struct {
		dispatcher       runtime.Dispatcher
		aspectDispatcher adapterManager.AspectDispatcher
		gp               *pool.GoroutinePool

		// the global dictionary. This will eventually be writable via config
		globalWordList []string
		globalDict     map[string]int32
	}
)

const (
	// defaultValidDuration is the default duration for which a check or quota result is valid.
	defaultValidDuration = 10 * time.Second
	// defaultValidUseCount is the default number of calls for which a check or quota result is valid.
	defaultValidUseCount = 200
)

var checkOk = &adapter.CheckResult{
	ValidDuration: defaultValidDuration,
	ValidUseCount: defaultValidUseCount,
}

// NewGRPCServer creates a gRPC serving stack.
func NewGRPCServer(aspectDispatcher adapterManager.AspectDispatcher, dispatcher runtime.Dispatcher, gp *pool.GoroutinePool) mixerpb.MixerServer {
	list := attribute.GlobalList()
	globalDict := make(map[string]int32, len(list))
	for i := 0; i < len(list); i++ {
		globalDict[list[i]] = int32(i)
	}

	return &grpcServer{
		dispatcher:       dispatcher,
		aspectDispatcher: aspectDispatcher,
		gp:               gp,
		globalWordList:   list,
		globalDict:       globalDict,
	}
}

// Check is the entry point for the external Check method
func (s *grpcServer) Check(legacyCtx legacyContext.Context, req *mixerpb.CheckRequest) (*mixerpb.CheckResponse, error) {
	// TODO: this code doesn't distinguish between RPC failures when communicating with adapters and
	//       their backend vs. semantic failures. For example, if the adapterDispatcher.Check
	//       method returns a bad status, is that because an adapter failed an RPC or because the
	//       request was denied? This will need to be addressed in the new adapter model. In the meantime,
	//       RPC failure is treated as a semantic denial.

	requestBag := attribute.NewProtoBag(&req.Attributes, s.globalDict, s.globalWordList)

	globalWordCount := int(req.GlobalWordCount)

	glog.V(1).Info("Dispatching Preprocess Check")
	preprocResponseBag := attribute.GetMutableBag(requestBag)
	out := s.aspectDispatcher.Preprocess(legacyCtx, requestBag, preprocResponseBag)

	if !status.IsOK(out) {
		glog.Error("Preprocess Check returned with: ", status.String(out))
		requestBag.Done()
		preprocResponseBag.Done()
		return nil, makeGRPCError(out)
	}
	glog.V(1).Info("Preprocess Check returned with: ", status.String(out))

	if glog.V(2) {
		glog.Info("Dispatching to main adapters after running processors")
		for _, name := range preprocResponseBag.Names() {
			v, _ := preprocResponseBag.Get(name)
			glog.Infof("  %s: %v", name, v)
		}
	}

	resp := &mixerpb.CheckResponse{}
	// TODO: these values need to initially come from config, and be modulated by the kind of attribute
	//       that was used in the check and the in-used aspects (for example, maybe an auth check has a
	//       30s TTL but a whitelist check has got a 120s TTL)
	resp.Precondition.ValidDuration = 5 * time.Second
	resp.Precondition.ValidUseCount = 10000

	responseBag := attribute.GetMutableBag(nil)
	out2 := status.OK

	if s.dispatcher != nil {
		// dispatch check2 and set success messages.
		glog.V(1).Info("Dispatching Check2")
		cr, err := s.dispatcher.Check(legacyCtx, preprocResponseBag)
		if err != nil {
			out2 = status.WithError(err)
		}

		if cr == nil {
			// There were no checks performed for this request.
			// return ok.
			cr = checkOk
		} else {
			out2 = cr.Status
		}

		resp.Precondition.ValidDuration = cr.ValidDuration
		resp.Precondition.ValidUseCount = cr.ValidUseCount

		if status.IsOK(out2) {
			glog.V(2).Info("Check2 returned with ok")
		} else {
			glog.Error("Check2 returned with error : ", status.String(out2))
		}
	}

	glog.V(1).Info("Dispatching Check")
	out = s.aspectDispatcher.Check(legacyCtx, preprocResponseBag, responseBag)
	if status.IsOK(out) {
		glog.V(1).Info("Check returned with ok : ", status.String(out))
	} else {
		glog.Error("Check returned with error : ", status.String(out))
	}

	// if out2 fails, we want to see that error
	// otherwise use out.
	if !status.IsOK(out2) {
		out = out2
	}

	resp.Precondition.Status = out
	responseBag.ToProto(&resp.Precondition.Attributes, s.globalDict, globalWordCount)
	responseBag.Done()
	resp.Precondition.ReferencedAttributes = requestBag.GetReferencedAttributes(s.globalDict, globalWordCount)
	requestBag.ClearReferencedAttributes()

	if status.IsOK(resp.Precondition.Status) && len(req.Quotas) > 0 {
		resp.Quotas = make(map[string]mixerpb.CheckResponse_QuotaResult, len(req.Quotas))
		var qr *mixerpb.CheckResponse_QuotaResult

		// TODO: should dispatch this loop in parallel
		// WARNING: if this is dispatched in parallel, then we need to do
		//          use a different protoBag for each individual goroutine
		//          such that we can get valid usage info for individual attributes.
		for name, param := range req.Quotas {
			qma := &aspect.QuotaMethodArgs{
				Quota:           name,
				Amount:          param.Amount,
				DeduplicationID: req.DeduplicationId + name,
				BestEffort:      param.BestEffort,
			}
			var err error

			qr, err = quota(legacyCtx, s.dispatcher, preprocResponseBag, qma)
			// if quota check fails, set status for the entire request and stop processing.
			if err != nil {
				resp.Precondition.Status = status.WithError(err)
				requestBag.ClearReferencedAttributes()
				break
			}

			if qr == nil {
				//TODO remove
				qr = quotaOld(legacyCtx, s.aspectDispatcher, preprocResponseBag, qma)
			}

			// If qma.Quota does not apply to this request give the client what it asked for.
			// Effectively the quota is unlimited.
			if qr == nil {
				qr = &mixerpb.CheckResponse_QuotaResult{
					ValidDuration: defaultValidDuration,
					GrantedAmount: qma.Amount,
				}
			}
			qr.ReferencedAttributes = requestBag.GetReferencedAttributes(s.globalDict, globalWordCount)
			resp.Quotas[name] = *qr
			requestBag.ClearReferencedAttributes()
		}
	}

	requestBag.Done()
	preprocResponseBag.Done()

	return resp, nil
}

// quotaOld is to be removed.
func quotaOld(legacyCtx legacyContext.Context, d adapterManager.AspectDispatcher, bag attribute.Bag,
	qma *aspect.QuotaMethodArgs) *mixerpb.CheckResponse_QuotaResult {
	glog.V(1).Info("Dispatching Quota")
	qmr, out := d.Quota(legacyCtx, bag, qma)
	if status.IsOK(out) {
		glog.V(1).Infof("Quota returned with ok '%s' and quota response '%v'", status.String(out), qmr)
	} else {
		glog.Warningf("Quota returned with error '%s' and quota response '%v'", status.String(out), qmr)

		if out.Code == int32(rpc.RESOURCE_EXHAUSTED) {
			qmr = &aspect.QuotaMethodResp{
				Amount:     0,
				Expiration: defaultValidDuration,
			}
		}
	}
	if qmr == nil {
		return nil
	}
	return &mixerpb.CheckResponse_QuotaResult{
		GrantedAmount: qmr.Amount,
		ValidDuration: qmr.Expiration,
	}
}

func quota(legacyCtx legacyContext.Context, d runtime.Dispatcher, bag attribute.Bag,
	qma *aspect.QuotaMethodArgs) (*mixerpb.CheckResponse_QuotaResult, error) {
	if d == nil {
		return nil, nil
	}
	glog.V(1).Infof("Dispatching Quota2: %s", qma.Quota)
	qmr, err := d.Quota(legacyCtx, bag, qma)
	if err != nil {
		// TODO record the error in the quota specific result
		glog.Warningf("Quota2 %s returned error: %v", qma.Quota, err)
		return nil, err
	}

	if qmr == nil { // no quota applied for the given request
		return nil, nil
	}

	if glog.V(2) {
		glog.Infof("Quota2 %s returned: %v", qma.Quota, qmr)
	}
	return &mixerpb.CheckResponse_QuotaResult{
		GrantedAmount: qmr.Amount,
		ValidDuration: qmr.ValidDuration,
	}, nil
}

var reportResp = &mixerpb.ReportResponse{}

// Report is the entry point for the external Report method
func (s *grpcServer) Report(legacyCtx legacyContext.Context, req *mixerpb.ReportRequest) (*mixerpb.ReportResponse, error) {
	if len(req.Attributes) == 0 {
		// early out
		return reportResp, nil
	}

	// apply the request-level word list to each attribute message if needed
	for i := 0; i < len(req.Attributes); i++ {
		if len(req.Attributes[i].Words) == 0 {
			req.Attributes[i].Words = req.DefaultWords
		}
	}

	protoBag := attribute.NewProtoBag(&req.Attributes[0], s.globalDict, s.globalWordList)
	requestBag := attribute.GetMutableBag(protoBag)
	preprocResponseBag := attribute.GetMutableBag(requestBag)

	var err error
	for i := 0; i < len(req.Attributes); i++ {
		span, newctx := opentracing.StartSpanFromContext(legacyCtx, fmt.Sprintf("Attributes %d", i))

		// the first attribute block is handled by the protoBag as a foundation,
		// deltas are applied to the child bag (i.e. requestBag)
		if i > 0 {
			err = requestBag.UpdateBagFromProto(&req.Attributes[i], s.globalWordList)
			if err != nil {
				msg := "Request could not be processed due to invalid attributes."
				glog.Error(msg, "\n", err)
				details := status.NewBadRequest("attributes", err)
				err = makeGRPCError(status.InvalidWithDetails(msg, details))
				break
			}
		}

		glog.V(1).Info("Dispatching Preprocess")
		out := s.aspectDispatcher.Preprocess(newctx, requestBag, preprocResponseBag)
		if !status.IsOK(out) {
			glog.Error("Preprocess returned with: ", status.String(out))
			err = makeGRPCError(out)
			span.LogFields(log.String("error", err.Error()))
			span.Finish()
			break
		}
		glog.V(1).Info("Preprocess returned with: ", status.String(out))

		if glog.V(2) {
			glog.Info("Dispatching to main adapters after running processors")
			for _, name := range preprocResponseBag.Names() {
				v, _ := preprocResponseBag.Get(name)
				glog.Infof("  %s: %v", name, v)
			}
		}
		out2 := status.OK

		if s.dispatcher != nil {
			// dispatch check2 and set success messages.
			glog.V(1).Infof("Dispatching Report2 %d out of %d", i, len(req.Attributes))
			err := s.dispatcher.Report(legacyCtx, preprocResponseBag)
			if err != nil {
				out2 = status.WithError(err)
				glog.Warningf("Report2 returned %v", err)
			}
		}

		glog.V(1).Infof("Dispatching Report %d out of %d", i, len(req.Attributes))
		out = s.aspectDispatcher.Report(legacyCtx, preprocResponseBag)

		// if out2 fails, we want to see that error
		// otherwise use out.
		if !status.IsOK(out2) {
			out = out2
		}

		if !status.IsOK(out) {
			glog.Errorf("Report %d returned with: %s", i, status.String(out))
			err = makeGRPCError(out)
			span.LogFields(log.String("error", err.Error()))
			span.Finish()
			break
		}
		glog.V(1).Infof("Report %d returned with: %s", i, status.String(out))

		span.LogFields(log.String("success", fmt.Sprintf("finished Report for attribute bag %d", i)))
		span.Finish()
		preprocResponseBag.Reset()
	}

	preprocResponseBag.Done()
	requestBag.Done()
	protoBag.Done()

	if err != nil {
		return nil, err
	}

	return reportResp, nil
}

func makeGRPCError(status rpc.Status) error {
	return grpc.Errorf(codes.Code(status.Code), status.Message)
}
