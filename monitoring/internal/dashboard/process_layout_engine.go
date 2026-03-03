package dashboard

// process_layout_engine.go implements coordinate computation for the SVG process view.
// Types are defined in process_layout.go. See SOW-022.0 for layout algorithm details.

import (
	"fmt"
	"math"
)

// ComputeProcessLayout takes a ProcessSchematic and computes display coordinates
// for all elements. The result is deterministic for a given input.
func ComputeProcessLayout(sc *ProcessSchematic, envID, envName string) *ProcessViewData {
	if sc == nil {
		return nil
	}

	pv := &ProcessViewData{
		EnvID:         envID,
		EnvName:       envName,
		FlowDirection: "horizontal",
		Defs:          SVGDefs{ArrowMarkerID: "arrow", MidArrowMarkerID: "mid-arrow"},
	}

	pv.Stages = computeStagePositions(sc.Stages)
	centerMap := buildEquipmentCenterMap(pv.Stages)
	pv.Connections = computeConnectionPaths(sc.Connections, centerMap, pv.Stages)
	pv.NetworkContext = computeNetworkContextBox(sc.NetworkContext, pv.Stages)
	pv.ViewBox = computeViewBox(pv.Stages, pv.NetworkContext)

	return pv
}

// computeStagePositions assigns x,y coordinates to each stage and its equipment.
func computeStagePositions(stages []StageDef) []StageLayout {
	layouts := make([]StageLayout, 0, len(stages))
	for i, stage := range stages {
		layouts = append(layouts, computeStageLayout(stage, i))
	}
	return layouts
}

// computeStageLayout positions one stage and its equipment.
func computeStageLayout(stage StageDef, stageIndex int) StageLayout {
	stageX := float64(stageIndex)*stageSpacing + stagePadding
	stageY := stagePadding

	equipment := computeEquipmentPositions(stage.Equipment, stageX, stageY)
	w, h := stageWidthHeight(equipment)

	return StageLayout{
		ID:         stage.ID,
		Name:       stage.Name,
		Controller: stage.Controller,
		X:          stageX,
		Y:          stageY,
		Width:      w,
		Height:     h,
		Equipment:  equipment,
	}
}

// computeEquipmentPositions assigns positions to equipment within a stage.
// All equipment in a stage share the same center X (fixed column).
// Equipment centers are spaced equipmentSpacing px apart vertically (center-to-center).
func computeEquipmentPositions(equip []EquipmentDef, stageX, stageY float64) []EquipmentLayout {
	columnCenterX := stageX + stageColumnWidth/2
	layouts := make([]EquipmentLayout, 0, len(equip))
	for i, e := range equip {
		w, h := equipmentSize(e.Type)
		equipY := stageY + stageHeaderHeight + float64(i)*equipmentSpacing + equipmentSpacing/2
		instruments := computeInstrumentPositions(e.Instruments, columnCenterX, equipY, h)
		layouts = append(layouts, EquipmentLayout{
			ID:          e.ID,
			Type:        e.Type,
			Label:       e.Label,
			X:           columnCenterX,
			Y:           equipY,
			Width:       w,
			Height:      h,
			Era:         e.Era,
			EraClass:    EraClass(e.Era),
			Instruments: instruments,
		})
	}
	return layouts
}

// computeInstrumentPositions assigns callout positions to instruments for one equipment item.
// Instruments are stacked 18px apart vertically, offset right of the equipment center.
func computeInstrumentPositions(instr []InstrumentDef, equipX, equipY, equipH float64) []InstrumentLayout {
	layouts := make([]InstrumentLayout, 0, len(instr))
	for i, inst := range instr {
		instX := equipX + instrumentOffsetX
		instY := equipY - equipH/4 + float64(i)*18.0
		layouts = append(layouts, InstrumentLayout{
			Tag:          inst.Tag,
			Name:         inst.Name,
			ISAType:      inst.ISAType,
			Placement:    inst.Placement,
			RegisterType: inst.RegisterType,
			RegisterAddr: inst.RegisterAddr,
			Unit:         inst.Unit,
			Range:        [2]float64{inst.RangeMin, inst.RangeMax},
			Scale:        [2]float64{inst.ScaleMin, inst.ScaleMax},
			X:            instX,
			Y:            instY,
		})
	}
	return layouts
}

// stageWidthHeight computes bounding box dimensions for a stage from its equipment.
func stageWidthHeight(equipment []EquipmentLayout) (width, height float64) {
	if len(equipment) == 0 {
		return stageColumnWidth + instrumentOffsetX + stagePadding, stageHeaderHeight + equipmentSpacing
	}
	maxRight := 0.0
	maxBottom := 0.0
	for _, e := range equipment {
		right := e.X + e.Width/2 + instrumentOffsetX + stagePadding
		bottom := e.Y + e.Height/2 + stagePadding
		if right > maxRight {
			maxRight = right
		}
		if bottom > maxBottom {
			maxBottom = bottom
		}
	}
	// Width is measured from the stage left edge (stageX), not equipment center.
	stageLeft := equipment[0].X - stageColumnWidth/2
	return maxRight - stageLeft, maxBottom
}

