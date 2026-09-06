//
//  Copyright 2026 The InfiniFlow Authors. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.
//

package cli

import (
	"testing"

	"github.com/ragflow/ragflow/internal/cli/filesystem"
	"github.com/stretchr/testify/assert"
)

// TestParseSearchOptionsReadsLimit covers issue #19284: a `search foo files -n 20`
// CLI invocation must propagate the user-supplied -n (top_k) to
// SearchOptions.Limit so FileProvider.Search uses page_size=20 instead
// of falling back to the default page_size=100.
//
// The fix is in filesystem_command.go: the search command's `Params`
// map now includes a `"limit"` key (in addition to `"top_k"`),
// mirroring the existing skills-search wiring.
func TestParseSearchOptionsReadsLimit(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		in        map[string]interface{}
		wantLimit int
		wantTopK  int
		wantQuery string
	}{
		{
			name:      "limit key alone populates Limit",
			in:        map[string]interface{}{"limit": 20},
			wantLimit: 20,
			wantTopK:  0,
		},
		{
			name:      "top_k key alone populates TopK",
			in:        map[string]interface{}{"top_k": 20},
			wantLimit: 0,
			wantTopK:  20,
		},
		{
			name:      "both keys populate both fields independently",
			in:        map[string]interface{}{"top_k": 20, "limit": 20},
			wantLimit: 20,
			wantTopK:  20,
		},
		{
			name:      "no keys leaves both fields at zero",
			in:        map[string]interface{}{"query": "secret"},
			wantLimit: 0,
			wantTopK:  0,
			wantQuery: "secret",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			opts := parseSearchOptions(tc.in)

			assert.Equal(t, tc.wantLimit, opts.Limit, "limit must propagate to SearchOptions.Limit")
			assert.Equal(t, tc.wantTopK, opts.TopK, "top_k must propagate to SearchOptions.TopK")
			assert.Equal(t, tc.wantQuery, opts.Query, "query must propagate to SearchOptions.Query")
		})
	}
}

// TestParseSearchCommandArgsSetsBothLimitAndTopK covers the CLI-to-command
// boundary for the -n flag, where the bug actually lived: parseSearchCommandArgs
// needs to populate BOTH “TopK“ and “Limit“ from the same user-supplied
// count, and the constructed “Params“ map must include the “"limit"“
// key for FileProvider.Search to pick it up.
func TestParseSearchCommandArgsSetsBothLimitAndTopK(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		args      []string
		wantLimit int
		wantTopK  int
	}{
		{
			name:      "search foo files -n 20 populates Limit and TopK",
			args:      []string{"foo", "files", "-n", "20"},
			wantLimit: 20,
			wantTopK:  20,
		},
		{
			name:      "no -n flag keeps both at default 10",
			args:      []string{"foo", "files"},
			wantLimit: 10,
			wantTopK:  10,
		},
		{
			name:      "search foo datasets -n 5 populates both",
			args:      []string{"foo", "datasets", "-n", "5"},
			wantLimit: 5,
			wantTopK:  5,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			opts, err := parseSearchCommandArgs(tc.args)
			assert.NoError(t, err)
			assert.Equal(t, tc.wantTopK, opts.TopK, "TopK must mirror -n for semantic search")
			assert.Equal(t, tc.wantLimit, opts.limit, "limit must mirror -n for file-search page_size")
		})
	}
}

// TestBuildSearchCommandPropagatesLimit covers the second half of the
// #19284 boundary: buildSearchCommand is what executeFilesystemInner
// sends to the engine. The Params map must include “limit“ so
// FileProvider.Search gets the user-supplied “-n“ (not the default
// page_size=100), and “top_k“ must remain so semantic / skills search
// still works.
func TestBuildSearchCommandPropagatesLimit(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		searchOpts    *SearchCommandOptions
		searchPath    string
		wantTopK      interface{}
		wantLimit     interface{}
		wantQuery     interface{}
		wantThreshold interface{}
		wantDirs      interface{}
	}{
		{
			name: "explicit -n 20 carries limit through",
			searchOpts: &SearchCommandOptions{
				Query:     "ragflow",
				TopK:      20,
				limit:     20,
				Threshold: 0.5,
				Dirs:      []string{"docs/"},
			},
			searchPath:    "tenant-a",
			wantTopK:      20,
			wantLimit:     20,
			wantQuery:     "ragflow",
			wantThreshold: 0.5,
			wantDirs:      []string{"docs/"},
		},
		{
			name: "no -n keeps limit at the parsed default 10",
			searchOpts: &SearchCommandOptions{
				Query:     "ragflow",
				TopK:      10,
				limit:     10,
				Threshold: 0.2,
				Dirs:      []string{},
			},
			searchPath:    "tenant-a",
			wantTopK:      10,
			wantLimit:     10,
			wantQuery:     "ragflow",
			wantThreshold: 0.2,
			wantDirs:      []string{},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmd := buildSearchCommand(tc.searchOpts, tc.searchPath)

			assert.Equal(t, filesystem.CommandSearch, cmd.Type, "command type must be CommandSearch")
			assert.Equal(t, tc.searchPath, cmd.Path, "command path must round-trip")
			assert.Equal(t, tc.wantTopK, cmd.Params["top_k"], "Params[\"top_k\"] must mirror -n for semantic search")
			assert.Equal(t, tc.wantLimit, cmd.Params["limit"], "Params[\"limit\"] must mirror -n for file-search page_size")
			assert.Equal(t, tc.wantQuery, cmd.Params["query"])
			assert.Equal(t, tc.wantThreshold, cmd.Params["threshold"])
			assert.Equal(t, tc.wantDirs, cmd.Params["dirs"])
		})
	}
}
