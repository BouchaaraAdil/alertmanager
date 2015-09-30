package test

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/prometheus/alertmanager/types"
)

// Collector gathers alerts received by a notification destination
// and verifies whether all arrived and within the correct time boundaries.
type Collector struct {
	t    *testing.T
	name string
	opts *AcceptanceOpts

	collected map[float64][][]*types.Alert
	exepected map[Interval][][]*types.Alert
}

func (c *Collector) String() string {
	return c.name
}

func batchesEqual(as, bs []*types.Alert, opts *AcceptanceOpts) bool {
	if len(as) != len(bs) {
		return false
	}

	sort.Sort(types.AlertTimeline(as))
	sort.Sort(types.AlertTimeline(bs))

	for i, a := range as {
		if !equalAlerts(a, bs[i], opts) {
			return false
		}
	}
	return true
}

// latest returns the latest relative point in time where a notification is
// expected.
func (c *Collector) latest() float64 {
	var latest float64
	for iv := range c.exepected {
		if iv.end > latest {
			latest = iv.end
		}
	}
	return latest
}

// want declares that the Collector expects to receive the given alerts
// within the given time boundaries.
func (c *Collector) Want(iv Interval, alerts ...*TestAlert) {
	var nas []*types.Alert
	for _, a := range alerts {
		nas = append(nas, a.nativeAlert(c.opts))
	}

	c.exepected[iv] = append(c.exepected[iv], nas)
}

// add the given alerts to the collected alerts.
func (c *Collector) add(alerts ...*types.Alert) {
	arrival := c.opts.relativeTime(time.Now())

	c.collected[arrival] = append(c.collected[arrival], alerts)
}

func (c *Collector) check() string {
	report := fmt.Sprintf("\ncollector %q:\n\n", c)

	for iv, expected := range c.exepected {
		report += fmt.Sprintf("interval %v\n", iv)

		for _, exp := range expected {
			var found []*types.Alert
			report += fmt.Sprintf("---\n")

			for _, e := range exp {
				report += fmt.Sprintf("- %v\n", e)
			}

			for at, got := range c.collected {
				if !iv.contains(at) {
					continue
				}
				for _, a := range got {
					if batchesEqual(exp, a, c.opts) {
						found = a
						break
					}
				}
				if found != nil {
					break
				}
			}

			if found != nil {
				report += fmt.Sprintf("  [ ✓ ]\n")
			} else {
				c.t.Fail()
				report += fmt.Sprintf("  [ ✗ ]\n")
			}
		}
	}

	// Detect unexpected notifications.
	var totalExp, totalAct int
	for _, exp := range c.exepected {
		for _, e := range exp {
			totalExp += len(e)
		}
	}
	for _, act := range c.collected {
		for _, a := range act {
			if len(a) == 0 {
				c.t.Error("received empty notifications")
			}
			totalAct += len(a)
		}
	}
	if totalExp != totalAct {
		c.t.Fail()
		report += fmt.Sprintf("\nExpected total of %d alerts, got %d", totalExp, totalAct)
	}

	if c.t.Failed() {
		report += "\nreceived:\n"

		for at, col := range c.collected {
			for _, alerts := range col {
				report += fmt.Sprintf("@ %v\n", at)
				for _, a := range alerts {
					report += fmt.Sprintf("- %v\n", a)
				}
			}
		}
	}

	return report
}
