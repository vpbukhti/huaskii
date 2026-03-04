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

## Fix Filler Letters Center of Mass
Inconsistent horizontal spacing between filler letters on a single row:
- Some letters clump together, others have too much padding on left/right

Proposed fix (X adjustment only, no vertical adjustment):
1. Sum brightness for each vertical column (X position) of the rasterized glyph
2. Find the X position that represents the horizontal center of mass
3. Normalize all glyph widths to the same size (use the largest needed)
   - Pre-scan all filler characters to find max width
   - Position each glyph so its center of mass aligns with bbox center
4. Spacing is then uniform: each glyph occupies the same horizontal space
