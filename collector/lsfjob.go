package collector

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"

	"github.com/jszwec/csvutil"
	"github.com/prometheus/client_golang/prometheus"

	"lsf_exporter/config"
)

type Job struct {
	ID            string
	User          string
	Status        string
	Queue         string
	FromHost      string
	ExecutionHost string
	JobName       string
	SubmitTime    string
	UserGroup     string
	Project       string
	Application   string
	JGroup        string
	Dependency    string
	NSlot         string
	NProc         string
	StartTime     string
	SubCWD        string
	PendTime      string
	EPendTime     string
	IPendTime     string
	Solver        string
	SrcJobid      string
	DstJobid      string
	SrcCluster    string
	DstCluster    string
}

type JobCollector struct {
	JobInfoNCpuCount    *prometheus.Desc
	JobInfoPendingTime  *prometheus.Desc
	JobInfoEPendingTime *prometheus.Desc
	JobInfoIPendingTime *prometheus.Desc
	//	JobInfo *prometheus.Desc
	logger    *slog.Logger
	solverMap map[string]string
}

func init() {
	registerCollector("lsfjob", defaultEnabled, NewLSFJobCollector)
}

// NewLSFJobCollector returns a new Collector exposing job info
func NewLSFJobCollector(logger *slog.Logger, config *config.Configuration) (Collector, error) {
	logger.Debug("LSFJobCollector: LsfStdSolverConfig path:", "path", config.CliOpts.LsfStdSolverConfig)
	solverMap := GetSolverMapping(config.CliOpts.LsfStdSolverConfig)
	logger.Debug("LSFJobCollector: Loaded solver mappings", "count", len(solverMap))

	labelsName := []string{
		"ID",
		"User",
		"Status",
		"Queue",
		"FromHost",
		"ExecutionHost",
		"JobName",
		"UserGroup",
		"Project",
		"Application",
		"Solver",
		"JGroup",
		"Dependency",
		"NSlot",
		"NProc",
		"StartTime",
		"SubCWD",
		"PendTime",
		"EPendTime",
		"IPendTime",
		"SrcJobid",
		"DstJobid",
		"SrcCluster",
		"DstCluster",
		"SubmitTime",
	}

	return &JobCollector{
		JobInfoNCpuCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bjobs", "ncpu_count"),
			"bjobs ncpu labeled by id, user, status, queue and FromHost of the starttime.",
			labelsName,
			nil,
		),

		JobInfoPendingTime: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bjobs", "pending_time_total"),
			"Job pending time since submission (sec)",
			labelsName,
			nil,
		),

		JobInfoEPendingTime: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bjobs", "pending_time_eligible_total"),
			"Job eligible pending time since submission (sec)",
			labelsName,
			nil,
		),

		JobInfoIPendingTime: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bjobs", "pending_time_ineligible_total"),
			"Job ineligible pending time since submission (sec)",
			labelsName,
			nil,
		),

		logger:    logger,
		solverMap: solverMap,
	}, nil
}

// Update calls c.getJobStatus to get the job info
// memory metrics.
func (c *JobCollector) Update(ch chan<- prometheus.Metric) error {
	err := c.getJobStatus(ch)
	if err != nil {
		return fmt.Errorf("couldn't get job infomation: %w", err)
	}

	return nil
}

func parseJobStatus(jobJson bjobsInfo) Job {
	//now := time.Now().Unix()
	//level.Info(logger).Log("Parsing Job")

	//fmt.Printf("%s %+v\n", "SubmitTime:", jobJson.SUBMIT_TIME)
	//ts, err := time.Parse("Jan _2 15:04", jobJson.SUBMIT_TIME)
	//if err != nil {
	//	// fmt.Printf("Error Decoding Time: %s\n", err)
	//}
	//ts = ts.AddDate(time.Now().Year(), 0, 0)

	return Job{
		ID:            jobJson.JOBID,
		User:          jobJson.USER,
		Status:        jobJson.STATUS,
		Queue:         jobJson.QUEUE,
		FromHost:      jobJson.FROM_HOST,
		ExecutionHost: jobJson.EXEC_HOST,
		JobName:       jobJson.JOB_NAME,
		UserGroup:     jobJson.UGROUP,
		Project:       jobJson.PROJECT,
		Application:   jobJson.APPLICATION,
		JGroup:        jobJson.JOB_GROUP,
		Dependency:    jobJson.DEPENDENCY,
		NSlot:         jobJson.NALLOC_SLOT,
		NProc:         jobJson.MIN_REQ_PROC,
		StartTime:     jobJson.START_TIME,
		SubCWD:        jobJson.SUB_CWD,
		PendTime:      jobJson.PEND_TIME,
		EPendTime:     jobJson.EPENDTIME,
		IPendTime:     jobJson.IPENDTIME,
		Solver:        jobJson.APPLICATION, // Placeholder, will be standardized later
		SrcJobid:      jobJson.SRCJOBID,
		DstJobid:      jobJson.DSTJOBID,
		SrcCluster:    jobJson.SRCCLUSTER,
		DstCluster:    jobJson.DSTCLUSTER,
		SubmitTime:    jobJson.SUBMIT_TIME, //int64(ts.Unix()),
	}
}

type lsf_answer struct {
	COMMAND string      `json:"COMMAND"`
	JOBS    int         `json:"JOBS"`
	RECORDS []bjobsInfo `json:"RECORDS"`
}

