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

package sample

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/ghodss/yaml"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes/empty"
	rpc "github.com/googleapis/googleapis/google/rpc"
	//rpc "github.com/googleapis/googleapis/google/rpc"

	pb "istio.io/api/mixer/v1/config/descriptor"
	"istio.io/mixer/pkg/adapter"
	adpTmpl "istio.io/mixer/pkg/adapter/template"
	//"istio.io/mixer/pkg/expr"
	"context"

	sample_check "istio.io/mixer/template/sample/check"
	sample_quota "istio.io/mixer/template/sample/quota"
	sample_report "istio.io/mixer/template/sample/report"
	//"istio.io/mixer/pkg/expr"
	"istio.io/mixer/pkg/expr"
)

// Does not implement any template interfaces.
type fakeBadHandler struct{}

func (h fakeBadHandler) Close() error { return nil }
func (h fakeBadHandler) Build(adapter.Config, adapter.Env) (adapter.Handler, error) {
	return nil, nil
}

type fakeReportHandler struct {
	adapter.Handler
	retError      error
	cnfgCallInput interface{}
	procCallInput interface{}
}

func (h *fakeReportHandler) Close() error { return nil }
func (h *fakeReportHandler) HandleSample(ctx context.Context, instances []*sample_report.Instance) error {
	h.procCallInput = instances
	return h.retError
}
func (h *fakeReportHandler) Build(adapter.Config, adapter.Env) (adapter.Handler, error) {
	return nil, nil
}
func (h *fakeReportHandler) ConfigureSampleHandler(t map[string]*sample_report.Type) error {
	h.cnfgCallInput = t
	return nil
}

type fakeCheckHandler struct {
	adapter.Handler
	retError      error
	cnfgCallInput interface{}
	procCallInput interface{}
	retResult     adapter.CheckResult
}

func (h *fakeCheckHandler) Close() error { return nil }
func (h *fakeCheckHandler) HandleSample(ctx context.Context, instance *sample_check.Instance) (adapter.CheckResult, error) {
	h.procCallInput = instance
	return h.retResult, h.retError
}
func (h *fakeCheckHandler) Build(adapter.Config, adapter.Env) (adapter.Handler, error) {
	return nil, nil
}
func (h *fakeCheckHandler) ConfigureSampleHandler(t map[string]*sample_check.Type) error {
	h.cnfgCallInput = t
	return nil
}

type fakeQuotaHandler struct {
	adapter.Handler
	retError      error
	retResult     adapter.QuotaResult2
	cnfgCallInput interface{}
	procCallInput interface{}
}

func (h *fakeQuotaHandler) Close() error { return nil }
func (h *fakeQuotaHandler) HandleQuota(ctx context.Context, instance *sample_quota.Instance, qra adapter.QuotaRequestArgs) (adapter.QuotaResult2, error) {
	h.procCallInput = instance
	return h.retResult, h.retError
}
func (h *fakeQuotaHandler) Build(adapter.Config, adapter.Env) (adapter.Handler, error) {
	return nil, nil
}
func (h *fakeQuotaHandler) ConfigureQuotaHandler(t map[string]*sample_quota.Type) error {
	h.cnfgCallInput = t
	return nil
}

type fakeBag struct{}

func (f fakeBag) Get(name string) (value interface{}, found bool) { return nil, false }
func (f fakeBag) Names() []string                                 { return []string{} }
func (f fakeBag) Done()                                           {}

