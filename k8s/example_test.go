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

package k8s_test

import (
	"context"
	"log/slog"
	"os"
	"time"

	"m4o.io/gslog/k8s"
	"m4o.io/gslog/stdout"
)

// In a Pod, a volume of the Downward API holds the labels file.  The option
// takes the directory of the volume.  Each entry then has the labels of the
// Pod.  The example uses the labels file in the testdata directory.  The
// handler adds the prefix "k8s-pod/" to each label.  The handler writes the
// labels in key order.  A fixed time keeps the timestamp of the output the
// same on each run.
func ExampleWithPodinfoLabels() {
	h := stdout.NewHandler(os.Stdout, k8s.WithPodinfoLabels("testdata/etc/podinfo"))

	at := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	r := slog.NewRecord(at, slog.LevelInfo, "How now brown cow?", 0)

	_ = h.Handle(context.Background(), r)

	// Output: {"severity":"INFO","message":"How now brown cow?","timestamp":"2024-01-02T03:04:05Z","logging.googleapis.com/labels":{"k8s-pod/app":"hello-world","k8s-pod/environment":"stg","k8s-pod/tier":"backend","k8s-pod/track":"stable"}}
}
