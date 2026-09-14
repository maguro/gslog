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

package gslog

import (
	"context"

	"m4o.io/gslog/core"
)

// LabelPair represents a key-value string pair.
//
// Deprecated: Use core.LabelPair.
type LabelPair = core.LabelPair

// Label returns a new LabelPair from a key and a value.
//
// Deprecated: Use core.Label.
func Label(key, value string) LabelPair {
	return core.Label(key, value)
}

// WithLabels returns a new Context that has the labels of ctx and the
// supplied labels.
//
// Deprecated: Use core.WithLabels.
func WithLabels(ctx context.Context, labelPairs ...LabelPair) context.Context {
	return core.WithLabels(ctx, labelPairs...)
}

// ExtractLabels returns the labels that WithLabels stored in ctx.
//
// Deprecated: Use core.ExtractLabels.
func ExtractLabels(ctx context.Context) map[string]string {
	return core.ExtractLabels(ctx)
}