func TestGeneratedFields(t *testing.T) {
	for _, tst := range []struct {
		tmpl      string
		ctrCfg    proto.Message
		variety   adpTmpl.TemplateVariety
		name      string
		bldrName  string
		hndlrName string
	}{
		{
			tmpl:      sample_report.TemplateName,
			ctrCfg:    &sample_report.InstanceParam{},
			variety:   adpTmpl.TEMPLATE_VARIETY_REPORT,
			bldrName:  "istio.io/mixer/template/sample/report.HandlerBuilder",
			hndlrName: "istio.io/mixer/template/sample/report.Handler",
			name:      sample_report.TemplateName,
		},
		{
			tmpl:      sample_check.TemplateName,
			ctrCfg:    &sample_check.InstanceParam{},
			variety:   adpTmpl.TEMPLATE_VARIETY_CHECK,
			bldrName:  "istio.io/mixer/template/sample/check.HandlerBuilder",
			hndlrName: "istio.io/mixer/template/sample/check.Handler",
			name:      sample_check.TemplateName,
		},
		{
			tmpl:      sample_quota.TemplateName,
			ctrCfg:    &sample_quota.InstanceParam{},
			variety:   adpTmpl.TEMPLATE_VARIETY_QUOTA,
			bldrName:  "istio.io/mixer/template/sample/quota.HandlerBuilder",
			hndlrName: "istio.io/mixer/template/sample/quota.Handler",
			name:      sample_quota.TemplateName,
		},
	} {
		t.Run(tst.tmpl, func(t *testing.T) {
			if !reflect.DeepEqual(SupportedTmplInfo[tst.tmpl].CtrCfg, tst.ctrCfg) {
				t.Errorf("SupportedTmplInfo[%s].CtrCfg = %T, want %T", tst.tmpl, SupportedTmplInfo[tst.tmpl].CtrCfg, tst.ctrCfg)
			}
			if SupportedTmplInfo[tst.tmpl].Variety != tst.variety {
				t.Errorf("SupportedTmplInfo[%s].Variety = %v, want %v", tst.tmpl, SupportedTmplInfo[tst.tmpl].Variety, tst.variety)
			}
			if SupportedTmplInfo[tst.tmpl].BldrInterfaceName != tst.bldrName {
				t.Errorf("SupportedTmplInfo[%s].BldrName = %v, want %v", tst.tmpl, SupportedTmplInfo[tst.tmpl].BldrInterfaceName, tst.bldrName)
			}
			if SupportedTmplInfo[tst.tmpl].HndlrInterfaceName != tst.hndlrName {
				t.Errorf("SupportedTmplInfo[%s].HndlrName = %v, want %v", tst.tmpl, SupportedTmplInfo[tst.tmpl].HndlrInterfaceName, tst.hndlrName)
			}
			if SupportedTmplInfo[tst.tmpl].Name != tst.name {
				t.Errorf("SupportedTmplInfo[%s].Name = %s, want %s", tst.tmpl, SupportedTmplInfo[tst.tmpl].Name, tst.name)
			}
		})
	}
}

func TestHandlerSupportsTemplate(t *testing.T) {
	for _, tst := range []struct {
		tmpl   string
		hndlr  adapter.Handler
		result bool
	}{
		{
			tmpl:   sample_report.TemplateName,
			hndlr:  fakeBadHandler{},
			result: false,
		},
		{
			tmpl:   sample_report.TemplateName,
			hndlr:  &fakeReportHandler{},
			result: true,
		},
		{
			tmpl:   sample_check.TemplateName,
			hndlr:  fakeBadHandler{},
			result: false,
		},
		{
			tmpl:   sample_check.TemplateName,
			hndlr:  &fakeCheckHandler{},
			result: true,
		},
		{
			tmpl:   sample_quota.TemplateName,
			hndlr:  fakeBadHandler{},
			result: false,
		},
		{
			tmpl:   sample_quota.TemplateName,
			hndlr:  &fakeQuotaHandler{},
			result: true,
		},
	} {
		t.Run(tst.tmpl, func(t *testing.T) {
			c := SupportedTmplInfo[tst.tmpl].HandlerSupportsTemplate(tst.hndlr)
			if c != tst.result {
				t.Errorf("SupportedTmplInfo[%s].HandlerSupportsTemplate(%T) = %t, want %t", tst.tmpl, tst.hndlr, c, tst.result)
			}
		})
	}
}

