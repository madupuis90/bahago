# World Map coordinates use a top-left origin

World Map tile coordinates use a top-left origin: (0,0) is the north-west
corner, X increases east, Y increases south. This replaces the earlier
bottom-left convention (Y increasing upward) that required a `py =
PageCount-1-row` row-flip on render in the world map board and minimap.

The decision was made during terrain authoring because an authored `[8][8]`
Region grid reads correctly top-to-bottom only under a top-left convention;
the flip was a source of off-by-one risk. Stored Kingdom X/Y values were **not**
remapped — the game is pre-launch with no real players, so seed Kingdoms moving
visually is acceptable. A future live-data remapping (`y → WorldSize-1-y`) is
not planned.