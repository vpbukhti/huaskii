# Future Development

## Filler Letter Orientation - DONE
Implemented using average normal direction:
- Sample normal at 5 points along curve (t = 0.1, 0.3, 0.5, 0.7, 0.9)
- Use right-perpendicular as outward normal for clockwise TrueType contours
- If average normal points upward (Y < 0), reverse the curve
- Reversing = reversing control point order
- See `geom/bezier.go`: `GetNormalAt()`, `AverageNormal()`, `ShouldReverse()`, `Reversed()`

## Row Packing Control
Add parameter to control how tightly packed rows of filler letters are:
- Currently rows are packed based on `fillerHeight`
- Add `RowSpacing` or `RowPadding` setting to RenderSettings
- Allow negative values for overlap, positive for gaps

## Randomize Starting Position - DONE
Implemented per-row randomization:
- Random starting character index (skipping whitespace)
- Random distance offset (0 to 0.5 * fillerHeight) at start of each row
- Prevents visible striping patterns across multiple rows

## Main Text Background
Add background to main text letter by letter:
- Draw a background shape behind each main text letter before rendering filler
- Could use the glyph bounding box or actual glyph outline with padding
- Useful for improving contrast/readability of the final output