func TestBuilderSupportsTemplate(t *testing.T) {
	for _, tst := range []struct {
		tmpl      string
		hndlrBldr adapter.HandlerBuilder
		result    bool
	}{
		{
			tmpl:      sample_report.TemplateName,
			hndlrBldr: fakeBadHandler{},
			result:    false,
		},
		{
			tmpl:      sample_report.TemplateName,
			hndlrBldr: &fakeReportHandler{},
			result:    true,
		},
		{
			tmpl:      sample_check.TemplateName,
			hndlrBldr: fakeBadHandler{},
			result:    false,
		},
		{
			tmpl:      sample_check.TemplateName,
			hndlrBldr: &fakeCheckHandler{},
			result:    true,
		},
		{
			tmpl:      sample_quota.TemplateName,
			hndlrBldr: fakeBadHandler{},
			result:    false,
		},
		{
			tmpl:      sample_quota.TemplateName,
			hndlrBldr: &fakeQuotaHandler{},
			result:    true,
		},
	} {
		t.Run(tst.tmpl, func(t *testing.T) {
			c := SupportedTmplInfo[tst.tmpl].BuilderSupportsTemplate(tst.hndlrBldr)
			if c != tst.result {
				t.Errorf("SupportedTmplInfo[%s].SupportsTemplate(%T) = %t, want %t", tst.tmpl, tst.hndlrBldr, c, tst.result)
			}
		})
	}
}

type inferTypeTest struct {
	name               string
	ctrCnfg            string
	cstrParam          interface{}
	typeEvalError      error
	wantValueType      pb.ValueType
	wantDimensionsType map[string]pb.ValueType
	wantErr            string
	willPanic          bool
}

func getExprEvalFunc(err error) func(string) (pb.ValueType, error) {
	return func(expr string) (pb.ValueType, error) {
		retType := pb.VALUE_TYPE_UNSPECIFIED
		if strings.HasSuffix(expr, "string") {
			retType = pb.STRING
		}
		if strings.HasSuffix(expr, "double") {
			retType = pb.DOUBLE
		}
		if strings.HasSuffix(expr, "bool") {
			retType = pb.BOOL
		}
		if strings.HasSuffix(expr, "int64") {
			retType = pb.INT64
		}
		return retType, err
	}
}

