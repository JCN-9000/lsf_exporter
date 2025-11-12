// Copyright 2015 The Prometheus Authors
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

package main

import (
	"context"
	"fmt"
	stdlog "log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/user"
	"runtime"
	"sort"
	"log/slog"

	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"

//	"github.com/a270443177/lsf_exporter/collector"
	"lsf_exporter/collector"
	"lsf_exporter/config"
	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	promcollectors "github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	"github.com/prometheus/exporter-toolkit/web/kingpinflag"
)

// handler wraps an unfiltered http.Handler but uses a filtered handler,
// created on the fly, if filtering is requested. Create instances with
// newHandler.
type handler struct {
	unfilteredHandler http.Handler
	// exporterMetricsRegistry is a separate registry for the metrics about
	// the exporter itself.
	exporterMetricsRegistry *prometheus.Registry
	includeExporterMetrics  bool
	maxRequests             int
	logger                  *slog.Logger
	config                  *config.Configuration
}

func newHandler(includeExporterMetrics bool, maxRequests int, logger *slog.Logger, cfg *config.Configuration) *handler {
	h := &handler{
		exporterMetricsRegistry: prometheus.NewRegistry(),
		includeExporterMetrics:  includeExporterMetrics,
		maxRequests:             maxRequests,
		logger:                  logger,
		config:                  cfg,
	}
	if h.includeExporterMetrics {
		h.exporterMetricsRegistry.MustRegister(
			promcollectors.NewProcessCollector(promcollectors.ProcessCollectorOpts{}),
			promcollectors.NewGoCollector(),
		)
	}
	if innerHandler, err := h.innerHandler(h.config); err != nil {
		panic(fmt.Sprintf("Couldn't create metrics handler: %s", err))
	} else {
		h.unfilteredHandler = innerHandler
	}
	return h
}

// ServeHTTP implements http.Handler.
func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	filters := r.URL.Query()["collect[]"]
	h.logger.Debug("collect query:", "filters", filters)

	if len(filters) == 0 {
		// No filters, use the prepared unfiltered handler.
		h.unfilteredHandler.ServeHTTP(w, r)
		return
	}
	// To serve filtered metrics, we create a filtering handler on the fly.
	filteredHandler, err := h.innerHandler(h.config, filters...)
	if err != nil {
		h.logger.Warn("Couldn't create filtered metrics handler:", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("Couldn't create filtered metrics handler: %s", err)))
		return
	}
	filteredHandler.ServeHTTP(w, r)
}

// innerHandler is used to create both the one unfiltered http.Handler to be
// wrapped by the outer handler and also the filtered handlers created on the
// fly. The former is accomplished by calling innerHandler without any arguments
// (in which case it will log all the collectors enabled via command-line
// flags).
func (h *handler) innerHandler(config *config.Configuration, filters ...string) (http.Handler, error) {
	nc, err := collector.NewLsfCollector(h.logger, config, filters...)
	if err != nil {
		return nil, fmt.Errorf("couldn't create collector: %s", err)
	}

	// Only log the creation of an unfiltered handler, which should happen
	// only once upon startup.
	if len(filters) == 0 {
		h.logger.Info("Enabled collectors")
		collectors := []string{}
		for n := range nc.Collectors {
			collectors = append(collectors, n)
		}
		sort.Strings(collectors)
		for _, c := range collectors {
			// FIX: Pass collector name as a key-value pair
			h.logger.Info("Collector enabled", "name", c)
		}
	}

	r := prometheus.NewRegistry()
	r.MustRegister(version.NewCollector("lsf_exporter"))
	if err := r.Register(nc); err != nil {
		return nil, fmt.Errorf("couldn't register lsf collector: %s", err)
	}
	handler := promhttp.HandlerFor(
		prometheus.Gatherers{h.exporterMetricsRegistry, r},
		promhttp.HandlerOpts{
			ErrorLog:            stdlog.New(os.Stderr, "ERROR: ", stdlog.Ldate|stdlog.Ltime|stdlog.Lshortfile),
			ErrorHandling:       promhttp.ContinueOnError,
			MaxRequestsInFlight: h.maxRequests,
			Registry:            h.exporterMetricsRegistry,
		},
	)
	if h.includeExporterMetrics {
		// Note that we have to use h.exporterMetricsRegistry here to
		// use the same promhttp metrics for all expositions.
		handler = promhttp.InstrumentMetricHandler(
			h.exporterMetricsRegistry, handler,
		)
	}
	return handler, nil
}

type slogAdapter struct {
	slog *slog.Logger
}

