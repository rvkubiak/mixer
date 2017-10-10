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

package runtime

// TODO this file is copied from pkg/adapterManager.
// Remove the adapterManager package once adapters2 switch is complete.

import (
	"errors"
	"fmt"

	"github.com/golang/glog"

	"istio.io/mixer/pkg/adapter"
)

type logger struct {
	name string
}

func newLogger(name string) logger {
	return logger{name: name}
}

func (l logger) VerbosityLevel(level adapter.VerbosityLevel) bool {
	v := glog.V(glog.Level(level))
	return bool(v)
}

func (l logger) Infof(format string, args ...interface{}) {
	glog.InfoDepth(1, l.name+":"+fmt.Sprintf(format, args...))
}

func (l logger) Warningf(format string, args ...interface{}) {
	glog.WarningDepth(1, l.name+":"+fmt.Sprintf(format, args...))
}

func (l logger) Errorf(format string, args ...interface{}) error {
	s := fmt.Sprintf(format, args...)
	glog.ErrorDepth(1, l.name+":"+s)
	return errors.New(s)
}
