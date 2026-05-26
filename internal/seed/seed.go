// Package seed fills an empty database with demo data: components,
// monitors pointing at the bundled demo target, and some history.
package seed

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"time"

	"github.com/sorin-work/status-page/internal/monitor"
	"github.com/sorin-work/status-page/internal/store"
)

// Options configure the demo data.
type Options struct {
	// TargetURL is the base URL of the demo target service.
	TargetURL string
	Now       time.Time
	Logger    *slog.Logger
}

type demoMonitor struct {
	component string
	monitor   monitor.Monitor
}

// Demo seeds the database unless it already has monitors. It reports
// whether anything was written.
func Demo(ctx context.Context, st *store.Store, opts Options) (bool, error) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	existing, err := st.ListMonitors(ctx)
	if err != nil {
		return false, err
	}
	if len(existing) > 0 {
		return false, nil
	}
	base, err := url.Parse(opts.TargetURL)
	if err != nil || base.Host == "" {
		return false, fmt.Errorf("demo target URL %q is not valid", opts.TargetURL)
	}
	tcpAddr := base.Host
	if base.Port() == "" {
		tcpAddr = net.JoinHostPort(base.Hostname(), "80")
	}

	components := []store.Component{
		{Name: "Website", Description: "Storefront and marketing pages"},
		{Name: "API", Description: "Public REST API"},
		{Name: "Checkout", Description: "Cart and payments"},
		{Name: "Search", Description: "Product search"},
		{Name: "Database", Description: "Primary database cluster"},
	}
	ids := map[string]int64{}
	for _, c := range components {
		created, err := st.CreateComponent(ctx, c)
		if err != nil {
			return false, err
		}
		ids[c.Name] = created.ID
	}

	target := func(path string) string {
		return base.ResolveReference(&url.URL{Path: path}).String()
	}
	monitors := []demoMonitor{
		{"Website", monitor.Monitor{Name: "Storefront", Type: monitor.TypeHTTP, Target: target("/"), Interval: time.Minute, Timeout: 10 * time.Second, Keyword: "Acme Shop", Retries: 1}},
		{"API", monitor.Monitor{Name: "API health", Type: monitor.TypeHTTP, Target: target("/api/health"), Interval: 30 * time.Second, Timeout: 5 * time.Second, ExpectedStatus: 200, Keyword: `"status":"ok"`, Retries: 1}},
		{"Checkout", monitor.Monitor{Name: "Checkout", Type: monitor.TypeHTTP, Target: target("/checkout"), Interval: time.Minute, Timeout: 10 * time.Second, Retries: 2}},
		{"Search", monitor.Monitor{Name: "Search", Type: monitor.TypeHTTP, Target: target("/search"), Interval: time.Minute, Timeout: 5 * time.Second, Retries: 1}},
		{"Database", monitor.Monitor{Name: "Database port", Type: monitor.TypeTCP, Target: tcpAddr, Interval: 30 * time.Second, Timeout: 3 * time.Second}},
		{"", monitor.Monitor{Name: "Status endpoint (internal)", Type: monitor.TypeHTTP, Target: target("/status/204"), Interval: 5 * time.Minute, Timeout: 5 * time.Second, ExpectedStatus: 204}},
	}
	var created []monitor.Monitor
	for _, dm := range monitors {
		m := dm.monitor
		m.ComponentID = ids[dm.component]
		if err := m.Validate(); err != nil {
			return false, fmt.Errorf("demo monitor %s: %w", m.Name, err)
		}
		saved, err := st.CreateMonitor(ctx, m)
		if err != nil {
			return false, err
		}
		created = append(created, saved)
	}
	if err := history(ctx, st, created, opts.Now.UTC()); err != nil {
		return false, fmt.Errorf("demo history: %w", err)
	}
	opts.Logger.Info("seeded demo data", "components", len(components), "monitors", len(monitors), "target", opts.TargetURL)
	return true, nil
}
