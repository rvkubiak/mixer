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

package store

import (
	"bytes"
	"context"
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"
)

const defaultDuration = time.Second / 2

var supportedExtensions = map[string]bool{
	".yaml": true,
	".yml":  true,
}

// resource is almost identical to crd/resource.go. This is defined here
// separately because:
// - no dependencies on actual k8s libraries
// - sha1 hash field, required for fsstore to check updates
type resource struct {
	Kind       string
	APIVersion string `json:"apiVersion"`
	Metadata   ResourceMeta
	Spec       map[string]interface{}
	sha        [sha1.Size]byte
}

func (r *resource) Key() Key {
	return Key{Kind: r.Kind, Namespace: r.Metadata.Namespace, Name: r.Metadata.Name}
}

// fsStore2 is Store2Backend implementation using filesystem.
type fsStore2 struct {
	root          string
	kinds         map[string]bool
	checkDuration time.Duration

	watchMutex sync.RWMutex
	watchCtx   context.Context
	watchCh    chan BackendEvent

	mu   sync.RWMutex
	data map[Key]*resource
}

var _ Store2Backend = &fsStore2{}

// parseFile parses the data and returns as a slice of resources. "path" is only used
// for error reporting.
func parseFile(path string, data []byte) []*resource {
	if bytes.HasPrefix(data, []byte("---\n")) {
		data = data[4:]
	}
	if bytes.HasSuffix(data, []byte("\n")) {
		data = data[:len(data)-1]
	}
	if bytes.HasSuffix(data, []byte("\n---")) {
		data = data[:len(data)-4]
	}
	if len(data) == 0 {
		return nil
	}
	chunks := bytes.Split(data, []byte("\n---\n"))
	resources := make([]*resource, 0, len(chunks))
	for i, chunk := range chunks {
		r, err := parseChunk(chunk)
		if err != nil {
			glog.Errorf("Error processing %s[%d]: %v", path, i, err)
			continue
		}
		if r == nil {
			continue
		}
		resources = append(resources, r)
	}
	return resources
}

func parseChunk(chunk []byte) (*resource, error) {
	r := &resource{}
	if err := yaml.Unmarshal(chunk, r); err != nil {
		return nil, err
	}
	if empty(r) {
		// can be empty because
		// There is just white space
		// There are just comments
		return nil, nil
	}
	if r.Kind == "" || r.Metadata.Namespace == "" || r.Metadata.Name == "" {
		return nil, fmt.Errorf("key elements are empty. Extracted as %s from\n <<%s>>", r.Key(), string(chunk))
	}
	r.sha = sha1.Sum(chunk)
	return r, nil
}

var emptyResource = &resource{}

// Check if the parsed resource is empty
func empty(r *resource) bool {
	return reflect.DeepEqual(*r, *emptyResource)
}

func (s *fsStore2) readFiles() map[Key]*resource {
	result := map[Key]*resource{}

	err := filepath.Walk(s.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !supportedExtensions[filepath.Ext(path)] || (info.Mode()&os.ModeType) != 0 {
			return nil
		}
		data, err := ioutil.ReadFile(path)
		if err != nil {
			glog.Warningf("Failed to read %s: %v", path, err)
			return err
		}
		for _, r := range parseFile(path, data) {
			k := r.Key()
			if !s.kinds[k.Kind] {
				continue
			}
			result[r.Key()] = r
		}
		return nil
	})
	if err != nil {
		glog.Errorf("failure during filepath.Walk: %v", err)
	}
	return result
}

func (s *fsStore2) checkAndUpdate() {
	newData := s.readFiles()
	s.mu.Lock()
	defer s.mu.Unlock()
	updated := []Key{}
	removed := map[Key]bool{}
	for k := range s.data {
		removed[k] = true
	}
	for k, r := range newData {
		oldR, ok := s.data[k]
		if !ok {
			updated = append(updated, k)
			continue
		}
		delete(removed, k)
		if r.sha != oldR.sha {
			updated = append(updated, k)
		}
	}
	if len(updated) == 0 && len(removed) == 0 {
		return
	}
	s.data = newData
	s.watchMutex.RLock()
	defer s.watchMutex.RUnlock()
	if s.watchCtx == nil || s.watchCtx.Err() != nil {
		return
	}
	evs := make([]BackendEvent, 0, len(updated)+len(removed))
	for _, key := range updated {
		br := &BackEndResource{
			Metadata: s.data[key].Metadata,
			Spec:     s.data[key].Spec,
		}
		evs = append(evs, BackendEvent{Key: key, Type: Update, Value: br})
	}
	for key := range removed {
		evs = append(evs, BackendEvent{Key: key, Type: Delete})
	}
	for _, ev := range evs {
		select {
		case <-s.watchCtx.Done():
		case s.watchCh <- ev:
		}
	}
}

// NewFsStore2 creates a new Store2Backend backed by the filesystem.
func NewFsStore2(root string) Store2Backend {
	return &fsStore2{
		root:          root,
		kinds:         map[string]bool{},
		checkDuration: defaultDuration,
		data:          map[Key]*resource{},
	}
}

// Init implements Store2Backend interface.
func (s *fsStore2) Init(ctx context.Context, kinds []string) error {
	for _, k := range kinds {
		s.kinds[k] = true
	}
	s.checkAndUpdate()
	go func() {
		tick := time.NewTicker(s.checkDuration)
		for {
			select {
			case <-ctx.Done():
				tick.Stop()
				return
			case <-tick.C:
				s.checkAndUpdate()
			}
		}
	}()
	return nil
}

// Watch implements Store2Backend interface.
func (s *fsStore2) Watch(ctx context.Context) (<-chan BackendEvent, error) {
	ch := make(chan BackendEvent)
	s.watchMutex.Lock()
	s.watchCtx = ctx
	s.watchCh = ch
	s.watchMutex.Unlock()
	return ch, nil
}

// Get implements Store2Backend interface.
func (s *fsStore2) Get(key Key) (*BackEndResource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.data[key]
	if !ok {
		return nil, ErrNotFound
	}
	return &BackEndResource{
		Metadata: r.Metadata,
		Spec:     r.Spec,
	}, nil
}

// List implements Store2Backend interface.
func (s *fsStore2) List() map[Key]*BackEndResource {
	s.mu.RLock()
	result := make(map[Key]*BackEndResource, len(s.data))
	for k, r := range s.data {
		result[k] = &BackEndResource{
			Metadata: r.Metadata,
			Spec:     r.Spec,
		}
	}
	s.mu.RUnlock()
	return result
}