func TestInferTypeForSampleReport(t *testing.T) {
	for _, tst := range []inferTypeTest{
		{
			name: "SimpleValid",
			ctrCnfg: `
value: source.int64
int64Primitive: source.int64
boolPrimitive: source.bool
doublePrimitive: source.double
stringPrimitive: source.string
dimensions:
  source: source.string
  target: source.string
`,
			cstrParam:          &sample_report.InstanceParam{},
			typeEvalError:      nil,
			wantValueType:      pb.INT64,
			wantDimensionsType: map[string]pb.ValueType{"source": pb.STRING, "target": pb.STRING},
			wantErr:            "",
			willPanic:          false,
		},
		{
			name: "MissingAFieldFromInstanceParam",
			ctrCnfg: `
value: source.int64
# int64Primitive: source.int64 # missing int64Primitive
boolPrimitive: source.bool
doublePrimitive: source.double
stringPrimitive: source.string
dimensions:
  source: source.string
  target: source.string
`,
			cstrParam:          &sample_report.InstanceParam{},
			typeEvalError:      nil,
			wantValueType:      pb.INT64,
			wantDimensionsType: map[string]pb.ValueType{"source": pb.STRING, "target": pb.STRING},
			wantErr:            "expression for field Int64Primitive cannot be empty",
			willPanic:          false,
		},
		{
			name: "InferredTypeNotMatchStaticTypeFromTemplate",
			ctrCnfg: `
value: source.int64
int64Primitive: source.int64 # missing int64Primitive
boolPrimitive: source.bool
doublePrimitive: source.double
stringPrimitive: source.double # Double does not match string
dimensions:
  source: source.string
  target: source.string
`,
			cstrParam:          &sample_report.InstanceParam{},
			typeEvalError:      nil,
			wantValueType:      pb.INT64,
			wantDimensionsType: map[string]pb.ValueType{"source": pb.STRING, "target": pb.STRING},
			wantErr:            "error type checking for field StringPrimitive: Evaluated expression type DOUBLE want STRING",
			willPanic:          false,
		},
		{
			name:      "NotValidInstanceParam",
			ctrCnfg:   ``,
			cstrParam: &empty.Empty{}, // cnstr type mismatch
			wantErr:   "is not of type",
			willPanic: true,
		},
		{
			name: "ErrorFromTypeEvaluator",
			ctrCnfg: `
value: response.int64
dimensions:
  source: source.string
`,
			cstrParam:     &sample_report.InstanceParam{},
			typeEvalError: fmt.Errorf("some expression x.y.z is invalid"),
			wantErr:       "some expression x.y.z is invalid",
		},
	} {
		t.Run(tst.name, func(t *testing.T) {
			cp := tst.cstrParam
			_ = fillProto(tst.ctrCnfg, cp)
			typeEvalFn := getExprEvalFunc(tst.typeEvalError)
			defer func() {
				r := recover()
				if tst.willPanic && r == nil {
					t.Errorf("Expected to recover from panic for %s, but recover was nil.", tst.name)
				} else if !tst.willPanic && r != nil {
					t.Errorf("got panic %v, expected success.", r)
				}
			}()
			cv, cerr := SupportedTmplInfo[sample_report.TemplateName].InferType(cp.(proto.Message), typeEvalFn)
			if tst.wantErr == "" {
				if cerr != nil {
					t.Errorf("got err %v\nwant <nil>", cerr)
				}
				if tst.wantValueType != cv.(*sample_report.Type).Value {
					t.Errorf("got inferTypeForSampleReport(\n%s\n).value=%v\nwant %v",
						tst.ctrCnfg, cv.(*sample_report.Type).Value, tst.wantValueType)
				}
				if len(tst.wantDimensionsType) != len(cv.(*sample_report.Type).Dimensions) {
					t.Errorf("got len ( inferTypeForSampleReport(\n%s\n).dimensions) =%v \n want %v",
						tst.ctrCnfg, len(cv.(*sample_report.Type).Dimensions), len(tst.wantDimensionsType))
				}
				for a, b := range tst.wantDimensionsType {
					if cv.(*sample_report.Type).Dimensions[a] != b {
						t.Errorf("got inferTypeForSampleReport(\n%s\n).dimensions[%s] =%v \n want %v",
							tst.ctrCnfg, a, cv.(*sample_report.Type).Dimensions[a], b)
					}
				}
			} else {
				if cerr == nil || !strings.Contains(cerr.Error(), tst.wantErr) {
					t.Errorf("got error %v\nwant %v", cerr, tst.wantErr)
				}
			}
		})
	}
}

func TestInferTypeForSampleCheck(t *testing.T) {
	for _, tst := range []inferTypeTest{
		{
			name: "SimpleValid",
			ctrCnfg: `
check_expression: source.string
`,
			cstrParam:     &sample_check.InstanceParam{},
			typeEvalError: nil,
			wantValueType: pb.STRING,
			wantErr:       "",
			willPanic:     false,
		},
		{
			name:      "NotValidInstanceParam",
			ctrCnfg:   ``,
			cstrParam: &empty.Empty{}, // cnstr type mismatch
			willPanic: true,
		},
	} {
		t.Run(tst.name, func(t *testing.T) {
			cp := tst.cstrParam
			_ = fillProto(tst.ctrCnfg, cp)
			typeEvalFn := getExprEvalFunc(tst.typeEvalError)
			defer func() {
				r := recover()
				if tst.willPanic && r == nil {
					t.Errorf("Expected to recover from panic for %s, but recover was nil.", tst.name)
				} else if !tst.willPanic && r != nil {
					t.Errorf("got panic %v, expected success.", r)
				}
			}()
			_, cerr := SupportedTmplInfo[sample_check.TemplateName].InferType(cp.(proto.Message), typeEvalFn)
			if tst.willPanic {
				t.Error("Should not reach this statement due to panic.")
			}
			if tst.wantErr == "" {
				if cerr != nil {
					t.Errorf("got err %v\nwant <nil>", cerr)
				}
			} else {
				if cerr == nil || !strings.Contains(cerr.Error(), tst.wantErr) {
					t.Errorf("got error %v\nwant %v", cerr, tst.wantErr)
				}
			}
		})
	}
}

