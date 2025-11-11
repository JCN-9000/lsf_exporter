package collector

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"strings"
	"log/slog"

	"github.com/jszwec/csvutil"
	"github.com/prometheus/client_golang/prometheus"

	"lsf_exporter/config"
)

type bHostsCollector struct {
	HostRunningJobCount *prometheus.Desc
	HostNJobsCount      *prometheus.Desc

	HostMaxJobCount   *prometheus.Desc
	HostSSUSPJobCount *prometheus.Desc
	HostUSUSPJobCount *prometheus.Desc
	HostStatus        *prometheus.Desc
	logger            *slog.Logger
}

func init() {
	registerCollector("bhosts", defaultEnabled, NewLSFbHostCollector)
}

// NewLmstatCollector returns a new Collector exposing lmstat license stats.
func NewLSFbHostCollector(logger *slog.Logger, config *config.Configuration) (Collector, error) {

	return &bHostsCollector{
		HostRunningJobCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bhost", "runningjob_count"),
			"The number of tasks for all running jobs on the host.",
			[]string{"host_name"}, nil,
		),
		HostNJobsCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bhost", "njobs_count"),
			"The number of tasks for all jobs that are dispatched to the host. The NJOBS value includes running, suspended, and chunk jobs.",
			[]string{"host_name"}, nil,
		),
		HostMaxJobCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bhost", "maxjob_count"),
			"The maximum number of job slots available. A dash (-1) indicates no limit.",
			[]string{"host_name"}, nil,
		),
		HostSSUSPJobCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bhost", "ssuspjob_count"),
			"The number of tasks for all system suspended jobs on the host.",
			[]string{"host_name"}, nil,
		),
		HostUSUSPJobCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bhost", "ususpjob_count"),
			"The number of tasks for all user suspended jobs on the host. Jobs can be suspended by the user or by the LSF administrator.",
			[]string{"host_name"}, nil,
		),
		HostStatus: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bhost", "host_status"),
			"The status of the host and the sbatchd daemon. Batch jobs can be dispatched only to hosts with an ok status. Host status has the following, 0:Unknow, 1:ok, 2:unavail, 3:unreach, 4:closed/closed_full, 5:closed_cu_excl",
			[]string{"host_name"}, nil,
		),
		logger: logger,
	}, nil
}

// Update calls (*lmstatCollector).getLmStat to get the platform specific
// memory metrics.
func (c *bHostsCollector) Update(ch chan<- prometheus.Metric) error {
	// err := c.getLmstatInfo(ch)
	// if err != nil {
	// 	return fmt.Errorf("couldn't get lmstat version information: %w", err)
	// }

	err := c.parsebHostJobCount(ch)

	if err != nil {
		return fmt.Errorf("couldn't get bhosts infomation: %w", err)
	}

	return nil
}

type TrimReader struct{ io.Reader }

var trailingws = regexp.MustCompile(` +\r?\n`)

func (tr TrimReader) Read(bs []byte) (int, error) {
	// Perform the requested read on the given reader.
	n, err := tr.Reader.Read(bs)
	if err != nil {
		return n, err
	}

	// Remove trailing whitespace from each line.
	lines := string(bs[:n])
	trimmed := []byte(trailingws.ReplaceAllString(lines, "\n"))
	copy(bs, trimmed)
	return len(trimmed), nil
}

func bhost_CsvtoStruct(lsfOutput []byte, logger *slog.Logger) ([]bhostInfo, error) {
	csv_out := csv.NewReader(TrimReader{bytes.NewReader(lsfOutput)})
	csv_out.LazyQuotes = true
	csv_out.Comma = ' '
	csv_out.TrimLeadingSpace = true

	dec, err := csvutil.NewDecoder(csv_out)
	if err != nil {
		logger.Error("err=", "err", err)
		return nil, nil
	}

	var bhostInfos []bhostInfo

	for {
		var u bhostInfo
		if err := dec.Decode(&u); err == io.EOF {
			break
		} else if err != nil {
			logger.Error("err=", "err", err)
			return nil, nil
		}

		bhostInfos = append(bhostInfos, u)
	}
	return bhostInfos, nil

}

func FormatbhostsStatus(status string, logger *slog.Logger) float64 {
	state := strings.ToLower(status)
	logger.Debug("The value currently obtained is: ", "status", status, "The converted value is: ", "state", state)
	switch state {
	case "ok":
		return float64(1)
	case "unavail", "closed_adm":
		return float64(2)
	case "unreach":
		return float64(3)
	case "closed", "closed_excl", "closed_full" :
		return float64(4)
	default:
		return float64(0)
	}
}

func (c *bHostsCollector) parsebHostJobCount(ch chan<- prometheus.Metric) error {
	output, err := lsfOutput(c.logger, "bhosts", "-w", "-X")
	if err != nil {
		c.logger.Error("err: ", "err", err)
		return nil
	}
	bhosts, err := bhost_CsvtoStruct(output, c.logger)
	if err != nil {
		c.logger.Error("err: ", "err", err)
		return nil
	}

	for _, bhost := range bhosts {
		ch <- prometheus.MustNewConstMetric(c.HostNJobsCount, prometheus.GaugeValue, bhost.NJOBS, bhost.HOST_NAME)
		ch <- prometheus.MustNewConstMetric(c.HostRunningJobCount, prometheus.GaugeValue, bhost.RUN, bhost.HOST_NAME)
		ch <- prometheus.MustNewConstMetric(c.HostMaxJobCount, prometheus.GaugeValue, bhost.MAX, bhost.HOST_NAME)
		ch <- prometheus.MustNewConstMetric(c.HostSSUSPJobCount, prometheus.GaugeValue, bhost.SSUSP, bhost.HOST_NAME)
		ch <- prometheus.MustNewConstMetric(c.HostUSUSPJobCount, prometheus.GaugeValue, bhost.USUSP, bhost.HOST_NAME)
		ch <- prometheus.MustNewConstMetric(c.HostStatus, prometheus.GaugeValue, FormatbhostsStatus(bhost.STATUS, c.logger), bhost.HOST_NAME)
	}

	return nil
}