func (a *slogAdapter) Log(keyvals ...interface{}) error {
	var slogMsg string = "(no message)"
	var slogLevel slog.Level = slog.LevelInfo // Default to Info level
	var slogArgs []any // Directly build the args slice for slog.Log

	for i := 0; i < len(keyvals); {
		var key string
		var value interface{}

		// Attempt to get a string key from keyvals[i]
		if k, ok := keyvals[i].(string); ok {
			key = k
			// If it's a string key, try to get its value from keyvals[i+1]
			if i+1 < len(keyvals) {
				value = keyvals[i+1]
				i += 2 // Consumed a key-value pair
			} else {
				// String key without a corresponding value
				value = "(MISSING_VALUE)"
				i += 1 // Consumed only the key
			}
		} else {
			// keyvals[i] is not a string, so it must be a value.
			// Generate a generic key for it.
			key = fmt.Sprintf("arg%d", len(slogArgs)/2) // Use len(slogArgs)/2 for arg index
			value = keyvals[i]
			i += 1 // Consumed only the value
		}

		// Now, process the extracted key and value
		switch key {
		case "level":
			if l, ok := value.(string); ok {
				switch l {
				case "debug":
					slogLevel = slog.LevelDebug
				case "info":
					slogLevel = slog.LevelInfo
				case "warn":
					slogLevel = slog.LevelWarn
				case "error":
					slogLevel = slog.LevelError
				}
			}
		case "msg":
			slogMsg = fmt.Sprint(value)
		default:
			// Explicitly convert numeric values to string to avoid potential !BADKEY= issues
			// if slog has a hidden expectation for string values in certain contexts.
			slogArgs = append(slogArgs, key) // Append key first
			switch v := value.(type) {
			case float64:
				slogArgs = append(slogArgs, fmt.Sprintf("%f", v))
			case float32:
				slogArgs = append(slogArgs, fmt.Sprintf("%f", v))
			case int:
				slogArgs = append(slogArgs, fmt.Sprintf("%d", v))
			case int64:
				slogArgs = append(slogArgs, fmt.Sprintf("%d", v))
			case int32:
				slogArgs = append(slogArgs, fmt.Sprintf("%d", v))
			case string:
				slogArgs = append(slogArgs, v)
			default:
				slogArgs = append(slogArgs, value)
			}
		}
	}

	a.slog.Log(context.Background(), slogLevel, slogMsg, slogArgs...)
	return nil
}

func main() {
	var (
		metricsPath = kingpin.Flag(
			"web.telemetry-path",
			"Path under which to expose metrics.",
		).Default("/metrics").String()
		disableExporterMetrics = kingpin.Flag(
			"web.disable-exporter-metrics",
			"Exclude metrics about the exporter itself (promhttp_*, process_*, go_*).",
		).Bool()
		maxRequests = kingpin.Flag(
			"web.max-requests",
			"Maximum number of parallel scrape requests. Use 0 to disable.",
		).Default("40").Int()
		disableDefaultCollectors = kingpin.Flag(
			"collector.disable-defaults",
			"Set all collectors to disabled by default.",
		).Default("false").Bool()
		maxProcs = kingpin.Flag(
			"runtime.gomaxprocs", "The target number of CPUs Go will run on (GOMAXPROCS)",
		).Envar("GOMAXPROCS").Default("1").Int()
		toolkitFlags = kingpinflag.AddFlags(kingpin.CommandLine, ":9818")
		lsfStdSolverConfig = kingpin.Flag(
			"lsf.std-solver-config",
			"Path to the solver standardization mapping file.",
		).Default("").String()
	)

	promlogConfig := &promlog.Config{}

	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.Version(version.Print("lsf_exporter"))
	kingpin.CommandLine.UsageWriter(os.Stdout)

	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	var programLevel = slog.LevelInfo
	if promlogConfig.Level.String() == "debug" {
		programLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: programLevel,
	}))

	if *disableDefaultCollectors {
		collector.DisableDefaultCollectors()
	}
	logger.Info("Starting lsf_exporter", "version", version.Info())
	logger.Info("Build context", "build_context", version.BuildContext())
	if user, err := user.Current(); err == nil && user.Uid == "0" {
		logger.Warn("lsf Exporter is running as root user. This exporter is designed to run as unprivileged user, root is not required.")
	}
	runtime.GOMAXPROCS(*maxProcs)
	logger.Debug("Go MAXPROCS", "procs", runtime.GOMAXPROCS(0))

	cfg := &config.Configuration{
		CliOpts: config.CliOpts{
			LsfStdSolverConfig: *lsfStdSolverConfig,
		},
	}

	http.Handle(*metricsPath, newHandler(!*disableExporterMetrics, *maxRequests, logger, cfg))
	if *metricsPath != "/" {
		landingConfig := web.LandingConfig{
			Name:        "Lsf Exporter",
			Description: "Prometheus Lsf Exporter",
			Version:     version.Info(),
			Links: []web.LandingLinks{
				{
					Address: *metricsPath,
					Text:    "Metrics",
				},
			},
		}
		landingPage, err := web.NewLandingPage(landingConfig)
		if err != nil {
			logger.Error("err", err)
			os.Exit(1)
		}
		http.Handle("/", landingPage)
	}

	server := &http.Server{}
	adapter := &slogAdapter{slog: logger}
	if err := web.ListenAndServe(server, toolkitFlags, adapter); err != nil {
		logger.Error("err", err)
		os.Exit(1)
	}
}
