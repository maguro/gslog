// Copyright 2024 The original author or authors.
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

// Package gslog contains Google Cloud Logging handlers for slog.
//
// The handlers are in the packages gcp and stdout.  The options, labels,
// and levels that the handlers share are in the package core.  This package
// exports the names of release v0.23.0, and imports the API client.  A
// program that uses only the stdout handler imports the packages core and
// stdout, and imports no client library.
//
// Deprecated: Use the packages gcp, stdout, and core.
package gslog
