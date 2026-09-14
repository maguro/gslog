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

import "m4o.io/gslog/core"

// The levels that Cloud Logging has and slog does not.
const (
	// LevelNotice is slog.Level 2.
	//
	// Deprecated: Use core.LevelNotice.
	LevelNotice = core.LevelNotice
	// LevelCritical is slog.Level 12.
	//
	// Deprecated: Use core.LevelCritical.
	LevelCritical = core.LevelCritical
	// LevelAlert is slog.Level 16.
	//
	// Deprecated: Use core.LevelAlert.
	LevelAlert = core.LevelAlert
	// LevelEmergency is slog.Level 20.
	//
	// Deprecated: Use core.LevelEmergency.
	LevelEmergency = core.LevelEmergency
)
