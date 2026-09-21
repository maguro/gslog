// Copyright the original author or authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gslog_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"m4o.io/gslog"
	"m4o.io/gslog/core"
)

func TestLabel(t *testing.T) {
	assert.Equal(t, core.Label("a", "one"), gslog.Label("a", "one"))
}

func TestWithLabels(t *testing.T) {
	ctx := context.Background()
	a := gslog.Label("a", "one")
	b := gslog.Label("b", "two")

	labeled := gslog.WithLabels(ctx, a, b)
	labels := gslog.ExtractLabels(labeled)

	assert.Equal(t, map[string]string{"a": "one", "b": "two"}, labels)
}

func TestExtractLabelsFromCore(t *testing.T) {
	ctx := context.Background()
	a := core.Label("a", "one")
	labeled := core.WithLabels(ctx, a)

	labels := gslog.ExtractLabels(labeled)

	assert.Equal(t, map[string]string{"a": "one"}, labels)
}
