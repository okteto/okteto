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