func TestInferTypeForSampleQuota(t *testing.T) {
	for _, tst := range []inferTypeTest{
		{
			name: "SimpleValid",
			ctrCnfg: `
dimensions:
  source: source.string
  target: source.string
  env: target.string
`,
			cstrParam:          &sample_quota.InstanceParam{},
			typeEvalError:      nil,
			wantValueType:      pb.STRING,
			wantDimensionsType: map[string]pb.ValueType{"source": pb.STRING, "target": pb.STRING, "env": pb.STRING},
			wantErr:            "",
			willPanic:          false,
		},
		{
			name:      "NotValidInstanceParam",
			ctrCnfg:   ``,
			cstrParam: &empty.Empty{}, // cnstr type mismatch
			wantErr:   "is not of type",
			willPanic: true,
		},
		{
			name: "ErrorFromTypeEvaluator",
			ctrCnfg: `
dimensions:
  source: source.ip
`,
			cstrParam:     &sample_quota.InstanceParam{},
			typeEvalError: fmt.Errorf("some expression x.y.z is invalid"),
			wantErr:       "some expression x.y.z is invalid",
		},
	} {
		t.Run(tst.name, func(t *testing.T) {
			cp := tst.cstrParam
			_ = fillProto(tst.ctrCnfg, cp)
			typeEvalFn := getExprEvalFunc(tst.typeEvalError)
			defer func() {
				r := recover()
				if tst.willPanic && r == nil {
					t.Errorf("Expected to recover from panic for %s, but recover was nil.", tst.name)
				} else if !tst.willPanic && r != nil {
					t.Errorf("got panic %v, expected success.", r)
				}
			}()
			cv, cerr := SupportedTmplInfo[sample_quota.TemplateName].InferType(cp.(proto.Message), typeEvalFn)
			if tst.wantErr == "" {
				if cerr != nil {
					t.Errorf("got err %v\nwant <nil>", cerr)
				}
				if len(tst.wantDimensionsType) != len(cv.(*sample_quota.Type).Dimensions) {
					t.Errorf("got len ( inferTypeForSampleReport(\n%s\n).dimensions) =%v \n want %v",
						tst.ctrCnfg, len(cv.(*sample_quota.Type).Dimensions), len(tst.wantDimensionsType))
				}
				for a, b := range tst.wantDimensionsType {
					if cv.(*sample_quota.Type).Dimensions[a] != b {
						t.Errorf("got inferTypeForSampleReport(\n%s\n).dimensions[%s] =%v \n want %v",
							tst.ctrCnfg, a, cv.(*sample_quota.Type).Dimensions[a], b)
					}
				}

			} else {
				if cerr == nil || !strings.Contains(cerr.Error(), tst.wantErr) {
					t.Errorf("got error %v\nwant %v", cerr, tst.wantErr)
				}
			}
		})
	}
}

type ConfigureTypeTest struct {
	name     string
	tmpl     string
	types    map[string]proto.Message
	hdlrBldr adapter.HandlerBuilder
	want     interface{}
}

