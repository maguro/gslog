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
	"log/slog"
	"testing"
	"time"

	"cloud.google.com/go/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"m4o.io/gslog"
)

func TestNewGcpHandler(t *testing.T) {
	var entries []logging.Entry
	logger := gslog.LoggerFunc(func(e logging.Entry) {
		entries = append(entries, e)
	})
	level := gslog.WithLogLeveler(slog.LevelWarn)

	handler := gslog.NewGcpHandler(logger, level)

	ctx := context.Background()
	assert.False(t, handler.Enabled(ctx, slog.LevelInfo))
	assert.True(t, handler.Enabled(ctx, slog.LevelWarn))

	record := slog.NewRecord(time.Now(), slog.LevelWarn, "hello", 0)
	require.NoError(t, handler.Handle(ctx, record))

	require.Len(t, entries, 1)
	assert.Equal(t, logging.Warning, entries[0].Severity)
}