func bjobs_JsontoStruct(lsfOutput []byte, logger *slog.Logger) ([]bjobsInfo, error) {
	//var bjobsInfos []bjobsInfo
	lsfAnswer := &lsf_answer{}

	//fmt.Printf("%+s", lsfOutput)
	//err := json.Unmarshal(lsfOutput, &bjobsInfos)
	err := json.Unmarshal(lsfOutput, lsfAnswer)
	if err != nil {
		logger.Error("Error unmarshalling JSON", "err", err)
		return nil, nil
	}
	//fmt.Printf("%+v\n", lsfAnswer.JOBS)
	//fmt.Printf("%+v\n", lsfAnswer.RECORDS)

	return lsfAnswer.RECORDS, nil
}

func bjobs_CsvtoStruct(lsfOutput []byte, logger *slog.Logger) ([]bjobsInfo, error) {
	csv_out := csv.NewReader(TrimReader{bytes.NewReader(lsfOutput)})
	csv_out.LazyQuotes = true
	csv_out.Comma = ' '
	csv_out.TrimLeadingSpace = true

	dec, err := csvutil.NewDecoder(csv_out)
	if err != nil {
		logger.Error("Error decoding CSV", "err", err)
		return nil, nil
	}

	var bjobsInfos []bjobsInfo

	for {
		var u bjobsInfo
		if err := dec.Decode(&u); err == io.EOF {
			break
		}

		bjobsInfos = append(bjobsInfos, u)
	}
	return bjobsInfos, nil

}

func (c *JobCollector) getJobStatus(ch chan<- prometheus.Metric) error {
	//output, err := lsfOutput(c.logger, "bjobs", "-w", "-u", "all")
	output, err := lsfOutput(c.logger, "bjobs", "-X", "-u", "all", "-o",
		"JOBID USER STAT QUEUE FROM_HOST EXEC_HOST JOB_NAME SUBMIT_TIME UGROUP PROJECT APPLICATION JOB_GROUP DEPENDENCY NALLOC_SLOT MIN_REQ_PROC START_TIME SUB_CWD PEND_TIME EPENDTIME IPENDTIME SRCJOBID DSTJOBID SRCLUSTER FWD_CLUSTER", "-json")
	if err != nil {
		c.logger.Error("Failed to get bjobs output", "err", err, "output", string(output))
		return nil
	}
	//fmt.Printf("%+s", output)
	//level.Info(c.logger).Log("err=", err, output)

	//jobs, err := bjobs_CsvtoStruct(output, c.logger)
	jobs, err := bjobs_JsontoStruct(output, c.logger)
	if err != nil {
		c.logger.Error("Failed to parse bjobs output", "err", err)
		return nil
	}
	//fmt.Printf("%+v\n", jobs)
	//fmt.Printf("%+v\n", len(jobs))

	for _, j := range jobs {
		jobStatus := parseJobStatus(j)

		var solverName string
		if jobStatus.Application != "" {
			solverName = jobStatus.Application
		} else {
			solverName = jobStatus.Queue
		}
		c.logger.Debug("LSFJobCollector",
			"application", jobStatus.Application,
			"queue", jobStatus.Queue,
			"solver_name", solverName)
		standardizedSolver := c.solverMap[strings.ToLower(solverName)]
		if standardizedSolver == "" {
			standardizedSolver = "unknown"
		}
		c.logger.Debug("LSFJobCollector",
			"standardized_solver", standardizedSolver)
		jobStatus.Solver = standardizedSolver

		labelsValue := []string{
			jobStatus.ID,
			jobStatus.User,
			jobStatus.Status,
			jobStatus.Queue,
			jobStatus.FromHost,
			jobStatus.ExecutionHost,
			jobStatus.JobName,
			jobStatus.UserGroup,
			jobStatus.Project,
			jobStatus.Application,
			jobStatus.Solver,
			jobStatus.JGroup,
			jobStatus.Dependency,
			jobStatus.NSlot,
			jobStatus.NProc,
			jobStatus.StartTime,
			jobStatus.SubCWD,
			jobStatus.PendTime,
			jobStatus.EPendTime,
			jobStatus.IPendTime,
			jobStatus.SrcJobid,
			jobStatus.DstJobid,
			jobStatus.SrcCluster,
			jobStatus.DstCluster,
			jobStatus.SubmitTime,
		}

		nCPU, _ := strconv.ParseFloat(jobStatus.NProc, 64)
		ch <- prometheus.MustNewConstMetric(c.JobInfoNCpuCount, prometheus.GaugeValue, nCPU,
			labelsValue...,
		)

		pTime, _ := strconv.ParseFloat(jobStatus.PendTime, 64)
		ch <- prometheus.MustNewConstMetric(c.JobInfoPendingTime, prometheus.CounterValue, pTime,
			labelsValue...,
		)

		epTime, _ := strconv.ParseFloat(jobStatus.EPendTime, 64)
		ch <- prometheus.MustNewConstMetric(c.JobInfoEPendingTime, prometheus.CounterValue, epTime,
			labelsValue...,
		)

		ipTime, _ := strconv.ParseFloat(jobStatus.IPendTime, 64)
		ch <- prometheus.MustNewConstMetric(c.JobInfoIPendingTime, prometheus.CounterValue, ipTime,
			labelsValue...,
		)

	}

	return nil

}