func TestConfigureType(t *testing.T) {
	for _, tst := range []ConfigureTypeTest{
		{
			name:     "SimpleReport",
			tmpl:     sample_report.TemplateName,
			types:    map[string]proto.Message{"foo": &sample_report.Type{}},
			hdlrBldr: &fakeReportHandler{},
			want:     map[string]*sample_report.Type{"foo": {}},
		},
		{
			name:     "SimpleCheck",
			tmpl:     sample_check.TemplateName,
			types:    map[string]proto.Message{"foo": &sample_check.Type{}},
			hdlrBldr: &fakeCheckHandler{},
			want:     map[string]*sample_check.Type{"foo": {}},
		},
		{
			name:     "SimpleQuota",
			tmpl:     sample_quota.TemplateName,
			types:    map[string]proto.Message{"foo": &sample_quota.Type{}},
			hdlrBldr: &fakeQuotaHandler{},
			want:     map[string]*sample_quota.Type{"foo": {}},
		},
	} {
		t.Run(tst.name, func(t *testing.T) {
			hb := &tst.hdlrBldr
			_ = SupportedTmplInfo[tst.tmpl].ConfigureType(tst.types, hb)

			var c interface{}
			if tst.tmpl == sample_report.TemplateName {
				c = tst.hdlrBldr.(*fakeReportHandler).cnfgCallInput
			} else if tst.tmpl == sample_check.TemplateName {
				c = tst.hdlrBldr.(*fakeCheckHandler).cnfgCallInput
			} else if tst.tmpl == sample_quota.TemplateName {
				c = tst.hdlrBldr.(*fakeQuotaHandler).cnfgCallInput
			}
			if !reflect.DeepEqual(c, tst.want) {
				t.Errorf("SupportedTmplInfo[%s].ConfigureType(%v) handler invoked value = %v, want %v", tst.tmpl, tst.types, c, tst.want)
			}
		})
	}
}

func TestProcessReport(t *testing.T) {
	for _, tst := range []struct {
		name         string
		insts        map[string]proto.Message
		hdlr         adapter.Handler
		wantInstance interface{}
		wantError    string
	}{
		{
			name: "Simple",
			insts: map[string]proto.Message{
				"foo": &sample_report.InstanceParam{Value: "1", Dimensions: map[string]string{"s": "2"}, BoolPrimitive: "true",
					DoublePrimitive: "1.2", Int64Primitive: "54362", StringPrimitive: `"myString"`, Int64Map: map[string]string{"a": "1"}},
				"bar": &sample_report.InstanceParam{Value: "2", Dimensions: map[string]string{"k": "3"}, BoolPrimitive: "true",
					DoublePrimitive: "1.2", Int64Primitive: "54362", StringPrimitive: `"myString"`, Int64Map: map[string]string{"b": "1"}},
			},
			hdlr: &fakeReportHandler{},
			wantInstance: []*sample_report.Instance{
				{
					Name:            "foo",
					Value:           int64(1),
					Dimensions:      map[string]interface{}{"s": int64(2)},
					BoolPrimitive:   true,
					DoublePrimitive: 1.2,
					Int64Primitive:  54362,
					StringPrimitive: "myString",
					Int64Map:        map[string]int64{"a": int64(1)},
				},
				{
					Name:            "bar",
					Value:           int64(2),
					Dimensions:      map[string]interface{}{"k": int64(3)},
					BoolPrimitive:   true,
					DoublePrimitive: 1.2,
					Int64Primitive:  54362,
					StringPrimitive: "myString",
					Int64Map:        map[string]int64{"b": int64(1)},
				},
			},
		},
		{
			name: "EvalAllError",
			insts: map[string]proto.Message{
				"foo": &sample_report.InstanceParam{Value: "1", Dimensions: map[string]string{"s": "bad.attribute"}, BoolPrimitive: "true",
					DoublePrimitive: "1.2", Int64Primitive: "54362", StringPrimitive: `"myString"`, Int64Map: map[string]string{"a": "1"}},
			},
			hdlr:      &fakeReportHandler{},
			wantError: "unresolved attribute bad.attribute",
		},
		{
			name: "EvalError",
			insts: map[string]proto.Message{
				"foo": &sample_report.InstanceParam{Value: "bad.attribute", Dimensions: map[string]string{"s": "2"}, BoolPrimitive: "true",
					DoublePrimitive: "1.2", Int64Primitive: "54362", StringPrimitive: `"myString"`, Int64Map: map[string]string{"a": "1"}},
			},
			hdlr:      &fakeReportHandler{},
			wantError: "unresolved attribute bad.attribute",
		},
		{
			name: "ProcessError",
			insts: map[string]proto.Message{
				"foo": &sample_report.InstanceParam{Value: "1", Dimensions: map[string]string{"s": "2"}, BoolPrimitive: "true",
					DoublePrimitive: "1.2", Int64Primitive: "54362", StringPrimitive: `"myString"`, Int64Map: map[string]string{"a": "1"}},
			},
			hdlr:      &fakeReportHandler{retError: fmt.Errorf("error from process method")},
			wantError: "error from process method",
		},
	} {
		t.Run(tst.name, func(t *testing.T) {
			h := &tst.hdlr
			ev, _ := expr.NewCEXLEvaluator(expr.DefaultCacheSize)
			err := SupportedTmplInfo[sample_report.TemplateName].ProcessReport(context.TODO(), tst.insts, fakeBag{}, ev, *h)

			if tst.wantError != "" {
				if !strings.Contains(err.Error(), tst.wantError) {
					t.Errorf("ProcessReport got error = %s, want %s", err.Error(), tst.wantError)
				}
			} else {
				if err != nil {
					t.Errorf("ProcessReport got error %v , want success", err)
				}
				v := (*h).(*fakeReportHandler).procCallInput.([]*sample_report.Instance)
				if !cmp(v, tst.wantInstance) {
					t.Errorf("ProcessReport handler invoked value = %v, want %v", spew.Sdump(v), spew.Sdump(tst.wantInstance))
				}
			}
		})
	}
}

