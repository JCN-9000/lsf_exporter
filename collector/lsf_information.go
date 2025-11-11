package collector

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"

	"lsf_exporter/config"
)

func GetSolverMapping(filePath string) map[string]string {
	solverMap := make(map[string]string)
	if filePath == "" {
		return solverMap
	}

		slog.Debug(fmt.Sprintf("GetSolverMapping: Attempting to open solver mapping file: %s", filePath))
	file, err := os.Open(filePath)
	if err != nil {
		slog.Error(fmt.Sprintf("GetSolverMapping: Failed to open solver mapping file %s: %q", filePath, err))
		return solverMap
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ",", 2)
		if len(parts) == 2 {
			allowedKey := strings.ToLower(strings.TrimSpace(parts[0]))
			solverLabel := strings.TrimSpace(parts[1])
			solverMap[allowedKey] = solverLabel
			slog.Debug(fmt.Sprintf("GetSolverMapping: Parsed mapping: raw_line='%s', key_part='%s', value_part='%s', allowedKey='%s', solverLabel='%s'", line, parts[0], parts[1], allowedKey, solverLabel))
		} else {
			slog.Debug(fmt.Sprintf("GetSolverMapping: Skipping line (not 2 parts): raw_line='%s'", line))
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Error(fmt.Sprintf("GetSolverMapping: Error reading solver mapping file %s: %q", filePath, err))
	}
	slog.Debug(fmt.Sprintf("GetSolverMapping: Loaded solver map: %+v", solverMap))
	return solverMap
}
type InformationCollector struct {
	LsfInformation *prometheus.Desc
	logger         *slog.Logger
}

func init() {
	registerCollector("lsf_information", defaultEnabled, NewLSFInformationCollector)
}

// NewLmstatCollector returns a new Collector exposing lmstat license stats.
func NewLSFInformationCollector(logger *slog.Logger, config *config.Configuration) (Collector, error) {

	return &InformationCollector{
		LsfInformation: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "cluster", "info"),
			"A metric with a constant '1' value labeled by ClusterName, MasterName and Version of the IBM Spectrum LSF .",
			[]string{"clustername", "mastername", "version"}, nil,
		),
		logger: logger,
	}, nil
}

// Update calls (*lmstatCollector).getLmStat to get the platform specific
// memory metrics.
func (c *InformationCollector) Update(ch chan<- prometheus.Metric) error {
	// err := c.getLmstatInfo(ch)
	// if err != nil {
	// 	return fmt.Errorf("couldn't get lmstat version information: %w", err)
	// }

	err := c.parsebLsfClusterInfo(ch)
	if err != nil {
		return fmt.Errorf("couldn't get queues infomation: %w", err)
	}

	return nil
}

func lsfOutput(logger *slog.Logger, exe_file string, args ...string) ([]byte, error) {
	// _, err := os.Stat(*LSF_BINDIR)
	// if os.IsNotExist(err) {
	// 	logger.Error("err", "err", *LSF_BINDIR, "missing")
	// 	os.Exit(1)
	// }

	// _, err = os.Stat(*LSF_SERVERDIR)
	// if os.IsNotExist(err) {
	// 	logger.Error("err", "err", *LSF_SERVERDIR, "missing")
	// 	os.Exit(1)
	// }

	// _, err = os.Stat(*LSF_ENVDIR)
	// if os.IsNotExist(err) {
	// 	logger.Error("err", "err", *LSF_ENVDIR, "missing")
	// 	os.Exit(1)
	// }

	cmd := exec.Command(exe_file, args...)

	out, err := cmd.Output()

	if err != nil {
		return nil, fmt.Errorf("error while calling '%s %s': %v:'unknown error'",
			exe_file, strings.Join(args, " "), err)
	}

	return out, nil
}

func (c *InformationCollector) parsebLsfClusterInfo(ch chan<- prometheus.Metric) error {
	output, err := lsfOutput(c.logger, "lsid", "")
	if err != nil {
		c.logger.Error("err: ", "err", err)
		return nil
	}
	lsf_summary := string(output)
	md := map[string]string{}
	if ClusterNameRegex.MatchString(lsf_summary) {
		names := ClusterNameRegex.SubexpNames()
		matches := ClusterNameRegex.FindAllStringSubmatch(lsf_summary, -1)[0]
		for i, n := range matches {
			md[names[i]] = n
		}
	}

	if MasterNameRegex.MatchString(lsf_summary) {
		names := MasterNameRegex.SubexpNames()
		matches := MasterNameRegex.FindAllStringSubmatch(lsf_summary, -1)[0]
		for i, n := range matches {
			md[names[i]] = n
		}
	}

	if LSFVersionRegex.MatchString(lsf_summary) {
		names := LSFVersionRegex.SubexpNames()
		matches := LSFVersionRegex.FindAllStringSubmatch(lsf_summary, -1)[0]
		for i, n := range matches {
			md[names[i]] = n
		}
	}

	c.logger.Debug("当前集群名称：", "cluster_name", md["cluster_name"], ",当前的master节点名是:", "master_name", md["master_name"], ",版本是:", "lsf_version", md["lsf_version"])
	ch <- prometheus.MustNewConstMetric(c.LsfInformation, prometheus.GaugeValue, 1.0, md["cluster_name"], md["master_name"], md["lsf_version"])

	return nil
}
