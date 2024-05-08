// Copyright 2024 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package textblock

// This package is similar to "encoding/pem" but allows custom headers and
// footers. This make it easier to integrate with any configuration
// language and their comment syntax.
//
// Example:
//
//   const data = `
//   unrelated data
//   # ---- BEGIN LOREMIPSUM ----
//   Lorem ipsum dolor sit amet,
//   consectetur adipiscing elit
//   # ---- END LOREMIPSUM ----
//   more unrelated data
//   ``
//
//   parser := block.NewBlockWriter(
//       "# ---- BEGIN LOREMIPSUM ----",
//       "# ---- END LOREMIPSUM ----",
//   )
//   blocks, err := parser.FindBlocks(data)
//   print(blocks[0])
//
// Output:
//
//       Lorem ipsum dolor sit amet,
//       consectetur adipiscing elit
//