func TestProcessCheck(t *testing.T) {
	for _, tst := range []struct {
		name            string
		instName        string
		inst            proto.Message
		hdlr            adapter.Handler
		wantInstance    interface{}
		wantCheckResult adapter.CheckResult
		wantError       string
	}{
		{
			name:     "Simple",
			instName: "foo",
			inst: &sample_check.InstanceParam{
				CheckExpression: `"abcd asd"`,
				StringMap:       map[string]string{"a": `"aaa"`},
			},
			hdlr: &fakeCheckHandler{
				retResult: adapter.CheckResult{Status: rpc.Status{Message: "msg"}},
			},
			wantInstance:    &sample_check.Instance{Name: "foo", CheckExpression: "abcd asd", StringMap: map[string]string{"a": "aaa"}},
			wantCheckResult: adapter.CheckResult{Status: rpc.Status{Message: "msg"}},
		},
		{
			name:     "EvalError",
			instName: "foo",
			inst: &sample_check.InstanceParam{
				CheckExpression: `"abcd asd"`,
				StringMap:       map[string]string{"a": "bad.attribute"},
			},
			wantError: "unresolved attribute bad.attribute",
		},
		{
			name:     "ProcessError",
			instName: "foo",
			inst: &sample_check.InstanceParam{
				CheckExpression: `"abcd asd"`,
				StringMap:       map[string]string{"a": `"aaa"`},
			},
			hdlr: &fakeCheckHandler{
				retError: fmt.Errorf("some error"),
			},
			wantError: "some error",
		},
	} {
		t.Run(tst.name, func(t *testing.T) {
			h := &tst.hdlr
			ev, _ := expr.NewCEXLEvaluator(expr.DefaultCacheSize)
			res, err := SupportedTmplInfo[sample_check.TemplateName].ProcessCheck(context.TODO(), tst.instName, tst.inst, fakeBag{}, ev, *h)

			if tst.wantError != "" {
				if !strings.Contains(err.Error(), tst.wantError) {
					t.Errorf("ProcessCheckSample got error = %s, want %s", err.Error(), tst.wantError)
				}
			} else {
				v := (*h).(*fakeCheckHandler).procCallInput
				if !reflect.DeepEqual(v, tst.wantInstance) {
					t.Errorf("CheckSample handler "+
						"invoked value = %v want %v", spew.Sdump(v), spew.Sdump(tst.wantInstance))
				}
				if !reflect.DeepEqual(tst.wantCheckResult, res) {
					t.Errorf("CheckSample result = %v want %v", res, spew.Sdump(tst.wantCheckResult))
				}
			}
		})
	}
}

