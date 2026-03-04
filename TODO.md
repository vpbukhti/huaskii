# Future Development

## Filler Letter Orientation
Reorient filler letters for better readability by reversing curve segments:
- For each bezier segment, check its overall direction (start to end)
- If segment goes right-to-left (or bottom-to-top for vertical segments), reverse it
- Reversing a bezier = reversing control point order:
  - Line: `p0, p1` → `p1, p0`
  - Quadratic: `p0, p1, p2` → `p2, p1, p0`
  - Cubic: `p0, p1, p2, p3` → `p3, p2, p1, p0`
- This way existing placement algorithm naturally places letters left-to-right
- Decision criteria options:
  - Simple: reverse if `end.X < start.X`
  - Smarter: consider dominant direction (horizontal vs vertical segments)

## Row Packing Control
Add parameter to control how tightly packed rows of filler letters are:
- Currently rows are packed based on `fillerHeight`
- Add `RowSpacing` or `RowPadding` setting to RenderSettings
- Allow negative values for overlap, positive for gaps

## Randomize Starting Position
Randomize the starting position of filler letters along each curve:
- Avoid visible striping patterns when multiple rows are rendered
- Add random offset (0 to charAdvance) at the start of each row
- Consider adding a `RandomSeed` parameter for reproducible results
