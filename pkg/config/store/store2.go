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
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/glog"
)

// ErrNotFound is the error to be returned when the given key does not exist in the storage.
var ErrNotFound = errors.New("not found")

// Key represents the key to identify a resource in the store.
type Key struct {
	Kind      string
	Namespace string
	Name      string
}

// BackendEvent is an event used between Store2Backend and Store2.
type BackendEvent struct {
	Key
	Type  ChangeType
	Value map[string]interface{}
}

// String is the Istio compatible string representation of the resource.
// Name.Kind.Namespace
// At the point of use Namespace can be omitted, and it is assumed to be the namespace
// of the document.
func (k Key) String() string {
	return k.Name + "." + k.Kind + "." + k.Namespace
}

// Event represents an event. Used by Store2.Watch.
type Event struct {
	Key
	Type ChangeType

	// Value refers the new value in the updated event. nil if the event type is delete.
	Value proto.Message
}

// Validator defines the interface to validate a new change.
type Validator interface {
	Validate(t ChangeType, key Key, spec proto.Message) bool
}

// Store2Backend defines the typeless storage backend for mixer.
// TODO: rename to StoreBackend.
type Store2Backend interface {
	Init(ctx context.Context, kinds []string) error

	// Watch creates a channel to receive the events.
	Watch(ctx context.Context) (<-chan BackendEvent, error)

	// Get returns a resource's spec to the key.
	Get(key Key) (map[string]interface{}, error)

	// List returns the whole mapping from key to resource specs in the store.
	List() map[Key]map[string]interface{}
}

// Store2 defines the access to the storage for mixer.
// TODO: rename to Store.
type Store2 interface {
	Init(ctx context.Context, kinds map[string]proto.Message) error

	// Watch creates a channel to receive the events.
	Watch(ctx context.Context) (<-chan Event, error)

	// Get returns a resource's spec to the key.
	Get(key Key, spec proto.Message) error

	// List returns the whole mapping from key to resource specs in the store.
	List() map[Key]proto.Message
}

// store2 is the implementation of Store2 interface.
type store2 struct {
	kinds   map[string]proto.Message
	backend Store2Backend
}

// Init initializes the connection with the storage backend. This uses "kinds"
// for the mapping from the kind's name and its structure in protobuf.
// The connection will be closed after ctx is done.
func (s *store2) Init(ctx context.Context, kinds map[string]proto.Message) error {
	kindNames := make([]string, 0, len(kinds))
	for k := range kinds {
		kindNames = append(kindNames, k)
	}
	if err := s.backend.Init(ctx, kindNames); err != nil {
		return err
	}
	s.kinds = kinds
	return nil
}

// Watch creates a channel to receive the events.
func (s *store2) Watch(ctx context.Context) (<-chan Event, error) {
	ch, err := s.backend.Watch(ctx)
	if err != nil {
		return nil, err
	}
	q := newQueue(ctx, ch, s.kinds)
	return q.chout, nil
}

// Get returns a resource's spec to the key.
func (s *store2) Get(key Key, spec proto.Message) error {
	obj, err := s.backend.Get(key)
	if err != nil {
		return err
	}

	return convert(obj, spec)
}

// List returns the whole mapping from key to resource specs in the store.
func (s *store2) List() map[Key]proto.Message {
	data := s.backend.List()
	result := make(map[Key]proto.Message, len(data))
	for k, spec := range data {
		pbSpec, err := cloneMessage(k.Kind, s.kinds)
		if err != nil {
			glog.Errorf("Failed to convert spec: %v", err)
			continue
		}
		if err = convert(spec, pbSpec); err != nil {
			glog.Errorf("Failed to convert spec: %v", err)
			continue
		}
		result[k] = pbSpec
	}
	return result
}

// Store2Builder is the type of function to build a Store2Backend.
type Store2Builder func(u *url.URL) (Store2Backend, error)

// RegisterFunc2 is the type to register a builder for URL scheme.
type RegisterFunc2 func(map[string]Store2Builder)

// Registry2 keeps the relationship between the URL scheme and
// the Store2Backend implementation.
type Registry2 struct {
	builders map[string]Store2Builder
}

// NewRegistry2 creates a new Registry instance for the inventory.
func NewRegistry2(inventory ...RegisterFunc2) *Registry2 {
	b := map[string]Store2Builder{}
	for _, rf := range inventory {
		rf(b)
	}
	return &Registry2{builders: b}
}

// NewStore2 creates a new Store2 instance with the specified backend.
func (r *Registry2) NewStore2(configURL string) (Store2, error) {
	u, err := url.Parse(configURL)

	if err != nil {
		return nil, fmt.Errorf("invalid config URL %s %v", configURL, err)
	}

	s2 := &store2{}
	if builder, ok := r.builders[u.Scheme]; ok {
		s2.backend, err = builder(u)
		if err == nil {
			return s2, nil
		}
	}

	return nil, fmt.Errorf("unknown config URL %s %v", configURL, u)
}