func TestProcessQuota(t *testing.T) {
	for _, tst := range []struct {
		name            string
		instName        string
		inst            proto.Message
		hdlr            adapter.Handler
		wantInstance    interface{}
		wantQuotaResult adapter.QuotaResult2
		wantError       string
	}{
		{
			name:     "Simple",
			instName: "foo",
			inst: &sample_quota.InstanceParam{
				Dimensions: map[string]string{"a": `"str"`},
				BoolMap:    map[string]string{"a": "true"},
			},
			hdlr: &fakeQuotaHandler{
				retResult: adapter.QuotaResult2{Amount: 1},
			},
			wantInstance:    &sample_quota.Instance{Name: "foo", Dimensions: map[string]interface{}{"a": "str"}, BoolMap: map[string]bool{"a": true}},
			wantQuotaResult: adapter.QuotaResult2{Amount: 1},
		},
		{
			name:     "EvalError",
			instName: "foo",
			inst: &sample_quota.InstanceParam{
				Dimensions: map[string]string{"a": "bad.attribute"},
				BoolMap:    map[string]string{"a": "true"},
			},
			wantError: "unresolved attribute bad.attribute",
		},
		{
			name:     "ProcessError",
			instName: "foo",
			inst: &sample_quota.InstanceParam{
				Dimensions: map[string]string{"a": `"str"`},
				BoolMap:    map[string]string{"a": "true"},
			},
			hdlr: &fakeQuotaHandler{
				retError: fmt.Errorf("some error"),
			},
			wantError: "some error",
		},
	} {
		t.Run(tst.name, func(t *testing.T) {
			h := &tst.hdlr
			ev, _ := expr.NewCEXLEvaluator(expr.DefaultCacheSize)
			res, err := SupportedTmplInfo[sample_quota.TemplateName].ProcessQuota(context.TODO(), tst.instName, tst.inst, fakeBag{}, ev, *h, adapter.QuotaRequestArgs{})

			if tst.wantError != "" {
				if !strings.Contains(err.Error(), tst.wantError) {
					t.Errorf("ProcessQuotaSample got error = %s, want %s", err.Error(), tst.wantError)
				}
			} else {
				v := (*h).(*fakeQuotaHandler).procCallInput
				if !reflect.DeepEqual(v, tst.wantInstance) {
					t.Errorf("ProcessQuotaSample handler "+
						"invoked value = %v want %v", spew.Sdump(v), spew.Sdump(tst.wantInstance))
				}
				if !reflect.DeepEqual(tst.wantQuotaResult, res) {
					t.Errorf("ProcessQuotaSample result = %v want %v", res, spew.Sdump(tst.wantQuotaResult))
				}
			}
		})
	}
}

func cmp(m interface{}, n interface{}) bool {
	a := InterfaceSlice(m)
	b := InterfaceSlice(n)
	if len(a) != len(b) {
		return false
	}

	for _, x1 := range a {
		f := false
		for _, x2 := range b {
			if reflect.DeepEqual(x1, x2) {
				f = true
			}
		}
		if !f {
			return false
		}
	}
	return true
}

func InterfaceSlice(slice interface{}) []interface{} {
	s := reflect.ValueOf(slice)

	ret := make([]interface{}, s.Len())
	for i := 0; i < s.Len(); i++ {
		ret[i] = s.Index(i).Interface()
	}

	return ret
}

func fillProto(cfg string, o interface{}) error {
	//data []byte, m map[string]interface{}, err error
	var m map[string]interface{}
	var data []byte
	var err error

	if err = yaml.Unmarshal([]byte(cfg), &m); err != nil {
		return err
	}

	if data, err = json.Marshal(m); err != nil {
		return err
	}

	err = yaml.Unmarshal(data, o)
	return err
}