// buildEquipmentCenterMap returns a map of equipment ID to (cx, cy) center coordinates.
func buildEquipmentCenterMap(stages []StageLayout) map[string][2]float64 {
	m := make(map[string][2]float64)
	for _, stage := range stages {
		for _, eq := range stage.Equipment {
			m[eq.ID] = [2]float64{eq.X, eq.Y}
		}
	}
	return m
}

// computeConnectionPaths generates SVG path data for connections between equipment.
func computeConnectionPaths(conns []ConnectionDef, centers map[string][2]float64, stages []StageLayout) []ConnectionPath {
	paths := make([]ConnectionPath, 0, len(conns))
	for _, c := range conns {
		path, ok := buildConnectionPath(c, centers, stages)
		if !ok {
			continue
		}
		paths = append(paths, path)
	}
	return paths
}

// buildConnectionPath constructs one ConnectionPath for a connection definition.
// Returns false if either endpoint is not found in the center map.
func buildConnectionPath(c ConnectionDef, centers map[string][2]float64, stages []StageLayout) (ConnectionPath, bool) {
	from, okF := centers[c.From]
	to, okT := centers[c.To]
	if !okF || !okT {
		return ConnectionPath{}, false
	}
	fromW, _ := equipmentSizeFromStages(c.From, stages)
	toW, _ := equipmentSizeFromStages(c.To, stages)
	// Connect right edge of 'from' to left edge of 'to'. Vertical routing deferred (TD-041).
	startX, startY := from[0]+fromW/2, from[1]
	endX, endY := to[0]-toW/2, to[1]
	return ConnectionPath{
		FromID:   c.From,
		ToID:     c.To,
		Type:     c.Type,
		Label:    c.Label,
		PathData: buildPathData(startX, startY, endX, endY),
		ArrowEnd: true,
	}, true
}

// buildPathData returns the SVG path d attribute string for a straight line.
func buildPathData(x1, y1, x2, y2 float64) string {
	return fmt.Sprintf("M %.1f,%.1f L %.1f,%.1f", x1, y1, x2, y2)
}

// PipeLength returns the Euclidean distance between two SVG points.
func PipeLength(x1, y1, x2, y2 float64) float64 {
	dx, dy := x2-x1, y2-y1
	return math.Sqrt(dx*dx + dy*dy)
}

// NeedsMidArrow returns true when a pipe is long enough to require a mid-pipe arrowhead.
func NeedsMidArrow(x1, y1, x2, y2 float64) bool {
	return PipeLength(x1, y1, x2, y2) > minPipeArrowMid
}

// MidPoint returns the midpoint of the segment from (x1,y1) to (x2,y2).
func MidPoint(x1, y1, x2, y2 float64) (float64, float64) {
	return (x1 + x2) / 2.0, (y1 + y2) / 2.0
}

// equipmentSizeFromStages looks up equipment width/height by ID from stage layouts.
func equipmentSizeFromStages(equipID string, stages []StageLayout) (float64, float64) {
	for _, s := range stages {
		for _, e := range s.Equipment {
			if e.ID == equipID {
				return e.Width, e.Height
			}
		}
	}
	return 60, 60
}

// computeNetworkContextBox positions the network context callout box below the main flow.
func computeNetworkContextBox(items []NetworkContextDef, stages []StageLayout) *NetworkContextLayout {
	if len(items) == 0 {
		return nil
	}
	maxBottom := 0.0
	for _, s := range stages {
		if b := s.Y + s.Height; b > maxBottom {
			maxBottom = b
		}
	}
	itemHeight := 28.0
	contextItems := make([]NetworkContextItem, 0, len(items))
	for _, item := range items {
		contextItems = append(contextItems, NetworkContextItem{
			ID:       item.ID,
			Label:    item.Label,
			Type:     item.Type,
			Era:      item.Era,
			EraClass: EraClass(item.Era),
			Warning:  item.Warning,
			Notes:    item.Notes,
		})
	}
	return &NetworkContextLayout{
		X:      stagePadding,
		Y:      maxBottom + 40.0,
		Width:  300.0,
		Height: float64(len(items))*itemHeight + 40.0,
		Items:  contextItems,
	}
}

// computeViewBox calculates the SVG viewBox that fits all content with margin.
func computeViewBox(stages []StageLayout, ctx *NetworkContextLayout) ViewBox {
	if len(stages) == 0 {
		return ViewBox{Width: 800, Height: 400}
	}
	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := -math.MaxFloat64, -math.MaxFloat64
	for _, s := range stages {
		if s.X < minX {
			minX = s.X
		}
		if s.Y < minY {
			minY = s.Y
		}
		if r := s.X + s.Width; r > maxX {
			maxX = r
		}
		if b := s.Y + s.Height; b > maxY {
			maxY = b
		}
	}
	if ctx != nil {
		if ctx.X < minX {
			minX = ctx.X
		}
		if r := ctx.X + ctx.Width; r > maxX {
			maxX = r
		}
		if b := ctx.Y + ctx.Height; b > maxY {
			maxY = b
		}
	}
	return ViewBox{
		X:      minX - viewBoxMargin,
		Y:      minY - viewBoxMargin,
		Width:  maxX - minX + 2*viewBoxMargin,
		Height: maxY - minY + 2*viewBoxMargin,
	}
}
