package findmy

import (
	"fmt"
	"os"
	"time"
)

// maxSidebarPages bounds ScrollPages so a list that never settles (rows
// re-sorting while distances load) cannot keep it scrolling forever.
const maxSidebarPages = 25

// ScrollPages reads a sidebar that can be longer than the window. It scrolls
// to the top, then captures and OCRs page after page. visit returns the row
// names it parsed from a page.
//
// The helper's scroll is a trackpad-style gesture that macOS accelerates, so
// the distance a step travels varies. Steps are a third of the visible list,
// and a page that shares no row with the one before it means a step jumped
// past rows: ScrollPages backs up half a step and reads again. It stops after
// two pages in a row add nothing new (the bottom), and scrolls back to the top.
//
// Scrolling posts events, which needs Accessibility. Without it only the rows
// in the window are read, and a note on stderr says so.
func ScrollPages(w *Window, shot string, visit func(lines []TextLine, first bool) (names []string, err error)) error {
	canScroll := false
	if p, err := CheckPermissions(); err == nil {
		canScroll = p.Accessibility
	}
	x, y := w.X+SidebarRightPt/2, w.Y+w.Height/2
	if canScroll {
		scrollSidebarToTop(x, y)
		defer scrollSidebarToTop(x, y)
	}
	// The helper scrolls dy*30 points before acceleration; the list starts
	// ~150pt below the top of the window.
	step := (w.Height - 150) / 3 / 30
	if step < 2 {
		step = 2
	}
	seen := map[string]bool{}
	var prev map[string]bool
	idle, backedUp := 0, false
	for page := 0; page < maxSidebarPages; page++ {
		if err := Capture(w, shot); err != nil {
			return err
		}
		lines, err := OCR(shot)
		if err != nil {
			return err
		}
		names, err := visit(lines, page == 0)
		if err != nil {
			return err
		}
		if !canScroll {
			fmt.Fprintln(os.Stderr, "note: Accessibility is not granted, so only rows visible in the Find My window were read")
			return nil
		}
		cur := map[string]bool{}
		added, overlap := 0, false
		for _, n := range names {
			cur[n] = true
			if prev[n] {
				overlap = true
			}
			if !seen[n] {
				seen[n] = true
				added++
			}
		}
		if prev != nil && !overlap && len(names) > 0 && !backedUp {
			// Jumped past at least one row: go back half a step, read again.
			backedUp = true
			if err := Scroll(x, y, step/2+1); err != nil {
				return fmt.Errorf("scroll sidebar: %w", err)
			}
			time.Sleep(700 * time.Millisecond)
			continue
		}
		backedUp = false
		if added == 0 {
			idle++
		} else {
			idle = 0
		}
		if page > 0 && idle >= 2 {
			return nil
		}
		prev = cur
		if err := Scroll(x, y, -step); err != nil {
			return fmt.Errorf("scroll sidebar: %w", err)
		}
		time.Sleep(700 * time.Millisecond)
	}
	return nil
}

func scrollSidebarToTop(x, y int) {
	for i := 0; i < 3; i++ {
		_ = Scroll(x, y, 40)
	}
	time.Sleep(600 * time.Millisecond)
}
